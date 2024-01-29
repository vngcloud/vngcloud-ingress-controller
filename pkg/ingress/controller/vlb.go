package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/loadbalancers"
	"github.com/sirupsen/logrus"
	"github.com/vngcloud/vngcloud-go-sdk/client"
	vconSdkClient "github.com/vngcloud/vngcloud-go-sdk/client"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud"
	lObjects "github.com/vngcloud/vngcloud-go-sdk/vngcloud/objects"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/identity/v2/extensions/oauth2"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/identity/v2/tokens"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/listener"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/loadbalancer"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/policy"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/pool"
	apiv1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	"k8s.io/cloud-provider-openstack/pkg/ingress/config"
	"k8s.io/cloud-provider-openstack/pkg/ingress/consts"
	"k8s.io/cloud-provider-openstack/pkg/ingress/utils/errors"
	"k8s.io/cloud-provider-openstack/pkg/ingress/utils/metadata"
	"k8s.io/klog/v2"
)

type (
	ExtraInfo struct {
		ProjectID string
		UserID    int64
	}
)

//type ILBProvider interface {
//	Init() error
//
//	// GetLoadbalancerByID returns the load balancer with the given name (list all lb in subnet and filter by name)
//	GetLoadbalancerByID(lbID string) (*loadbalancers.LoadBalancer, error)
//
//	// Update lb memebers when node change
//	UpdateLoadbalancerMembers(lbID string, nodes []*apiv1.Node) error
//
//	// GetLoadbalancerIDByIngress returns the load balancer id with the given ingress
//	GetLoadbalancerIDByIngress(ing *nwv1.Ingress) string
//
//	EnsureFloatingIP(needDelete bool, portID string, floatingIPNetwork string, description string) (string, error)
//	DeleteLoadbalancer(lbID string, cascade bool) error
//
//	// EnsureLoadBalancer creates a new load balancer or updates an existing one.
//	EnsureLoadBalancer(con *Controller, ing *nwv1.Ingress) (*loadbalancers.LoadBalancer, error)
//	EnsureListener(name string, lbID string, secretRefs []string, listenerAllowedCIDRs []string, timeoutClientData, timeoutMemberData, timeoutTCPInspect, timeoutMemberConnect *int) (*listeners.Listener, error)
//
//	GetL7policies(listenerID string) ([]l7policies.L7Policy, error)
//	GetL7Rules(policyID string) ([]l7policies.Rule, error)
//	GetPools(lbID string) ([]pools.Pool, error)
//
//	// UpdateLoadBalancerDescription updates the load balancer description field.
//	UpdateLoadBalancerDescription(lbID string, newDescription string) error
//}

type VLBProvider struct {
	config *config.Config

	provider  *vconSdkClient.ProviderClient
	vLBSC     *client.ServiceClient
	vServerSC *client.ServiceClient

	cluster     *lObjects.Cluster
	lbsInSubnet []*lObjects.LoadBalancer

	extraInfo    *ExtraInfo
	metadataOpts metadata.Opts
	api          API
}

type PoolExpander struct {
	isInUse bool
	Name    string
	UUID    string
	// Members []lObjects.Member
	Members []pool.Member
	// Protocol          string
	// Description       string
	// LoadBalanceMethod string
	// Status            string
	// Stickiness        bool
	// TLSEncryption     bool
}

type PolicyExpander struct {
	isInUse         bool
	isHttpsListener bool
	listenerID      string
	// shouldForwardToPoolName string

	UUID             string
	Name             string
	RedirectPoolID   string
	RedirectPoolName string
	Action           policy.PolicyOptsActionOpt
	L7Rules          []policy.Rule
	// *lObjects.Policy
}

type ListenerExpander struct {
	defaultPoolName string
	defaultPoolId   string
	UUID            string
	listener.CreateOpts
}
type IngressInspect struct {
	PolicyExpander   []*PolicyExpander
	PoolExpander     []*PoolExpander
	ListenerExpander []*ListenerExpander
}

func (ing *IngressInspect) Print() {
	for _, l := range ing.ListenerExpander {
		fmt.Println("LISTENER: id:", l.UUID, "defaultPoolName:", l.defaultPoolName, "defaultPoolId:", l.defaultPoolId)
	}
	for _, p := range ing.PolicyExpander {
		fmt.Println("---- POLICY: id:", p.UUID, "name:", p.Name, "redirectPoolName:", p.RedirectPoolName, "redirectPoolId:", p.RedirectPoolID, "action:", p.Action, "l7Rules:", p.L7Rules)
	}
	for _, p := range ing.PoolExpander {
		fmt.Println("++++ POOL: name:", p.Name, "uuid:", p.UUID, "members:", p.Members)
	}
}

