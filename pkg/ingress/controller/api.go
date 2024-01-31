package controller

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/coe/v2/cluster"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/listener"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/policy"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/pool"

	"github.com/vngcloud/vngcloud-go-sdk/client"
	lObjects "github.com/vngcloud/vngcloud-go-sdk/vngcloud/objects"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/loadbalancer"
	"k8s.io/klog/v2"
)

type API struct{}

// COMMON
func (c *API) GetClusterInfo(vSC *client.ServiceClient, projectID, clusterID string) (*lObjects.Cluster, error) {
	// logrus.Infoln("*****API__GetClusterInfo: ", "clusterID: ", clusterID, "projectID: ", projectID)
	opts := &cluster.GetOpts{}
	opts.ProjectID = projectID
	opts.ClusterID = clusterID

	resp, err := cluster.Get(vSC, opts)
	// logrus.Infoln("*****API__GetClusterInfo: ", "resp: ", resp, "err: ", err)
	return resp, err
}

// LB
func (c *API) ListLBBySubnetID(vSC *client.ServiceClient, projectID, subnetID string) ([]*lObjects.LoadBalancer, error) {
	// logrus.Infoln("*****API__ListLBBySubnetID: ", "subnetID: ", subnetID, "projectID: ", projectID)
	opt := &loadbalancer.ListBySubnetIDOpts{}
	opt.ProjectID = projectID
	opt.SubnetID = subnetID

	resp, err := loadbalancer.ListBySubnetID(vSC, opt)
	// logrus.Infoln("*****API__ListLBBySubnetID: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) GetLB(vSC *client.ServiceClient, projectID, lbID string) (*lObjects.LoadBalancer, error) {
	// logrus.Infoln("*****API__GetLB: ", "lbID: ", lbID, "projectID: ", projectID)
	opt := &loadbalancer.GetOpts{}
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID

	resp, err := loadbalancer.Get(vSC, opt)
	// logrus.Infoln("*****API__GetLB: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) CreateLB(vSC *client.ServiceClient, name, packageId, subnetID, projectID string,
	scheme loadbalancer.CreateOptsSchemeOpt,
	typeLB loadbalancer.CreateOptsTypeOpt,
) (*lObjects.LoadBalancer, error) {
	logrus.Infoln("*****API__CreateLB: ", "name: ", name, "packageId: ", packageId, "subnetID: ", subnetID, "projectID: ", projectID)
	opt := &loadbalancer.CreateOpts{
		Name:      name,
		PackageID: packageId,
		Scheme:    scheme,
		SubnetID:  subnetID,
		Type:      typeLB,
	}
	opt.ProjectID = projectID

	resp, err := loadbalancer.Create(vSC, opt)
	logrus.Infoln("*****API__CreateLB: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) DeleteLB(vSC *client.ServiceClient, projectID, lbID string) error {
	logrus.Infoln("*****API__DeleteLB: ", "lbID: ", lbID, "projectID: ", projectID)
	opt := &loadbalancer.DeleteOpts{}
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID

	err := loadbalancer.Delete(vSC, opt)
	logrus.Infoln("*****API__DeleteLB: ", "err: ", err)
	return err
}

// POOL
func (c *API) CreatePool(vSC *client.ServiceClient, projectID, lbID string, opt pool.CreateOpts) (*lObjects.Pool, error) {
	logrus.Infoln("*****API__CreatePool: ", "lbID: ", lbID, "projectID: ", projectID)
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID

	resp, err := pool.Create(vSC, &opt)
	logrus.Infoln("*****API__CreatePool: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) ListPoolOfLB(vSC *client.ServiceClient, projectID, lbID string) ([]*lObjects.Pool, error) {
	// logrus.Infoln("*****API__ListPool: ", "lbID: ", lbID, "projectID: ", projectID)
	opt := &pool.ListPoolsBasedLoadBalancerOpts{}
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID

	resp, err := pool.ListPoolsBasedLoadBalancer(vSC, opt)
	// logrus.Infoln("*****API__ListPool: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) UpdatePoolMember(vSC *client.ServiceClient, projectID, lbID, poolID string, mems []pool.Member) error {
	logrus.Infoln("*****API__UpdatePoolMember: ", "poolID: ", poolID, "projectID: ", projectID)
	opt := &pool.UpdatePoolMembersOpts{
		Members: mems,
	}
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID
	opt.PoolID = poolID

	err := pool.UpdatePoolMembers(vSC, opt)
	logrus.Infoln("*****API__UpdatePoolMember: ", "err: ", err)
	return err
}

func (c *API) GetPool(vSC *client.ServiceClient, projectID, lbID, poolID string) (*lObjects.Pool, error) {
	// logrus.Infoln("*****API__GetPool: ", "poolID: ", poolID, "projectID: ", projectID)
	opt := &pool.GetOpts{}
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID
	opt.PoolID = poolID

	resp, err := pool.Get(vSC, opt)
	// logrus.Infoln("*****API__GetPool: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) GetMemberPool(vSC *client.ServiceClient, projectID, lbID, poolID string) ([]*lObjects.Member, error) {
	// logrus.Infoln("*****API__GetMemberPool: ", "poolID: ", poolID, "projectID: ", projectID)
	opt := &pool.GetMemberOpts{}
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID
	opt.PoolID = poolID

	resp, err := pool.GetMember(vSC, opt)
	// logrus.Infoln("*****API__GetMemberPool: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) DeletePool(vSC *client.ServiceClient, projectID, lbID, poolID string) error {
	logrus.Infoln("*****API__DeletePool: ", "poolID: ", poolID, "projectID: ", projectID)
	opt := &pool.DeleteOpts{}
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID
	opt.PoolID = poolID

	err := pool.Delete(vSC, opt)
	logrus.Infoln("*****API__DeletePool: ", "err: ", err)
	return err
}

// LISTENER
func (c *API) CreateListener(vSC *client.ServiceClient, projectID, lbID string, opt *listener.CreateOpts) (*lObjects.Listener, error) {
	logrus.Infoln("*****API__CreateListener: ", "lbID: ", lbID, "projectID: ", projectID)
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID
	resp, err := listener.Create(vSC, opt)
	logrus.Infoln("*****API__CreateListener: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) ListListenerOfLB(vSC *client.ServiceClient, projectID, lbID string) ([]*lObjects.Listener, error) {
	// logrus.Infoln("*****API__ListListenerOfLB: ", "lbID: ", lbID, "projectID: ", projectID)
	opt := &listener.GetBasedLoadBalancerOpts{}
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID
	resp, err := listener.GetBasedLoadBalancer(vSC, opt)
	// logrus.Infoln("*****API__ListListenerOfLB: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) DeleteListener(vSC *client.ServiceClient, projectID, lbID, listenerID string) error {
	logrus.Infoln("*****API__DeleteListener: ", "lbID: ", lbID, "projectID: ", projectID, "listenerID: ", listenerID)
	opt := &listener.DeleteOpts{}
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID
	err := listener.Delete(vSC, opt)
	logrus.Infoln("*****API__DeleteListener: ", "err: ", err)
	return err
}

func (c *API) UpdateListener(vSC *client.ServiceClient, projectID, lbID, listenerID string, opt *listener.UpdateOpts) error {
	logrus.Infoln("*****API__UpdateListener: ", "lbID: ", lbID, "projectID: ", projectID, "listenerID: ", listenerID)
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID
	err := listener.Update(vSC, opt)
	logrus.Infoln("*****API__UpdateListener: ", "err: ", err)
	return err
}

// POLICY
func (c *API) CreatePolicy(vSC *client.ServiceClient, projectID, lbID, listenerID string, opt *policy.CreateOptsBuilder) (*lObjects.Policy, error) {
	logrus.Infoln("*****API__CreatePolicy: ", "lbID: ", lbID, "projectID: ", projectID, "listenerID: ", listenerID, "opt: ", opt)
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID
	resp, err := policy.Create(vSC, opt)
	logrus.Infoln("*****API__CreatePolicy: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) ListPolicyOfListener(vSC *client.ServiceClient, projectID, lbID, listenerID string) ([]*lObjects.Policy, error) {
	// logrus.Infoln("*****API__ListPolicyOfListener: ", "lbID: ", lbID, "projectID: ", projectID, "listenerID: ", listenerID)
	opt := &policy.ListOptsBuilder{}
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID
	resp, err := policy.List(vSC, opt)
	// logrus.Infoln("*****API__ListPolicyOfListener: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) GetPolicy(vSC *client.ServiceClient, projectID, lbID, listenerID, policyID string) (*lObjects.Policy, error) {
	// logrus.Infoln("*****API__GetPolicy: ", "lbID: ", lbID, "projectID: ", projectID, "listenerID: ", listenerID, "policyID: ", policyID)
	opt := &policy.GetOptsBuilder{}
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID
	opt.PolicyID = policyID
	resp, err := policy.Get(vSC, opt)
	// logrus.Infoln("*****API__GetPolicy: ", "resp: ", resp, "err: ", err)
	return resp, err
}

func (c *API) UpdatePolicy(vSC *client.ServiceClient, projectID, lbID, listenerID, policyID string, opt *policy.UpdateOptsBuilder) error {
	logrus.Infoln("*****API__UpdatePolicy: ", "lbID: ", lbID, "projectID: ", projectID, "listenerID: ", listenerID, "policyID: ", policyID)
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID
	opt.PolicyID = policyID
	err := policy.Update(vSC, opt)
	logrus.Infoln("*****API__UpdatePolicy: ", "err: ", err)
	return err
}

func (c *API) DeletePolicy(vSC *client.ServiceClient, projectID, lbID, listenerID, policyID string) error {
	logrus.Infoln("*****API__DeletePolicy: ", "lbID: ", lbID, "projectID: ", projectID, "listenerID: ", listenerID, "policyID: ", policyID)
	opt := &policy.DeleteOptsBuilder{}
	opt.ProjectID = projectID
	opt.LoadBalancerID = lbID
	opt.ListenerID = listenerID
	opt.PolicyID = policyID
	err := policy.Delete(vSC, opt)
	logrus.Infoln("*****API__DeletePolicy: ", "err: ", err)
	return err
}

////////////////////////////////////////////////////////////////////////////////////////////////

func Sleep(t int) {
	for i := 0; i < t; i++ {
		time.Sleep(time.Second)
		klog.Infof("Wait %d seconds", t-i)
	}
}
