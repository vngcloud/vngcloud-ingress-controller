package controller

import (
	"fmt"
	"strconv"

	client2 "github.com/vngcloud/vngcloud-go-sdk/client"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/listener"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/loadbalancer"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/pool"
	v1Portal "github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/portal/v1"
	"github.com/vngcloud/vngcloud-ingress-controller/pkg/consts"
	"github.com/vngcloud/vngcloud-ingress-controller/pkg/utils/metadata"
	nwv1 "k8s.io/api/networking/v1"
	"k8s.io/klog/v2"
)

func getMetadataOption(pMetadata metadata.Opts) metadata.Opts {
	if pMetadata.SearchOrder == "" {
		pMetadata.SearchOrder = fmt.Sprintf("%s,%s", metadata.ConfigDriveID, metadata.MetadataID)
	}
	klog.Info("getMetadataOption; metadataOpts is ", pMetadata)
	return pMetadata
}

func setupPortalInfo(pProvider *client2.ProviderClient, pMedatata metadata.IMetadata, pPortalURL string) (*ExtraInfo, error) {
	projectID, err := pMedatata.GetProjectID()
	if err != nil {
		return nil, err
	}

	portalClient, _ := vngcloud.NewServiceClient(pPortalURL, pProvider, "portal")
	portalInfo, err := v1Portal.Get(portalClient, projectID)

	if err != nil {
		return nil, err
	}

	if portalInfo == nil {
		return nil, fmt.Errorf("can not get portal information")
	}

	return &ExtraInfo{
		ProjectID: portalInfo.ProjectID,
		UserID:    portalInfo.UserID,
	}, nil
}

func PointerOf[T any](t T) *T {
	return &t
}

func CreateLoadbalancerOptions(ing *nwv1.Ingress) *loadbalancer.CreateOpts {
	opt := &loadbalancer.CreateOpts{
		Name:      "",
		PackageID: consts.DEFAULT_PACKAGE_ID,
		Scheme:    loadbalancer.CreateOptsSchemeOptInternet,
		SubnetID:  "",
		Type:      loadbalancer.CreateOptsTypeOptLayer7,
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationLoadBalancerName]; ok {
		opt.Name = option
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationPackageID]; ok {
		opt.PackageID = option
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationLoadBalancerInternal]; ok {
		switch option {
		case "true":
			opt.Scheme = loadbalancer.CreateOptsSchemeOptInternal
		case "false":
			opt.Scheme = loadbalancer.CreateOptsSchemeOptInternet
		default:
			klog.Warningf("Invalid annotation \"%s\" value, must be true or false", consts.ServiceAnnotationLoadBalancerInternal)
		}
	}
	return opt
}

func CreateListenerOptions(ing *nwv1.Ingress, isHTTPS bool) *listener.CreateOpts {
	opt := &listener.CreateOpts{
		ListenerName:                consts.DEFAULT_HTTP_LISTENER_NAME,
		ListenerProtocol:            listener.CreateOptsListenerProtocolOptHTTP,
		ListenerProtocolPort:        80,
		CertificateAuthorities:      nil,
		ClientCertificate:           nil,
		DefaultCertificateAuthority: nil,
		DefaultPoolId:               "",
		TimeoutClient:               50,
		TimeoutMember:               50,
		TimeoutConnection:           5,
		AllowedCidrs:                "0.0.0.0/0",
	}
	if isHTTPS {
		opt.ListenerName = consts.DEFAULT_HTTPS_LISTENER_NAME
		opt.ListenerProtocol = listener.CreateOptsListenerProtocolOptHTTPS
		opt.ListenerProtocolPort = 443

	}
	if ing == nil {
		return opt
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationIdleTimeoutClient]; ok {
		opt.TimeoutClient = ParseIntAnnotation(option, consts.ServiceAnnotationIdleTimeoutClient, opt.TimeoutClient)
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationIdleTimeoutMember]; ok {
		opt.TimeoutMember = ParseIntAnnotation(option, consts.ServiceAnnotationIdleTimeoutMember, opt.TimeoutMember)
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationIdleTimeoutConnection]; ok {
		opt.TimeoutConnection = ParseIntAnnotation(option, consts.ServiceAnnotationIdleTimeoutConnection, opt.TimeoutConnection)
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationListenerAllowedCIDRs]; ok {
		opt.AllowedCidrs = option
	}
	return opt
}