func (c *VLBProvider) Init() error {
	c.api = API{}
	provider, err := vngcloud.NewClient(c.config.Global.IdentityURL)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("failed to init VNGCLOUD client")
	}
	err = vngcloud.Authenticate(provider, &oauth2.AuthOptions{
		ClientID:     c.config.Global.ClientID,
		ClientSecret: c.config.Global.ClientSecret,
		AuthOptionsBuilder: &tokens.AuthOptions{
			IdentityEndpoint: c.config.Global.IdentityURL,
		},
	})
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("failed to Authenticate VNGCLOUD client")
	}
	c.provider = provider

	vlbSC, err := vngcloud.NewServiceClient(
		"https://hcm-3.api.vngcloud.vn/vserver/vlb-gateway/v2",
		provider, "vlb-gateway")
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("failed to init VLB VNGCLOUD client")
	}
	c.vLBSC = vlbSC

	vserverSC, err := vngcloud.NewServiceClient(
		"https://hcm-3.api.vngcloud.vn/vserver/vserver-gateway/v2",
		provider, "vserver-gateway")
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("failed to init VSERVER VNGCLOUD client")
	}
	c.vServerSC = vserverSC

	c.setUpPortalInfo()
	c.cluster, err = c.api.GetClusterInfo(c.vServerSC, c.extraInfo.ProjectID, c.config.ClusterID)
	c.ListLoadBalancerBySubnetID()

	return nil
}

func (c *VLBProvider) GetLoadbalancerByID(lbID string) (*loadbalancers.LoadBalancer, error) {
	c.ListLoadBalancerBySubnetID()

	for _, lb := range c.lbsInSubnet {
		if lb.UUID == lbID {
			return &loadbalancers.LoadBalancer{
				ID:              lb.UUID,
				VipAddress:      lb.Address,
				Name:            lb.Name,
				OperatingStatus: lb.Status,
			}, nil
		}
	}
	return nil, nil
}

func (c *VLBProvider) UpdateLoadbalancerMembers(lbID string, nodes []*apiv1.Node) error {
	// for every pools, except the default pool, update the members with the new nodes id
	// ..........................................................
	// how to find the default pool?
	return nil
}

func (c *VLBProvider) GetLoadbalancerIDByIngress(ing *nwv1.Ingress) (string, error) {
	klog.Infof("----------------- GetLoadbalancerIDByIngress(%s/%s) ------------------", ing.Namespace, ing.Name)
	c.ListLoadBalancerBySubnetID()
	// check in annotation
	if lbID, ok := ing.Annotations[ServiceAnnotationLoadBalancerID]; ok {
		logrus.Infof("have annotation lbID: %s", lbID)
		for _, lb := range c.lbsInSubnet {
			if lb.UUID == lbID {
				logrus.Infof("found lbID: %s", lbID)
				return lb.UUID, nil
			}
		}
		logrus.Infof("have annotation but not found lbID: %s", lbID)
		return "", errors.ErrLoadBalancerIDNotFoundAnnotation
	}

	// check in list lb name
	lbName := c.GetResourceName(ing)
	for _, lb := range c.lbsInSubnet {
		if lb.Name == lbName {
			logrus.Infof("Found lb match Name: %s", lbName)
			return lb.UUID, nil
		}
	}
	logrus.Infof("Not found lb match Name: %s", lbName)
	return "", nil
}

func (c *VLBProvider) DeleteLoadbalancer(con *Controller, ing *nwv1.Ingress) error {
	klog.Infof("----------------- DeleteLoadbalancer(%s/%s) ------------------", ing.Namespace, ing.Name)
	lbID, err := c.ensureLoadBalancer(ing)
	if err != nil {
		logrus.Errorln("error when ensure loadbalancer", err)
		return err
	}

	oldIngExpander, err := c.InspectIngress(con, ing)
	if err != nil {
		logrus.Errorln("error when inspect old ingress", err)
		return err
	}
	newIngExpander, err := c.InspectIngress(con, nil)
	if err != nil {
		logrus.Errorln("error when inspect new ingress", err)
		return err
	}

	_, err = c.ActionCompareIngress(lbID, oldIngExpander, newIngExpander)
	if err != nil {
		logrus.Errorln("error when compare ingress", err)
		return err
	}
	// can delete lb instance here ......................
	return nil
}

// /////////////////////////////////// PRIVATE METHOD /////////////////////////////////////////
func (c *VLBProvider) mapHostTLS(ing *nwv1.Ingress) (map[string]bool, []string) {
	m := make(map[string]bool)
	certArr := make([]string, 0)
	for _, tls := range ing.Spec.TLS {
		for _, host := range tls.Hosts {
			certArr = append(certArr, strings.TrimSpace(tls.SecretName))
			m[host] = true
		}
	}
	return m, certArr
}

