package controller

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/coe/v2/cluster"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/certificates"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/listener"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/policy"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/pool"

	"github.com/vngcloud/vngcloud-go-sdk/client"
	lObjects "github.com/vngcloud/vngcloud-go-sdk/vngcloud/objects"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/loadbalancer"
	"github.com/vngcloud/vngcloud-ingress-controller/pkg/ingress/utils/errors"
	"k8s.io/klog/v2"
)

type API struct {
	VLBSC     *client.ServiceClient
	VServerSC *client.ServiceClient
	ProjectID string
}

// COMMON
func (c *API) GetClusterInfo(clusterID string) (*lObjects.Cluster, error) {
	// klog.V(5).Infoln("*****API__GetClusterInfo: ", "clusterID: ", clusterID)
	opts := &cluster.GetOpts{}
	opts.ProjectID = c.ProjectID
	opts.ClusterID = clusterID

	resp, err := cluster.Get(c.VServerSC, opts)
	// klog.V(5).Infoln("*****API__GetClusterInfo: ", "resp: ", resp, "err: ", err)
	return resp, err
}

// LB
func (c *API) ListLBBySubnetID(subnetID string) ([]*lObjects.LoadBalancer, error) {
	// klog.V(5).Infoln("*****API__ListLBBySubnetID: ", "subnetID: ", subnetID)
	opt := &loadbalancer.ListBySubnetIDOpts{}
	opt.ProjectID = c.ProjectID
	opt.SubnetID = subnetID

	resp, err := loadbalancer.ListBySubnetID(c.VLBSC, opt)
	// klog.V(5).Infoln("*****API__ListLBBySubnetID: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) GetLB(lbID string) (*lObjects.LoadBalancer, error) {
	// klog.V(5).Infoln("*****API__GetLB: ", "lbID: ", lbID)
	opt := &loadbalancer.GetOpts{}
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID

	resp, err := loadbalancer.Get(c.VLBSC, opt)
	// klog.V(5).Infoln("*****API__GetLB: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) CreateLB(lbOptions *loadbalancer.CreateOpts) (*lObjects.LoadBalancer, error) {
	klog.V(5).Infoln("*****API__CreateLB: ", "name: ", lbOptions.Name, "packageID: ", lbOptions.PackageID, "scheme: ", lbOptions.Scheme, "subnetID: ", lbOptions.SubnetID, "type: ", lbOptions.Type)
	lbOptions.ProjectID = c.ProjectID

	resp, err := loadbalancer.Create(c.VLBSC, lbOptions)
	klog.V(5).Infoln("*****API__CreateLB: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) DeleteLB(lbID string) error {
	klog.V(5).Infoln("*****API__DeleteLB: ", "lbID: ", lbID)
	opt := &loadbalancer.DeleteOpts{}
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID

	var err error
	for {
		err = loadbalancer.Delete(c.VLBSC, opt)
		if err != nil && IsLoadBalancerNotReady(err) {
			klog.V(5).Infof("LoadBalancerNotReady, retry after 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}

	klog.V(5).Infoln("*****API__DeleteLB: ", "err: ", err)
	return err
}

func (c *API) ResizeLB(lbID, packageID string) error {
	klog.V(5).Infoln("*****API__ResizeLB: ", "lbID: ", lbID, "packageID: ", packageID)
	opt := &loadbalancer.UpdateOpts{}
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	opt.PackageID = packageID

	var err error
	for {
		_, err = loadbalancer.Update(c.VLBSC, opt)
		if err != nil && IsLoadBalancerNotReady(err) {
			klog.V(5).Infof("LoadBalancerNotReady, retry after 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}
	klog.V(5).Infoln("*****API__ResizeLB: ", "err: ", err)
	return err
}

// POOL
func (c *API) CreatePool(lbID string, opt *pool.CreateOpts) (*lObjects.Pool, error) {
	klog.V(5).Infoln("*****API__CreatePool: ", "lbID: ", lbID)
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID

	var resp *lObjects.Pool
	var err error
	for {
		resp, err = pool.Create(c.VLBSC, opt)
		if err != nil && IsLoadBalancerNotReady(err) {
			klog.V(5).Infof("LoadBalancerNotReady, retry after 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}
	klog.V(5).Infoln("*****API__CreatePool: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) ListPoolOfLB(lbID string) ([]*lObjects.Pool, error) {
	// klog.V(5).Infoln("*****API__ListPool: ", "lbID: ", lbID)
	opt := &pool.ListPoolsBasedLoadBalancerOpts{}
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID

	resp, err := pool.ListPoolsBasedLoadBalancer(c.VLBSC, opt)
	// klog.V(5).Infoln("*****API__ListPool: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) UpdatePoolMember(lbID, poolID string, mems []*pool.Member) error {
	klog.V(5).Infoln("*****API__UpdatePoolMember: ", "poolID: ", poolID, "mems: ", mems)
	opt := &pool.UpdatePoolMembersOpts{
		Members: mems,
	}
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	opt.PoolID = poolID

	var err error
	for {
		err = pool.UpdatePoolMembers(c.VLBSC, opt)
		if err != nil && IsLoadBalancerNotReady(err) {
			klog.V(5).Infof("LoadBalancerNotReady, retry after 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}

	klog.V(5).Infoln("*****API__UpdatePoolMember: ", "err: ", err)
	return err
}

func (c *API) GetPool(lbID, poolID string) (*lObjects.Pool, error) {
	// klog.V(5).Infoln("*****API__GetPool: ", "poolID: ", poolID)
	opt := &pool.GetOpts{}
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	opt.PoolID = poolID

	resp, err := pool.GetTotal(c.VLBSC, opt)
	// klog.V(5).Infoln("*****API__GetPool: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) GetMemberPool(lbID, poolID string) ([]*lObjects.Member, error) {
	// klog.V(5).Infoln("*****API__GetMemberPool: ", "poolID: ", poolID)
	opt := &pool.GetMemberOpts{}
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	opt.PoolID = poolID

	resp, err := pool.GetMember(c.VLBSC, opt)
	// klog.V(5).Infoln("*****API__GetMemberPool: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) DeletePool(lbID, poolID string) error {
	klog.V(5).Infoln("*****API__DeletePool: ", "poolID: ", poolID)
	opt := &pool.DeleteOpts{}
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	opt.PoolID = poolID

	var err error
	for {
		err = pool.Delete(c.VLBSC, opt)
		if err != nil && IsLoadBalancerNotReady(err) {
			klog.V(5).Infof("LoadBalancerNotReady, retry after 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}
	klog.V(5).Infoln("*****API__DeletePool: ", "err: ", err)
	return err
}

func (c *API) UpdatePool(lbID, poolID string, opt *pool.UpdateOpts) error {
	klog.V(5).Infoln("*****API__UpdatePool: ", "lbID: ", lbID, "poolID: ", poolID)
	klog.V(5).Infoln("*****API__UpdatePool: ", "opt: ", opt)
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	opt.PoolID = poolID

	var err error
	for {
		err = pool.Update(c.VLBSC, opt)
		if err != nil && IsLoadBalancerNotReady(err) {
			klog.V(5).Infof("LoadBalancerNotReady, retry after 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}
	klog.V(5).Infoln("*****API__UpdatePool: ", "err: ", err)
	return err
}

// LISTENER
func (c *API) CreateListener(lbID string, opt *listener.CreateOpts) (*lObjects.Listener, error) {
	klog.V(5).Infoln("*****API__CreateListener: ", "lbID: ", lbID)
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID

	var resp *lObjects.Listener
	var err error
	for {
		resp, err = listener.Create(c.VLBSC, opt)
		if err != nil && IsLoadBalancerNotReady(err) {
			klog.V(5).Infof("LoadBalancerNotReady, retry after 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}
	klog.V(5).Infoln("*****API__CreateListener: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) ListListenerOfLB(lbID string) ([]*lObjects.Listener, error) {
	// klog.V(5).Infoln("*****API__ListListenerOfLB: ", "lbID: ", lbID)
	opt := &listener.GetBasedLoadBalancerOpts{}
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	resp, err := listener.GetBasedLoadBalancer(c.VLBSC, opt)
	// klog.V(5).Infoln("*****API__ListListenerOfLB: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) DeleteListener(lbID, listenerID string) error {
	klog.V(5).Infoln("*****API__DeleteListener: ", "lbID: ", lbID, "listenerID: ", listenerID)
	opt := &listener.DeleteOpts{}
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID

	var err error
	for {
		err = listener.Delete(c.VLBSC, opt)
		if err != nil && IsLoadBalancerNotReady(err) {
			klog.V(5).Infof("LoadBalancerNotReady, retry after 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}
	klog.V(5).Infoln("*****API__DeleteListener: ", "err: ", err)
	return err
}

func (c *API) UpdateListener(lbID, listenerID string, opt *listener.UpdateOpts) error {
	klog.V(5).Infoln("*****API__UpdateListener: ", "lbID: ", lbID, "listenerID: ", listenerID)
	klog.V(5).Infoln("*****API__UpdateListener: ", "opt: ", opt)
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID

	var err error
	for {
		err = listener.Update(c.VLBSC, opt)
		if err != nil && IsLoadBalancerNotReady(err) {
			klog.V(5).Infof("LoadBalancerNotReady, retry after 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}
	klog.V(5).Infoln("*****API__UpdateListener: ", "err: ", err)
	return err
}

// POLICY
func (c *API) CreatePolicy(lbID, listenerID string, opt *policy.CreateOptsBuilder) (*lObjects.Policy, error) {
	klog.V(5).Infoln("*****API__CreatePolicy: ", "lbID: ", lbID, "listenerID: ", listenerID, "opt: ", opt)
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID

	var resp *lObjects.Policy
	var err error
	for {
		resp, err = policy.Create(c.VLBSC, opt)
		if err != nil && IsLoadBalancerNotReady(err) {
			klog.V(5).Infof("LoadBalancerNotReady, retry after 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}
	klog.V(5).Infoln("*****API__CreatePolicy: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) ListPolicyOfListener(lbID, listenerID string) ([]*lObjects.Policy, error) {
	// klog.V(5).Infoln("*****API__ListPolicyOfListener: ", "lbID: ", lbID, "listenerID: ", listenerID)
	opt := &policy.ListOptsBuilder{}
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID
	resp, err := policy.List(c.VLBSC, opt)
	// klog.V(5).Infoln("*****API__ListPolicyOfListener: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) GetPolicy(lbID, listenerID, policyID string) (*lObjects.Policy, error) {
	// klog.V(5).Infoln("*****API__GetPolicy: ", "lbID: ", lbID, "listenerID: ", listenerID, "policyID: ", policyID)
	opt := &policy.GetOptsBuilder{}
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID
	opt.PolicyID = policyID
	resp, err := policy.Get(c.VLBSC, opt)
	// klog.V(5).Infoln("*****API__GetPolicy: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) UpdatePolicy(lbID, listenerID, policyID string, opt *policy.UpdateOptsBuilder) error {
	klog.V(5).Infoln("*****API__UpdatePolicy: ", "lbID: ", lbID, "listenerID: ", listenerID, "policyID: ", policyID)
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID
	opt.PolicyID = policyID

	var err error
	for {
		err = policy.Update(c.VLBSC, opt)
		if err != nil && IsLoadBalancerNotReady(err) {
			klog.V(5).Infof("LoadBalancerNotReady, retry after 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}
	klog.V(5).Infoln("*****API__UpdatePolicy: ", "err: ", err)
	return err
}

func (c *API) DeletePolicy(lbID, listenerID, policyID string) error {
	klog.V(5).Infoln("*****API__DeletePolicy: ", "lbID: ", lbID, "listenerID: ", listenerID, "policyID: ", policyID)
	opt := &policy.DeleteOptsBuilder{}
	opt.ProjectID = c.ProjectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID
	opt.PolicyID = policyID

	var err error
	for {
		err = policy.Delete(c.VLBSC, opt)
		if err != nil && IsLoadBalancerNotReady(err) {
			klog.V(5).Infof("LoadBalancerNotReady, retry after 5 seconds")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}
	klog.V(5).Infoln("*****API__DeletePolicy: ", "err: ", err)
	return err
}

// CERTIFICATE
func (c *API) ImportCertificate(opt *certificates.ImportOpts) (*lObjects.Certificate, error) {
	klog.V(5).Infoln("*****API__ImportCertificate: ", "opt: ", opt)
	opt.ProjectID = c.ProjectID
	resp, err := certificates.Import(c.VLBSC, opt)
	klog.V(5).Infoln("*****API__ImportCertificate: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) ListCertificate() ([]*lObjects.Certificate, error) {
	// klog.V(5).Infoln("*****API__ListCertificate: ")
	opt := &certificates.ListOpts{}
	opt.ProjectID = c.ProjectID
	resp, err := certificates.List(c.VLBSC, opt)
	// klog.V(5).Infoln("*****API__ListCertificate: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) GetCertificate(certificateID string) (*lObjects.Certificate, error) {
	klog.V(5).Infoln("*****API__GetCertificate: ", "certificateID: ", certificateID)
	opt := &certificates.GetOpts{}
	opt.ProjectID = c.ProjectID
	opt.CaID = certificateID
	resp, err := certificates.Get(c.VLBSC, opt)
	klog.V(5).Infoln("*****API__GetCertificate: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) DeleteCertificate(certificateID string) error {
	klog.V(5).Infoln("*****API__DeleteCertificate: ", "certificateID: ", certificateID)
	opt := &certificates.DeleteOpts{}
	opt.ProjectID = c.ProjectID
	opt.CaID = certificateID
	err := certificates.Delete(c.VLBSC, opt)
	klog.V(5).Infoln("*****API__DeleteCertificate: ", "err: ", err)
	return err
}

////////////////////////////////////////////////////////////////////////////////////////////////

func (c *API) FindPolicyByName(lbID, listenerID, name string) (*lObjects.Policy, error) {
	policyArr, err := c.ListPolicyOfListener(lbID, listenerID)
	if err != nil {
		klog.Errorln("error when list policy", err)
		return nil, err
	}
	for _, policy := range policyArr {
		if policy.Name == name {
			return policy, nil
		}
	}
	return nil, errors.ErrNotFound
}

func (c *API) FindPoolByName(lbID, name string) (*lObjects.Pool, error) {
	pools, err := c.ListPoolOfLB(lbID)
	if err != nil {
		return nil, err
	}
	for _, pool := range pools {
		if pool.Name == name {
			ipool, err := c.GetPool(lbID, pool.UUID)
			if err != nil {
				return nil, err
			}
			return ipool, nil
		}
	}
	return nil, errors.ErrNotFound
}

func (c *API) FindListenerByName(lbID, name string) (*lObjects.Listener, error) {
	listeners, err := c.ListListenerOfLB(lbID)
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

func (c *API) FindListenerByPort(lbID string, port int) (*lObjects.Listener, error) {
	listeners, err := c.ListListenerOfLB(lbID)
	if err != nil {
		return nil, err
	}
	for _, listener := range listeners {
		if listener.ProtocolPort == port {
			if (port == 443 && listener.Protocol != "HTTPS") || (port == 80 && listener.Protocol != "HTTP") {
				klog.Infof("listener %s has wrong protocol %s or wrong port %d", listener.UUID, listener.Protocol, listener.ProtocolPort)
				return nil, fmt.Errorf("listener %s has wrong protocol %s or wrong port %d", listener.UUID, listener.Protocol, listener.ProtocolPort)
			}
			return listener, nil
		}
	}
	return nil, errors.ErrNotFound
}

func (c *API) WaitForLBActive(lbID string) *lObjects.LoadBalancer {
	for {
		lb, err := c.GetLB(lbID)
		if err != nil {
			klog.Errorln("error when get lb status: ", err)
		} else if lb.Status == "ACTIVE" {
			return lb
		} else if lb.Status == "ERROR" {
			klog.Error("LoadBalancer is in ERROR state")
			return lb
		}
		klog.V(3).Infoln("------- wait for lb active:", lb.Status, "-------")
		time.Sleep(10 * time.Second)
	}
}

type ErrorRespone struct {
	Message    string `json:"message"`
	ErrorCode  string `json:"errorCode"`
	StatusCode int    `json:"statusCode"`
}

func ParseError(errStr string) *ErrorRespone {
	if errStr == "" {
		return nil
	}
	e := &ErrorRespone{}
	err := json.Unmarshal([]byte(errStr), e)
	if err != nil {
		klog.Errorf("error when parse error: %s", err)
		return nil
	}
	return e
}

func IsLoadBalancerNotReady(err error) bool {
	e := ParseError(err.Error())
	if e != nil && (e.ErrorCode == "LoadBalancerNotReady" || e.ErrorCode == "ListenerNotReady") {
		return true
	}
	return false
}