func CreatePoolOptions(ing *nwv1.Ingress) *pool.CreateOpts {
	opt := &pool.CreateOpts{
		PoolName:      "",
		PoolProtocol:  pool.CreateOptsProtocolOptHTTP,
		Stickiness:    PointerOf[bool](false),
		TLSEncryption: PointerOf[bool](false),
		HealthMonitor: pool.HealthMonitor{
			HealthyThreshold:    3,
			UnhealthyThreshold:  3,
			Interval:            30,
			Timeout:             5,
			HealthCheckProtocol: pool.CreateOptsHealthCheckProtocolOptTCP,
		},
		Algorithm: pool.CreateOptsAlgorithmOptRoundRobin,
		Members:   []*pool.Member{},
	}
	if ing == nil {
		return opt
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationMonitorProtocol]; ok {
		switch option {
		case string(pool.CreateOptsHealthCheckProtocolOptTCP), string(pool.CreateOptsHealthCheckProtocolOptHTTP):
			opt.HealthMonitor.HealthCheckProtocol = pool.CreateOptsHealthCheckProtocolOpt(option)
			if option == string(pool.CreateOptsHealthCheckProtocolOptHTTP) {
				opt.HealthMonitor = pool.HealthMonitor{
					HealthyThreshold:    3,
					UnhealthyThreshold:  3,
					Interval:            30,
					Timeout:             5,
					HealthCheckProtocol: pool.CreateOptsHealthCheckProtocolOptHTTP,
					HealthCheckMethod:   PointerOf(pool.CreateOptsHealthCheckMethodOptGET),
					HealthCheckPath:     PointerOf("/"),
					SuccessCode:         PointerOf("200"),
					HttpVersion:         PointerOf(pool.CreateOptsHealthCheckHttpVersionOptHttp1),
					DomainName:          PointerOf(""),
				}
				if option, ok := ing.Annotations[consts.ServiceAnnotationMonitorHttpMethod]; ok {
					switch option {
					case string(pool.CreateOptsHealthCheckMethodOptGET),
						string(pool.CreateOptsHealthCheckMethodOptPUT),
						string(pool.CreateOptsHealthCheckMethodOptPOST):
						opt.HealthMonitor.HealthCheckMethod = PointerOf(pool.CreateOptsHealthCheckMethodOpt(option))
					default:
						klog.Warningf("Invalid annotation \"%s\" value, must be one of %s, %s, %s", consts.ServiceAnnotationMonitorHttpMethod,
							pool.CreateOptsHealthCheckMethodOptGET,
							pool.CreateOptsHealthCheckMethodOptPUT,
							pool.CreateOptsHealthCheckMethodOptPOST)
					}
				}
				if option, ok := ing.Annotations[consts.ServiceAnnotationMonitorHttpPath]; ok {
					opt.HealthMonitor.HealthCheckPath = PointerOf(option)
				}
				if option, ok := ing.Annotations[consts.ServiceAnnotationMonitorHttpSuccessCode]; ok {
					opt.HealthMonitor.SuccessCode = PointerOf(option)
				}
				if option, ok := ing.Annotations[consts.ServiceAnnotationMonitorHttpVersion]; ok {
					switch option {
					case string(pool.CreateOptsHealthCheckHttpVersionOptHttp1),
						string(pool.CreateOptsHealthCheckHttpVersionOptHttp1Minor1):
						opt.HealthMonitor.HttpVersion = PointerOf(pool.CreateOptsHealthCheckHttpVersionOpt(option))
					default:
						klog.Warningf("Invalid annotation \"%s\" value, must be one of %s, %s", consts.ServiceAnnotationMonitorHttpVersion,
							pool.CreateOptsHealthCheckHttpVersionOptHttp1,
							pool.CreateOptsHealthCheckHttpVersionOptHttp1Minor1)
					}
				}
				if option, ok := ing.Annotations[consts.ServiceAnnotationMonitorHttpDomainName]; ok {
					opt.HealthMonitor.DomainName = PointerOf(option)
				}
			}
		default:
			klog.Warningf("Invalid annotation \"%s\" value, must be one of %s, %s", consts.ServiceAnnotationMonitorProtocol,
				pool.CreateOptsHealthCheckProtocolOptTCP,
				pool.CreateOptsHealthCheckProtocolOptHTTP)
		}
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationPoolAlgorithm]; ok {
		switch option {
		case string(pool.CreateOptsAlgorithmOptRoundRobin),
			string(pool.CreateOptsAlgorithmOptLeastConn),
			string(pool.CreateOptsAlgorithmOptSourceIP):
			opt.Algorithm = pool.CreateOptsAlgorithmOpt(option)
		default:
			klog.Warningf("Invalid annotation \"%s\" value, must be one of %s, %s, %s", consts.ServiceAnnotationPoolAlgorithm,
				pool.CreateOptsAlgorithmOptRoundRobin,
				pool.CreateOptsAlgorithmOptLeastConn,
				pool.CreateOptsAlgorithmOptSourceIP)
		}
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationHealthyThreshold]; ok {
		opt.HealthMonitor.HealthyThreshold = ParseIntAnnotation(option, consts.ServiceAnnotationHealthyThreshold, opt.HealthMonitor.HealthyThreshold)
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationMonitorUnhealthyThreshold]; ok {
		opt.HealthMonitor.UnhealthyThreshold = ParseIntAnnotation(option, consts.ServiceAnnotationMonitorUnhealthyThreshold, opt.HealthMonitor.UnhealthyThreshold)
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationMonitorTimeout]; ok {
		opt.HealthMonitor.Timeout = ParseIntAnnotation(option, consts.ServiceAnnotationMonitorTimeout, opt.HealthMonitor.Timeout)
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationMonitorInterval]; ok {
		opt.HealthMonitor.Interval = ParseIntAnnotation(option, consts.ServiceAnnotationMonitorInterval, opt.HealthMonitor.Interval)
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationEnableStickySession]; ok {
		switch option {
		case "true", "false":
			boolValue, _ := strconv.ParseBool(option)
			opt.Stickiness = PointerOf(boolValue)
		default:
			klog.Warningf("Invalid annotation \"%s\" value, must be true or false", consts.ServiceAnnotationEnableStickySession)
		}
	}
	if option, ok := ing.Annotations[consts.ServiceAnnotationEnableTLSEncryption]; ok {
		switch option {
		case "true", "false":
			boolValue, _ := strconv.ParseBool(option)
			opt.TLSEncryption = PointerOf(boolValue)
		default:
			klog.Warningf("Invalid annotation \"%s\" value, must be true or false", consts.ServiceAnnotationEnableTLSEncryption)
		}
	}
	return opt
}

func ParseIntAnnotation(s, annotation string, defaultValue int) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		klog.Warningf("Invalid annotation \"%s\" value, use default value = %d", annotation, defaultValue)
		return defaultValue
	}
	return i
}