func (c *VLBProvider) InspectCurrentLB(lbID string) (*IngressInspect, error) {
	expectPolicyName := make([]*PolicyExpander, 0)
	expectPoolName := make([]*PoolExpander, 0)
	expectListenerName := make([]*ListenerExpander, 0)

	liss, err := c.api.ListListenerOfLB(c.vLBSC, c.extraInfo.ProjectID, lbID)
	if err != nil {
		logrus.Errorln("error when list listener of lb", err)
		return nil, err
	}
	for _, lis := range liss {
		listenerOpts := consts.OPT_LISTENER_HTTP_DEFAULT
		if lis.Protocol == "HTTPS" {
			listenerOpts = consts.OPT_LISTENER_HTTPS_DEFAULT
		}
		listenerOpts.DefaultPoolId = lis.DefaultPoolId
		expectListenerName = append(expectListenerName, &ListenerExpander{
			UUID:            lis.UUID,
			defaultPoolName: lis.DefaultPoolName,
			defaultPoolId:   lis.DefaultPoolId,
			CreateOpts:      listenerOpts,
		})

	}

	getPools, err := c.api.ListPoolOfLB(c.vLBSC, c.extraInfo.ProjectID, lbID)
	if err != nil {
		logrus.Errorln("error when list pool of lb", err)
		return nil, err
	}
	for _, p := range getPools {
		poolMembers := make([]pool.Member, 0)
		for _, m := range p.Members {
			poolMembers = append(poolMembers, pool.Member{
				IpAddress:   m.Address,
				Port:        m.ProtocolPort,
				Backup:      m.Backup,
				Weight:      m.Weight,
				Name:        m.Name,
				MonitorPort: m.MonitorPort,
			})
		}
		expectPoolName = append(expectPoolName, &PoolExpander{
			isInUse: false,
			Name:    p.Name,
			UUID:    p.UUID,
			Members: poolMembers,
		})
	}

	for _, lis := range liss {
		pols, err := c.api.ListPolicyOfListener(c.vLBSC, c.extraInfo.ProjectID, lbID, lis.UUID)
		if err != nil {
			logrus.Errorln("error when list policy of listener", err)
			return nil, err
		}
		for _, pol := range pols {
			l7Rules := make([]policy.Rule, 0)
			for _, r := range pol.L7Rules {
				l7Rules = append(l7Rules, policy.Rule{
					CompareType: policy.PolicyOptsCompareTypeOpt(r.CompareType),
					RuleValue:   r.RuleValue,
					RuleType:    policy.PolicyOptsRuleTypeOpt(r.RuleType),
				})
			}
			expectPolicyName = append(expectPolicyName, &PolicyExpander{
				isInUse:          false,
				listenerID:       lis.UUID,
				UUID:             pol.UUID,
				Name:             pol.Name,
				RedirectPoolID:   pol.RedirectPoolID,
				RedirectPoolName: pol.RedirectPoolName,
				Action:           policy.PolicyOptsActionOpt(pol.Action),
				L7Rules:          l7Rules,
			})
		}
	}
	return &IngressInspect{
		PolicyExpander:   expectPolicyName,
		PoolExpander:     expectPoolName,
		ListenerExpander: expectListenerName,
	}, nil
}
func (c *VLBProvider) InspectIngress(con *Controller, ing *nwv1.Ingress) (*IngressInspect, error) {
	if ing == nil {
		return &IngressInspect{
			PolicyExpander:   make([]*PolicyExpander, 0),
			PoolExpander:     make([]*PoolExpander, 0),
			ListenerExpander: make([]*ListenerExpander, 0),
		}, nil
	}
	klog.Infof("----------------- InspectIngress(%s/%s) ------------------", ing.Namespace, ing.Name)
	lb_prefix_name := c.GetResourceName(ing)
	mapTLS, certArr := c.mapHostTLS(ing)

	expectPolicyName := make([]*PolicyExpander, 0)
	expectPoolName := make([]*PoolExpander, 0)
	expectListenerName := make([]*ListenerExpander, 0)

	GetPoolExpander := func(service *nwv1.IngressServiceBackend) (*PoolExpander, error) {
		serviceName := fmt.Sprintf("%s/%s", ing.ObjectMeta.Namespace, service.Name)
		klog.Infof("serviceName: %v", serviceName)
		poolName := c.GetPoolName(lb_prefix_name, serviceName, int(service.Port.Number))
		nodePort, err := con.getServiceNodePort(serviceName, service)
		if err != nil {
			klog.Errorf("error when get node port: %v", err)
			return nil, err
		}
		klog.Infof("nodePort: %v", nodePort)

		membersAddr, _ := con.GetNodeMembersAddr()
		klog.Infof("membersAddr: %v", membersAddr)
		members := make([]pool.Member, 0)
		for _, addr := range membersAddr {
			members = append(members, pool.Member{
				IpAddress:   addr,
				Port:        nodePort,
				Backup:      false,
				Weight:      1,
				Name:        addr,
				MonitorPort: nodePort,
			})
		}
		return &PoolExpander{
			isInUse: false,
			Name:    poolName,
			UUID:    "",
			Members: members,
		}, nil
	}

	// check if have default pool
	defaultPoolName := ""
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		defaultPoolExpander, err := GetPoolExpander(ing.Spec.DefaultBackend.Service)
		if err != nil {
			logrus.Errorln("error when get default pool expander", err)
			return nil, err
		}
		expectPoolName = append(expectPoolName, defaultPoolExpander)
		defaultPoolName = defaultPoolExpander.Name
	}

	isHaveHTTPListener := false
	isHaveHTTPSListener := false
	for ruleIndex, rule := range ing.Spec.Rules {
		_, isHttpsListener := mapTLS[rule.Host]
		if isHttpsListener && !isHaveHTTPSListener {
			listenerOpts := consts.OPT_LISTENER_HTTPS_DEFAULT
			listenerOpts.CertificateAuthorities = &certArr
			listenerOpts.DefaultCertificateAuthority = &certArr[0]
			listenerOpts.ClientCertificate = consts.PointerOf[string]("")
			expectListenerName = append(expectListenerName, &ListenerExpander{
				defaultPoolName: defaultPoolName,
				CreateOpts:      listenerOpts,
			})
		}
		if !isHttpsListener && !isHaveHTTPListener {
			expectListenerName = append(expectListenerName, &ListenerExpander{
				defaultPoolName: defaultPoolName,
				CreateOpts:      consts.OPT_LISTENER_HTTP_DEFAULT,
			})
		}

		for pathIndex, path := range rule.HTTP.Paths {
			policyName := c.GetPolicyName(lb_prefix_name, isHttpsListener, ruleIndex, pathIndex)

			poolExpander, err := GetPoolExpander(path.Backend.Service)
			if err != nil {
				logrus.Errorln("error when get pool expander", err)
				return nil, err
			}
			expectPoolName = append(expectPoolName, poolExpander)

			// ensure policy
			newRules := []policy.Rule{
				{
					RuleType:    policy.PolicyOptsRuleTypeOptPATH,
					CompareType: policy.PolicyOptsCompareTypeOptEQUALS,
					RuleValue:   path.Path,
				},
			}
			if rule.Host != "" {
				newRules = append(newRules, policy.Rule{
					RuleType:    policy.PolicyOptsRuleTypeOptHOSTNAME,
					CompareType: policy.PolicyOptsCompareTypeOptEQUALS,
					RuleValue:   rule.Host,
				})
			}
			expectPolicyName = append(expectPolicyName, &PolicyExpander{
				isHttpsListener:  isHttpsListener,
				isInUse:          false,
				UUID:             "",
				Name:             policyName,
				RedirectPoolID:   "",
				RedirectPoolName: poolExpander.Name,
				Action:           policy.PolicyOptsActionOptREDIRECTTOPOOL,
				L7Rules:          newRules,
			})
		}
	}
	return &IngressInspect{
		PolicyExpander:   expectPolicyName,
		PoolExpander:     expectPoolName,
		ListenerExpander: expectListenerName,
	}, nil
}

