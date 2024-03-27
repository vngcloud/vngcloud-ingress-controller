package controller

import (
	"fmt"

	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/listener"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/loadbalancer"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/policy"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/pool"
	"k8s.io/klog/v2"
)

type PoolExpander struct {
	UUID string
	pool.CreateOpts
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
	UUID string
	listener.CreateOpts
}
type CertificateExpander struct {
	UUID       string
	Name       string
	Version    string
	SecretName string
}

type IngressInspect struct {
	defaultPool *PoolExpander
	name        string
	namespace   string
	lbID        string                   // store the lb id
	lbName      string                   // auto generate or pass by user through annotation
	lbPostfix   string                   // a hash id
	lbOptions   *loadbalancer.CreateOpts // create options for lb

	PolicyExpander      []*PolicyExpander
	PoolExpander        []*PoolExpander
	ListenerExpander    []*ListenerExpander
	CertificateExpander []*CertificateExpander
}

func (ing *IngressInspect) Print() {
	fmt.Println("DEFAULT POOL: name:", ing.defaultPool.PoolName, "id:", ing.defaultPool.UUID, "members:", ing.defaultPool.Members)
	for _, l := range ing.ListenerExpander {
		fmt.Println("LISTENER: id:", l.UUID)
	}
	for _, p := range ing.PolicyExpander {
		fmt.Println("---- POLICY: id:", p.UUID, "name:", p.Name, "redirectPoolName:", p.RedirectPoolName, "redirectPoolId:", p.RedirectPoolID, "action:", p.Action, "l7Rules:", p.L7Rules)
	}
	for _, p := range ing.PoolExpander {
		fmt.Println("++++ POOL: name:", p.PoolName, "uuid:", p.UUID, "members:", p.Members)
	}
}

func MapIDExpander(old, cur *IngressInspect) {
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
			klog.Errorf("policy not found when map ingress: %v", oldPol)
		}
	}

	// map pool
	mapPoolIndex := make(map[string]int)
	for curIndex, curPol := range cur.PoolExpander {
		mapPoolIndex[curPol.PoolName] = curIndex
	}
	for _, oldPol := range old.PoolExpander {
		if curIndex, ok := mapPoolIndex[oldPol.PoolName]; ok {
			oldPol.UUID = cur.PoolExpander[curIndex].UUID
		} else {
			klog.Errorf("pool not found when map ingress: %v", oldPol)
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
			klog.Errorf("listener not found when map ingress: %v", oldPol)
		}
	}
}
