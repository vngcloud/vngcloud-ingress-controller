package controller

import (
	"strconv"

	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/listener"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/loadbalancer"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/pool"
	nwv1 "k8s.io/api/networking/v1"
	"k8s.io/klog/v2"
)

const (
	DEFAULT_HASH_NAME_LENGTH          = 5       // a unique hash name
	DEFAULT_PORTAL_DESCRIPTION_LENGTH = 255     // All the description must be less than 255 characters
	DEFAULT_LB_PREFIX_NAME            = "vks" // "vks" is abbreviated of "cluster"
	DEFAULT_NAME_DEFAULT_POOL         = "vks_default_pool"
	DEFAULT_PACKAGE_ID                = "lbp-f562b658-0fd4-4fa6-9c57-c1a803ccbf86"
	DEFAULT_HTTPS_LISTENER_NAME       = "vks_https_listener"
	DEFAULT_HTTP_LISTENER_NAME        = "vks_http_listener"
)

func PointerOf[T any](t T) *T {
	return &t
}

func CreateLoadbalancerOptions(ing *nwv1.Ingress) *loadbalancer.CreateOpts {
	opt := &loadbalancer.CreateOpts{
		Name:      "",
		PackageID: DEFAULT_PACKAGE_ID,
		Scheme:    loadbalancer.CreateOptsSchemeOptInternet,
		SubnetID:  "",
		Type:      loadbalancer.CreateOptsTypeOptLayer7,
	}
	if option, ok := ing.Annotations[ServiceAnnotationLoadBalancerName]; ok {
		opt.Name = option
	}
	if option, ok := ing.Annotations[ServiceAnnotationPackageID]; ok {
		opt.PackageID = option
	}
	if option, ok := ing.Annotations[ServiceAnnotationLoadBalancerInternal]; ok {
		switch option {
		case "true":
			opt.Scheme = loadbalancer.CreateOptsSchemeOptInternal
		case "false":
			opt.Scheme = loadbalancer.CreateOptsSchemeOptInternet
		default:
			klog.Warningf("Invalid annotation \"%s\" value, must be true or false", ServiceAnnotationLoadBalancerInternal)
		}
	}
	return opt
}