func (c *VLBProvider) EnsureLoadBalancer(con *Controller, oldIng, ing *nwv1.Ingress) (*lObjects.LoadBalancer, error) {
	klog.Infof("----------------- EnsureLoadBalancer(%s/%s) ------------------", ing.Namespace, ing.Name)
	lbID, err := c.ensureLoadBalancer(ing)
	if err != nil {
		logrus.Errorln("error when ensure loadbalancer", err)
		return nil, err
	}

	oldIngExpander, err := c.InspectIngress(con, oldIng)
	if err != nil {
		logrus.Errorln("error when inspect old ingress", err)
		return nil, err
	}
	newIngExpander, err := c.InspectIngress(con, ing)
	if err != nil {
		logrus.Errorln("error when inspect new ingress", err)
		return nil, err
	}

	lb, err := c.ActionCompareIngress(lbID, oldIngExpander, newIngExpander)
	if err != nil {
		logrus.Errorln("error when compare ingress", err)
		return nil, err
	}
	return lb, nil
}

// find or create lb
func (c *VLBProvider) ensureLoadBalancer(ing *nwv1.Ingress) (string, error) {

	lbID, err := c.GetLoadbalancerIDByIngress(ing)
	if err != nil {
		if err == errors.ErrLoadBalancerIDNotFoundAnnotation {
			return "", err
		}

		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("error not handled when list loadbalancer by subnet id")
	}

	if lbID == "" {
		klog.Infof("--------------- create new lb for ingress %s/%s -------------------", ing.Namespace, ing.Name)
		lbName := c.GetResourceName(ing)
		packageID := getStringFromIngressAnnotation(ing, ServiceAnnotationPackageID, consts.DEFAULT_PACKAGE_ID)

		lb, err := c.api.CreateLB(c.vLBSC,
			lbName, packageID, c.cluster.SubnetID, c.extraInfo.ProjectID,
			loadbalancer.CreateOptsSchemeOptInternet,
			loadbalancer.CreateOptsTypeOptLayer7)
		if err != nil {
			klog.Errorf("error when create new lb: %v", err)
			return "", err
		}
		lbID = lb.UUID
	}
	return lbID, nil
}

