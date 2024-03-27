/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	lObjects "github.com/vngcloud/vngcloud-go-sdk/vngcloud/objects"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/listener"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/policy"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/pool"
	"github.com/vngcloud/vngcloud-ingress-controller/pkg/consts"
	"github.com/vngcloud/vngcloud-ingress-controller/pkg/utils/errors"
	apiv1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

// IsValid returns true if the given Ingress either doesn't specify
// the ingress.class annotation, or it's set to the configured in the
// ingress controller.
func IsValid(ing *nwv1.Ingress) bool {
	ingress, ok := ing.GetAnnotations()[consts.IngressKey]
	if !ok {
		// check in spec
		if ing.Spec.IngressClassName != nil {
			return *ing.Spec.IngressClassName == consts.IngressClass
		}
		return false
	}

	return ingress == consts.IngressClass
}

// create k8s client from config file
func createApiserverClient(apiserverHost string, kubeConfig string) (*kubernetes.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(apiserverHost, kubeConfig)
	if err != nil {
		return nil, err
	}

	cfg.QPS = consts.DefaultQPS
	cfg.Burst = consts.DefaultBurst
	cfg.ContentType = "application/vnd.kubernetes.protobuf"

	log.Debug("creating kubernetes API client")

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	v, err := client.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}
	log.WithFields(log.Fields{
		"version": fmt.Sprintf("v%v.%v", v.Major, v.Minor),
	}).Debug("kubernetes API client created")

	return client, nil
}

type NodeConditionPredicate func(node *apiv1.Node) bool

// listWithPredicate gets nodes that matches predicate function.
func listWithPredicate(nodeLister corelisters.NodeLister, predicate NodeConditionPredicate) ([]*apiv1.Node, error) {
	nodes, err := nodeLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var filtered []*apiv1.Node
	for i := range nodes {
		if predicate(nodes[i]) {
			filtered = append(filtered, nodes[i])
		}
	}

	return filtered, nil
}

func getNodeConditionPredicate() NodeConditionPredicate {
	return func(node *apiv1.Node) bool {
		// We add the master to the node list, but its unschedulable.  So we use this to filter
		// the master.
		if node.Spec.Unschedulable {
			return false
		}

		// Recognize nodes labeled as not suitable for LB, and filter them also, as we were doing previously.
		if _, hasExcludeLBRoleLabel := node.Labels[consts.LabelNodeExcludeLB]; hasExcludeLBRoleLabel {
			return false
		}

		// Deprecated in favor of LabelNodeExcludeLB, kept for consistency and will be removed later
		if _, hasNodeRoleMasterLabel := node.Labels[consts.DeprecatedLabelNodeRoleMaster]; hasNodeRoleMasterLabel {
			return false
		}

		// If we have no info, don't accept
		if len(node.Status.Conditions) == 0 {
			return false
		}
		for _, cond := range node.Status.Conditions {
			// We consider the node for load balancing only when its NodeReady condition status
			// is ConditionTrue
			if cond.Type == apiv1.NodeReady && cond.Status != apiv1.ConditionTrue {
				log.WithFields(log.Fields{"name": node.Name, "status": cond.Status}).Info("ignoring node")
				return false
			}
		}
		return true
	}
}

///////////////////////////////////////////////////////////////////////////////////////
// CERT
///////////////////////////////////////////////////////////////////////////////////////

func getNodeAddressForLB(node *apiv1.Node) (string, error) {
	addrs := node.Status.Addresses
	if len(addrs) == 0 {
		return "", errors.NewErrNodeAddressNotFound(node.Name, "")
	}

	for _, addr := range addrs {
		if addr.Type == apiv1.NodeInternalIP {
			return addr.Address, nil
		}
	}

	return addrs[0].Address, nil
}