func CreateListenerOptions(ing *nwv1.Ingress, isHTTPS bool) *listener.CreateOpts {
	opt := &listener.CreateOpts{
		ListenerName:                DEFAULT_HTTP_LISTENER_NAME,
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
		opt.ListenerName = DEFAULT_HTTPS_LISTENER_NAME
		opt.ListenerProtocol = listener.CreateOptsListenerProtocolOptHTTPS
		opt.ListenerProtocolPort = 443

	}
	if ing == nil {
		return opt
	}
	if option, ok := ing.Annotations[ServiceAnnotationIdleTimeoutClient]; ok {
		opt.TimeoutClient = ParseIntAnnotation(option, ServiceAnnotationIdleTimeoutClient, opt.TimeoutClient)
	}
	if option, ok := ing.Annotations[ServiceAnnotationIdleTimeoutMember]; ok {
		opt.TimeoutMember = ParseIntAnnotation(option, ServiceAnnotationIdleTimeoutMember, opt.TimeoutMember)
	}
	if option, ok := ing.Annotations[ServiceAnnotationIdleTimeoutConnection]; ok {
		opt.TimeoutConnection = ParseIntAnnotation(option, ServiceAnnotationIdleTimeoutConnection, opt.TimeoutConnection)
	}
	if option, ok := ing.Annotations[ServiceAnnotationListenerAllowedCIDRs]; ok {
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
	if option, ok := ing.Annotations[ServiceAnnotationMonitorProtocol]; ok {
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
				if option, ok := ing.Annotations[ServiceAnnotationMonitorHttpMethod]; ok {
					switch option {
					case string(pool.CreateOptsHealthCheckMethodOptGET),
						string(pool.CreateOptsHealthCheckMethodOptPUT),
						string(pool.CreateOptsHealthCheckMethodOptPOST):
						opt.HealthMonitor.HealthCheckMethod = PointerOf(pool.CreateOptsHealthCheckMethodOpt(option))
					default:
						klog.Warningf("Invalid annotation \"%s\" value, must be one of %s, %s, %s", ServiceAnnotationMonitorHttpMethod,
							pool.CreateOptsHealthCheckMethodOptGET,
							pool.CreateOptsHealthCheckMethodOptPUT,
							pool.CreateOptsHealthCheckMethodOptPOST)
					}
				}
				if option, ok := ing.Annotations[ServiceAnnotationMonitorHttpPath]; ok {
					opt.HealthMonitor.HealthCheckPath = PointerOf(option)
				}
				if option, ok := ing.Annotations[ServiceAnnotationMonitorHttpSuccessCode]; ok {
					opt.HealthMonitor.SuccessCode = PointerOf(option)
				}
				if option, ok := ing.Annotations[ServiceAnnotationMonitorHttpVersion]; ok {
					switch option {
					case string(pool.CreateOptsHealthCheckHttpVersionOptHttp1),
						string(pool.CreateOptsHealthCheckHttpVersionOptHttp1Minor1):
						opt.HealthMonitor.HttpVersion = PointerOf(pool.CreateOptsHealthCheckHttpVersionOpt(option))
					default:
						klog.Warningf("Invalid annotation \"%s\" value, must be one of %s, %s", ServiceAnnotationMonitorHttpVersion,
							pool.CreateOptsHealthCheckHttpVersionOptHttp1,
							pool.CreateOptsHealthCheckHttpVersionOptHttp1Minor1)
					}
				}
				if option, ok := ing.Annotations[ServiceAnnotationMonitorHttpDomainName]; ok {
					opt.HealthMonitor.DomainName = PointerOf(option)
				}
			}
		default:
			klog.Warningf("Invalid annotation \"%s\" value, must be one of %s, %s", ServiceAnnotationMonitorProtocol,
				pool.CreateOptsHealthCheckProtocolOptTCP,
				pool.CreateOptsHealthCheckProtocolOptHTTP)
		}
	}
	if option, ok := ing.Annotations[ServiceAnnotationPoolAlgorithm]; ok {
		switch option {
		case string(pool.CreateOptsAlgorithmOptRoundRobin),
			string(pool.CreateOptsAlgorithmOptLeastConn),
			string(pool.CreateOptsAlgorithmOptSourceIP):
			opt.Algorithm = pool.CreateOptsAlgorithmOpt(option)
		default:
			klog.Warningf("Invalid annotation \"%s\" value, must be one of %s, %s, %s", ServiceAnnotationPoolAlgorithm,
				pool.CreateOptsAlgorithmOptRoundRobin,
				pool.CreateOptsAlgorithmOptLeastConn,
				pool.CreateOptsAlgorithmOptSourceIP)
		}
	}
	if option, ok := ing.Annotations[ServiceAnnotationHealthyThreshold]; ok {
		opt.HealthMonitor.HealthyThreshold = ParseIntAnnotation(option, ServiceAnnotationHealthyThreshold, opt.HealthMonitor.HealthyThreshold)
	}
	if option, ok := ing.Annotations[ServiceAnnotationMonitorUnhealthyThreshold]; ok {
		opt.HealthMonitor.UnhealthyThreshold = ParseIntAnnotation(option, ServiceAnnotationMonitorUnhealthyThreshold, opt.HealthMonitor.UnhealthyThreshold)
	}
	if option, ok := ing.Annotations[ServiceAnnotationMonitorTimeout]; ok {
		opt.HealthMonitor.Timeout = ParseIntAnnotation(option, ServiceAnnotationMonitorTimeout, opt.HealthMonitor.Timeout)
	}
	if option, ok := ing.Annotations[ServiceAnnotationMonitorInterval]; ok {
		opt.HealthMonitor.Interval = ParseIntAnnotation(option, ServiceAnnotationMonitorInterval, opt.HealthMonitor.Interval)
	}
	if option, ok := ing.Annotations[ServiceAnnotationEnableStickySession]; ok {
		switch option {
		case "true", "false":
			boolValue, _ := strconv.ParseBool(option)
			opt.Stickiness = PointerOf(boolValue)
		default:
			klog.Warningf("Invalid annotation \"%s\" value, must be true or false", ServiceAnnotationEnableStickySession)
		}
	}
	if option, ok := ing.Annotations[ServiceAnnotationEnableTLSEncryption]; ok {
		switch option {
		case "true", "false":
			boolValue, _ := strconv.ParseBool(option)
			opt.TLSEncryption = PointerOf(boolValue)
		default:
			klog.Warningf("Invalid annotation \"%s\" value, must be true or false", ServiceAnnotationEnableTLSEncryption)
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