func (c *VLBProvider) ActionCompareIngress(lbID string, oldIngExpander, newIngExpander *IngressInspect) (*lObjects.LoadBalancer, error) {
	lb := c.WaitForLBActive(lbID)

	curLBExpander, err := c.InspectCurrentLB(lbID)
	if err != nil {
		logrus.Errorln("error when inspect current lb", err)
		return nil, err
	}
	logrus.Infoln("curLBExpander:")
	curLBExpander.Print()

	c.MapIDExpander(oldIngExpander, curLBExpander)
	logrus.Infoln("oldIngExpander:")
	oldIngExpander.Print()

	// ensure all from newIngExpander
	mapPoolNameIndex := make(map[string]int)
	for poolIndex, ipool := range newIngExpander.PoolExpander {
		newPool, err := c.ensurePool(lb.UUID, ipool.Name, false)
		if err != nil {
			logrus.Errorln("error when ensure pool", err)
			return nil, err
		}
		ipool.UUID = newPool.UUID
		mapPoolNameIndex[ipool.Name] = poolIndex
		_, err = c.ensurePoolMember(lb.UUID, newPool.UUID, ipool.Members)
		if err != nil {
			logrus.Errorln("error when ensure pool member", err)
			return nil, err
		}
	}
	mapListenerNameIndex := make(map[string]int)
	for listenerIndex, ilistener := range newIngExpander.ListenerExpander {
		poolID := ""
		if ilistener.defaultPoolName != "" {
			poolIndex, isHave := mapPoolNameIndex[ilistener.defaultPoolName]
			if !isHave {
				logrus.Errorf(".........pool not found in listener: %v", ilistener.defaultPoolName)
				return nil, err
			}
			poolID = newIngExpander.PoolExpander[poolIndex].UUID
		}
		ilistener.CreateOpts.DefaultPoolId = poolID

		lis, err := c.ensureListener(lb.UUID, ilistener.CreateOpts.ListenerName, ilistener.CreateOpts, false)
		if err != nil {
			logrus.Errorln("error when ensure listener:", ilistener.CreateOpts.ListenerName, err)
			return nil, err
		}
		ilistener.UUID = lis.UUID
		mapListenerNameIndex[ilistener.CreateOpts.ListenerName] = listenerIndex
	}
	logrus.Infof("newIngExpander: %v", newIngExpander.PolicyExpander)
	for _, ipolicy := range newIngExpander.PolicyExpander {
		// get pool name from redirect pool name
		poolIndex, isHave := mapPoolNameIndex[ipolicy.RedirectPoolName]
		if !isHave {
			logrus.Errorf(".........pool not found in policy: %v", ipolicy.RedirectPoolName)
			return nil, err
		}
		poolID := newIngExpander.PoolExpander[poolIndex].UUID
		listenerName := consts.DEFAULT_HTTP_LISTENER_NAME
		if ipolicy.isHttpsListener {
			listenerName = consts.DEFAULT_HTTPS_LISTENER_NAME
		}
		listenerIndex, isHave := mapListenerNameIndex[listenerName]
		if !isHave {
			logrus.Errorf("listener index not found: %v", listenerName)
			return nil, err
		}
		listenerID := newIngExpander.ListenerExpander[listenerIndex].UUID
		if listenerID == "" {
			logrus.Errorf("listenerID not found: %v", listenerName)
			return nil, err
		}

		policyOpts := &policy.CreateOptsBuilder{
			Name:           ipolicy.Name,
			Action:         ipolicy.Action,
			RedirectPoolID: poolID,
			Rules:          ipolicy.L7Rules,
		}
		_, err := c.ensurePolicy(lb.UUID, listenerID, ipolicy.Name, policyOpts, false)
		if err != nil {
			logrus.Errorln("error when ensure policy", err)
			return nil, err
		}
	}

	// delete redundant policy and pool if in oldIng
	// with id from curLBExpander
	klog.Infof("*************** DELETE REDUNDANT POLICY AND POOL *****************")
	policyWillUse := make(map[string]int)
	for policyIndex, pol := range newIngExpander.PolicyExpander {
		policyWillUse[pol.Name] = policyIndex
	}
	logrus.Infof("policyWillUse: %v", policyWillUse)
	for _, oldIngPolicy := range oldIngExpander.PolicyExpander {
		_, isPolicyWillUse := policyWillUse[oldIngPolicy.Name]
		if !isPolicyWillUse {
			logrus.Warnf("policy not in use: %v, delete", oldIngPolicy.Name)
			_, err := c.ensurePolicy(lb.UUID, oldIngPolicy.listenerID, oldIngPolicy.Name, nil, true)
			if err != nil {
				logrus.Errorln("error when ensure policy", err)
				// maybe it's already deleted
				// return nil, err
			}
		} else {
			logrus.Infof("policy in use: %v, not delete", oldIngPolicy.Name)
		}
	}

	poolWillUse := make(map[string]bool)
	for _, pool := range newIngExpander.PoolExpander {
		poolWillUse[pool.Name] = true
	}
	for _, oldIngPool := range oldIngExpander.PoolExpander {
		_, isPoolWillUse := poolWillUse[oldIngPool.Name]
		if !isPoolWillUse {
			logrus.Warnf("pool not in use: %v, delete", oldIngPool.Name)
			_, err := c.ensurePool(lb.UUID, oldIngPool.Name, true)
			if err != nil {
				logrus.Errorln("error when ensure pool", err)
				// maybe it's already deleted
				// return nil, err
			}
		} else {
			logrus.Infof("pool in use: %v, not delete", oldIngPool.Name)
		}
	}
	return lb, nil
}