func comparePoolOptions(ipool *lObjects.Pool, poolOptions *pool.CreateOpts) *pool.UpdateOpts {
	isNeedUpdate := false
	updateOptions := &pool.UpdateOpts{
		Algorithm:     poolOptions.Algorithm,
		Stickiness:    poolOptions.Stickiness,
		TLSEncryption: poolOptions.TLSEncryption,
		HealthMonitor: poolOptions.HealthMonitor,
	}
	if ipool.LoadBalanceMethod != string(poolOptions.Algorithm) ||
		ipool.Stickiness != *poolOptions.Stickiness ||
		ipool.TLSEncryption != *poolOptions.TLSEncryption {
		isNeedUpdate = true
	}
	if ipool.HealthMonitor.HealthyThreshold != poolOptions.HealthMonitor.HealthyThreshold ||
		ipool.HealthMonitor.UnhealthyThreshold != poolOptions.HealthMonitor.UnhealthyThreshold ||
		ipool.HealthMonitor.Interval != poolOptions.HealthMonitor.Interval ||
		ipool.HealthMonitor.Timeout != poolOptions.HealthMonitor.Timeout {
		isNeedUpdate = true
	}
	if ipool.HealthMonitor.HealthCheckProtocol == "HTTP" && poolOptions.HealthMonitor.HealthCheckProtocol == pool.CreateOptsHealthCheckProtocolOptHTTP {
		// domain may return nil
		if ipool.HealthMonitor.HealthCheckPath == nil || *ipool.HealthMonitor.HealthCheckPath != *poolOptions.HealthMonitor.HealthCheckPath ||
			ipool.HealthMonitor.DomainName == nil || *ipool.HealthMonitor.DomainName != *poolOptions.HealthMonitor.DomainName ||
			ipool.HealthMonitor.HttpVersion == nil || *ipool.HealthMonitor.HttpVersion != string(*poolOptions.HealthMonitor.HttpVersion) ||
			ipool.HealthMonitor.HealthCheckMethod == nil || *ipool.HealthMonitor.HealthCheckMethod != string(*poolOptions.HealthMonitor.HealthCheckMethod) ||
			ipool.HealthMonitor.SuccessCode == nil || *ipool.HealthMonitor.SuccessCode != *poolOptions.HealthMonitor.SuccessCode {
			isNeedUpdate = true
		}
	} else if ipool.HealthMonitor.HealthCheckProtocol == "HTTP" && poolOptions.HealthMonitor.HealthCheckProtocol == pool.CreateOptsHealthCheckProtocolOptTCP {
		updateOptions.HealthMonitor.HealthCheckProtocol = pool.CreateOptsHealthCheckProtocolOptHTTP
		updateOptions.HealthMonitor.HealthCheckPath = ipool.HealthMonitor.HealthCheckPath
		updateOptions.HealthMonitor.DomainName = ipool.HealthMonitor.DomainName
		*updateOptions.HealthMonitor.HttpVersion = pool.CreateOptsHealthCheckHttpVersionOpt(*ipool.HealthMonitor.HttpVersion)
		*updateOptions.HealthMonitor.HealthCheckMethod = pool.CreateOptsHealthCheckMethodOpt(*ipool.HealthMonitor.HealthCheckMethod)
	} else if ipool.HealthMonitor.HealthCheckProtocol == "TCP" && poolOptions.HealthMonitor.HealthCheckProtocol == pool.CreateOptsHealthCheckProtocolOptHTTP {
		updateOptions.HealthMonitor.HealthCheckProtocol = pool.CreateOptsHealthCheckProtocolOptTCP
		updateOptions.HealthMonitor.HealthCheckPath = nil
		updateOptions.HealthMonitor.DomainName = nil
		updateOptions.HealthMonitor.HttpVersion = nil
		updateOptions.HealthMonitor.HealthCheckMethod = nil
	}

	if !isNeedUpdate {
		return nil
	}
	return updateOptions
}