// GetResourceName get Ingress related resource name.
func (c *VLBProvider) GetResourceName(ing *nwv1.Ingress) string {
	fullName := fmt.Sprintf("%s_%s_%s", c.config.ClusterName, ing.Namespace, ing.Name)
	hash := HashString(fullName)

	MinInt := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	trim := func(str string, length int) string {
		return str[:MinInt(len(str), length)]
	}
	return fmt.Sprintf("annd_%s", trim(hash, 10))
	// return fmt.Sprintf("annd2_%s_%s_%s",
	// 	trim(c.config.ClusterName, 10),
	// 	trim(ing.Name, 10),
	// 	trim(hash, 10),
	// )
}
func (c *VLBProvider) GetPolicyName(prefix string, mode bool, ruleIndex, pathIndex int) string {
	return fmt.Sprintf("%s_%t_r%d_p%d", prefix, mode, ruleIndex, pathIndex)
}
func (c *VLBProvider) GetPoolName(prefix, serviceName string, port int) string {
	return fmt.Sprintf("%s_%s_%d", prefix, strings.ReplaceAll(serviceName, "/", "-"), port)
}

func (c *VLBProvider) setUpPortalInfo() {
	c.config.Metadata = getMetadataOption(metadata.Opts{})
	metadator := metadata.GetMetadataProvider(c.config.Metadata.SearchOrder)
	extraInfo, err := setupPortalInfo(
		c.provider,
		metadator,
		"https://hcm-3.api.vngcloud.vn/vserver/vserver-gateway/v1")
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("failed to setup portal info")
	}
	c.extraInfo = extraInfo
}

func (c *VLBProvider) ensurePool(lbID, poolName string, isDelete bool) (*lObjects.Pool, error) {
	klog.Infof("------------ ensurePool: %s", poolName)
	// c.WaitForLBActive(lbID) // ..................................................
	pool, err := c.FindPoolByName(lbID, poolName)
	if err != nil {
		if err == errors.ErrNotFound {
			if isDelete {
				logrus.Infof("pool not found: %s, maybe deleted", poolName)
				return nil, nil
			}
			newPoolOpts := consts.OPT_POOL_DEFAULT
			newPoolOpts.PoolName = poolName
			newPool, err := c.api.CreatePool(c.vLBSC, c.extraInfo.ProjectID, lbID, newPoolOpts)
			if err != nil {
				logrus.Errorln("error when create new pool", err)
				return nil, err
			}
			pool = newPool
		} else {
			logrus.Errorln("error when find pool", err)
			return nil, err
		}
	}
	if isDelete {
		err := c.api.DeletePool(c.vLBSC, c.extraInfo.ProjectID, lbID, pool.UUID)
		if err != nil {
			logrus.Errorln("error when delete pool", err)
			return nil, err
		}
	}
	c.WaitForLBActive(lbID)
	return pool, nil
}

func (c *VLBProvider) ensurePoolMember(lbID, poolID string, members []pool.Member) (*lObjects.Pool, error) {
	klog.Infof("------------ ensurePoolMember: %s", poolID)
	memsGet, err := c.api.GetMemberPool(c.vLBSC, c.extraInfo.ProjectID, lbID, poolID)
	if err != nil {
		logrus.Errorln("error when get pool members", err)
		return nil, err
	}
	comparePoolMembers := func(p1 []pool.Member, p2 []*lObjects.Member) bool {
		if len(p1) != len(p2) {
			return false
		}
		checkIfExist := func(mems []*lObjects.Member, mem pool.Member) bool {
			for _, r := range mems {
				if r.Address == mem.IpAddress &&
					r.ProtocolPort == mem.Port &&
					r.MonitorPort == mem.MonitorPort &&
					r.Backup == mem.Backup &&
					r.Name == mem.Name &&
					r.Weight == mem.Weight {
					return true
				}
			}
			return false
		}
		for _, p := range p1 {
			if !checkIfExist(p2, p) {
				logrus.Infof("member in pool not exist: %v", p)
				return false
			}
		}
		return true
	}
	if !comparePoolMembers(members, memsGet) {
		err := c.api.UpdatePoolMember(c.vLBSC, c.extraInfo.ProjectID, lbID, poolID, members)
		if err != nil {
			logrus.Errorln("error when update pool members", err)
			return nil, err
		}
	}

	c.WaitForLBActive(lbID)
	return nil, nil
}