func compareListenerOptions(ilis *lObjects.Listener, lisOptions *listener.CreateOpts) *listener.UpdateOpts {
	isNeedUpdate := false
	updateOptions := &listener.UpdateOpts{
		AllowedCidrs:                lisOptions.AllowedCidrs,
		TimeoutClient:               lisOptions.TimeoutClient,
		TimeoutMember:               lisOptions.TimeoutMember,
		TimeoutConnection:           lisOptions.TimeoutConnection,
		DefaultPoolId:               lisOptions.DefaultPoolId,
		DefaultCertificateAuthority: lisOptions.DefaultCertificateAuthority,
		// Headers:                     lisOptions.Headers,
		// ClientCertificate:           lisOptions.ClientCertificateAuthentication,
		// ......................................... update later
	}
	if ilis.AllowedCidrs != lisOptions.AllowedCidrs ||
		ilis.TimeoutClient != lisOptions.TimeoutClient ||
		ilis.TimeoutMember != lisOptions.TimeoutMember ||
		ilis.TimeoutConnection != lisOptions.TimeoutConnection {
		isNeedUpdate = true
	}

	if ilis.DefaultPoolId != lisOptions.DefaultPoolId {
		klog.Infof("listener need update default pool id: %s", lisOptions.DefaultPoolId)
		isNeedUpdate = true
	}
	if lisOptions.DefaultCertificateAuthority != nil && (ilis.DefaultCertificateAuthority == nil || *(ilis.DefaultCertificateAuthority) != *(lisOptions.DefaultCertificateAuthority)) {
		klog.Infof("listener need update default certificate authority: %s", *lisOptions.DefaultCertificateAuthority)
		isNeedUpdate = true
	}
	// update cert SNI here .......................................................
	if !isNeedUpdate {
		return nil
	}
	return updateOptions
}

func comparePoolDefaultMember(memsGet []*lObjects.Member, oldMembers, newMembers []*pool.Member) ([]*pool.Member, error) {
	memsGetConvert := ConvertObjectToPoolMemberArray(memsGet)
	getRedundant := func(old, new []*pool.Member) []*pool.Member {
		redundant := make([]*pool.Member, 0)
		for _, o := range old {
			isHave := false
			for _, n := range new {
				if o.IpAddress == n.IpAddress &&
					o.MonitorPort == n.MonitorPort &&
					o.Weight == n.Weight &&
					o.Backup == n.Backup &&
					// o.Name == n.Name &&
					o.Port == n.Port {
					isHave = true
					break
				}
			}
			if !isHave {
				redundant = append(redundant, o)
			}
		}
		return redundant
	}
	needDelete := getRedundant(oldMembers, newMembers)
	needCreate := newMembers // need ensure

	for _, m := range memsGetConvert {
		klog.V(5).Infof("current: [ %s, %s, %d ]", m.Name, m.IpAddress, m.Port)
	}
	for _, m := range needDelete {
		klog.V(5).Infof("delete : [ %s, %s, %d ]", m.Name, m.IpAddress, m.Port)
	}
	for _, m := range needCreate {
		klog.V(5).Infof("create : [ %s, %s, %d ]", m.Name, m.IpAddress, m.Port)
	}

	updateMember := make([]*pool.Member, 0)
	for _, m := range memsGetConvert {
		// remove all member in needCreate and add later (maybe member is scale down, then redundant)
		isAddLater := false
		for _, nc := range needCreate {
			if strings.HasPrefix(m.Name, nc.Name) {
				isAddLater = true
				break
			}
		}
		if isAddLater {
			continue
		}

		if !CheckIfPoolMemberExist(needDelete, m) {
			updateMember = append(updateMember, m)
		}
	}
	for _, m := range needCreate {
		if !CheckIfPoolMemberExist(updateMember, m) {
			updateMember = append(updateMember, m)
		}
	}

	if ComparePoolMembers(updateMember, memsGetConvert) {
		klog.Infof("no need update default pool member")
		return nil, nil
	}

	return updateMember, nil
}

func comparePolicy(currentPolicy *lObjects.Policy, policyOpt *policy.CreateOptsBuilder) *policy.UpdateOptsBuilder {
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
				klog.Infof("rule not exist: %v", rule)
				return false
			}
		}
		return true
	}
	if !comparePolicy(currentPolicy) {
		updateOpts := &policy.UpdateOptsBuilder{
			Action:         policyOpt.Action,
			RedirectPoolID: policyOpt.RedirectPoolID,
			Rules:          policyOpt.Rules,
		}
		return updateOpts
	}
	return nil
}