func (c *VLBProvider) ensureListener(lbID, lisName string, listenerOpts listener.CreateOpts, isDelete bool) (*lObjects.Listener, error) {
	klog.Infof("------------ ensureListener ----------")
	lis, err := c.FindListenerByName(lbID, lisName)
	if err != nil {
		if err == errors.ErrNotFound {
			if isDelete {
				logrus.Infof("listener not found: %s, maybe deleted", lisName)
				return nil, nil
			}
			// create listener point to default pool
			listenerOpts.ListenerName = lisName
			listener, err := c.api.CreateListener(c.vLBSC, c.extraInfo.ProjectID, lbID, &listenerOpts)
			if err != nil {
				logrus.Fatal("error when create listener", err)
				return nil, err
			}
			lis = listener
		} else {
			logrus.Errorln("error when find listener", err)
			return nil, err
		}
	}
	if isDelete {
		err := c.api.DeleteListener(c.vLBSC, c.extraInfo.ProjectID, lbID, lis.UUID)
		if err != nil {
			logrus.Errorln("error when delete listener", err)
			return nil, err
		}
	}
	c.WaitForLBActive(lbID)
	return lis, nil
}

func (c *VLBProvider) ensurePolicy(lbID, listenerID, policyName string, policyOpt *policy.CreateOptsBuilder, isDelete bool) (*lObjects.Policy, error) {
	klog.Infof("------------ ensurePolicy: %s", policyName)
	FindPolicyByName := func(lbID, lisID, name string) (*lObjects.Policy, error) {
		klog.Infof("------------ FindPolicyByName: lbID: %s, lisID: %s, name: %s", lbID, lisID, name)
		policyArr, err := c.api.ListPolicyOfListener(c.vLBSC, c.extraInfo.ProjectID, lbID, lisID)
		if err != nil {
			logrus.Errorln("error when list policy", err)
			return nil, err
		}
		for _, policy := range policyArr {
			if policy.Name == name {
				return policy, nil
			}
		}
		return nil, errors.ErrNotFound
	}

	pol, err := FindPolicyByName(lbID, listenerID, policyName)
	if err != nil {
		if err == errors.ErrNotFound {
			if isDelete {
				logrus.Infof("policy not found: %s, maybe deleted", policyName)
				return nil, nil
			}
			newPolicy, err := c.api.CreatePolicy(c.vLBSC, c.extraInfo.ProjectID, lbID, listenerID, policyOpt)
			if err != nil {
				logrus.Fatal("error when create policy", err)
				return nil, err
			}
			pol = newPolicy
		} else {
			logrus.Errorln("error when find policy", err)
			return nil, err
		}
	} else if isDelete {
		err := c.api.DeletePolicy(c.vLBSC, c.extraInfo.ProjectID, lbID, listenerID, pol.UUID)
		if err != nil {
			logrus.Errorln("error when delete policy", err)
			return nil, err
		}
	} else {
		// get policy and update policy
		newpolicy, err := c.api.GetPolicy(c.vLBSC, c.extraInfo.ProjectID, lbID, listenerID, pol.UUID)
		if err != nil {
			logrus.Fatal("error when get policy", err)
			return nil, err
		}
		comparePolicy := func(p2 *lObjects.Policy) bool {
			if string(policyOpt.Action) != p2.Action ||
				policyOpt.RedirectPoolID != p2.RedirectPoolID ||
				policyOpt.Name != p2.Name {
				return false
			}
			if len(policyOpt.Rules) != len(p2.L7Rules) {
				return false
			}

			checkIfExist := func(rules []*lObjects.L7Rule, rule policy.Rule) bool {
				for _, r := range rules {
					if r.CompareType == string(rule.CompareType) &&
						r.RuleType == string(rule.RuleType) &&
						r.RuleValue == rule.RuleValue {
						return true
					}
				}
				return false
			}
			for _, rule := range policyOpt.Rules {
				if !checkIfExist(p2.L7Rules, rule) {
					logrus.Infof("rule not exist: %v", rule)
					return false
				}
			}
			return true
		}
		if !comparePolicy(newpolicy) {
			updateOpts := &policy.UpdateOptsBuilder{
				Action:         policyOpt.Action,
				RedirectPoolID: policyOpt.RedirectPoolID,
				Rules:          policyOpt.Rules,
			}
			err := c.api.UpdatePolicy(c.vLBSC, c.extraInfo.ProjectID, lbID, listenerID, pol.UUID, updateOpts)
			if err != nil {
				logrus.Fatal("error when update policy", err)
				return nil, err
			}
		}
	}
	c.WaitForLBActive(lbID)
	// pol, err = c.api.GetPolicy(c.vLBSC, c.extraInfo.ProjectID, lbID, listenerID, pol.UUID)
	// if err != nil {
	// 	logrus.Fatal("error when get policy", err)
	// 	return nil, err
	// }
	return pol, nil
}

// API
func (c *VLBProvider) ListLoadBalancerBySubnetID() {
	klog.Infof("--------------- ListLoadBalancerBySubnetID -------------------")
	c.lbsInSubnet, _ = c.api.ListLBBySubnetID(c.vLBSC, c.extraInfo.ProjectID, c.cluster.SubnetID)
	for _, lb := range c.lbsInSubnet {
		klog.Infof("lb: %v", lb)
	}
}

func (c *VLBProvider) WaitForLBActive(lbID string) *lObjects.LoadBalancer {
	for {
		lb, err := c.api.GetLB(c.vLBSC, c.extraInfo.ProjectID, lbID)
		if err != nil {
			logrus.Errorln("error when get lb status: ", err)
		} else if lb.Status == "ACTIVE" {
			return lb
		}
		logrus.Infoln("------- wait for lb active:", lb.Status, "-------")
		time.Sleep(5 * time.Second)
	}
}

func (c *VLBProvider) FindPoolByName(lbID, name string) (*lObjects.Pool, error) {
	pools, err := c.api.ListPoolOfLB(c.vLBSC, c.extraInfo.ProjectID, lbID)
	if err != nil {
		return nil, err
	}
	for _, pool := range pools {
		if pool.Name == name {
			return pool, nil
		}
	}
	return nil, errors.ErrNotFound
}

func (c *VLBProvider) FindListenerByName(lbID, name string) (*lObjects.Listener, error) {
	listeners, err := c.api.ListListenerOfLB(c.vLBSC, c.extraInfo.ProjectID, lbID)
	if err != nil {
		return nil, err
	}
	for _, listener := range listeners {
		if listener.Name == name {
			return listener, nil
		}
	}
	return nil, errors.ErrNotFound
}

func EncodeToValidName(str string) string {
	// Only letters (a-z, A-Z, 0-9, '_', '.', '-') are allowed.
	// the other char will repaced by ":{number}:"
	for _, char := range str {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char == '_' || char == '.' || char == '-' {
			continue
		}
		str = strings.ReplaceAll(str, string(char), fmt.Sprintf("-%d-", char))
	}
	return str
}
func DecodeFromValidName(str string) string {
	r, _ := regexp.Compile("-[0-9]+-")
	matchs := r.FindStringSubmatch(str)
	for _, match := range matchs {
		number, _ := strconv.Atoi(match[1 : len(match)-1])
		str = strings.ReplaceAll(str, match, fmt.Sprintf("%c", number))
	}
	return str
}

// hash a string to a string have 10 char
func HashString(str string) string {
	// Create a new SHA-256 hash
	hasher := sha256.New()
	// Write the input string to the hash
	hasher.Write([]byte(str))
	// Sum returns the hash as a byte slice
	hashBytes := hasher.Sum(nil)
	// Truncate the hash to 10 characters
	truncatedHash := hashBytes[:10]
	// Convert the truncated hash to a hex-encoded string
	hashString := hex.EncodeToString(truncatedHash)
	return hashString
}

func (c *VLBProvider) MapIDExpander(old, cur *IngressInspect) {
	// map policy
	mapPolicyIndex := make(map[string]int)
	for curIndex, curPol := range cur.PolicyExpander {
		mapPolicyIndex[curPol.Name] = curIndex
	}
	for _, oldPol := range old.PolicyExpander {
		if curIndex, ok := mapPolicyIndex[oldPol.Name]; ok {
			oldPol.UUID = cur.PolicyExpander[curIndex].UUID
			oldPol.listenerID = cur.PolicyExpander[curIndex].listenerID
		} else {
			logrus.Error("policy not found when map ingress: %v", oldPol)
		}
	}

	// map pool
	mapPoolIndex := make(map[string]int)
	for curIndex, curPol := range cur.PoolExpander {
		mapPoolIndex[curPol.Name] = curIndex
	}
	for _, oldPol := range old.PoolExpander {
		if curIndex, ok := mapPoolIndex[oldPol.Name]; ok {
			oldPol.UUID = cur.PoolExpander[curIndex].UUID
		} else {
			logrus.Error("pool not found when map ingress: %v", oldPol)
		}
	}

	// map listener
	mapListenerIndex := make(map[string]int)
	for curIndex, curPol := range cur.ListenerExpander {
		mapListenerIndex[curPol.CreateOpts.ListenerName] = curIndex
	}
	for _, oldPol := range old.ListenerExpander {
		if curIndex, ok := mapListenerIndex[oldPol.CreateOpts.ListenerName]; ok {
			oldPol.UUID = cur.ListenerExpander[curIndex].UUID
		} else {
			logrus.Error("listener not found when map ingress: %v", oldPol)
		}
	}
}
