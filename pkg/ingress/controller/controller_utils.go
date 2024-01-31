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
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/objects"
	lObjects "github.com/vngcloud/vngcloud-go-sdk/vngcloud/objects"
	apiv1 "k8s.io/api/core/v1"
	lCoreV1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/cloud-provider-openstack/pkg/ingress/utils/errors"
	"k8s.io/klog/v2"
	"strconv"
)

// IsValid returns true if the given Ingress either doesn't specify
// the ingress.class annotation, or it's set to the configured in the
// ingress controller.
func IsValid(ing *nwv1.Ingress) bool {
	ingress, ok := ing.GetAnnotations()[IngressKey]
	if !ok {
		log.WithFields(log.Fields{
			"ingress_name": ing.Name, "ingress_ns": ing.Namespace,
		}).Info("annotation not present in ingress")
		return false
	}

	return ingress == IngressClass
}

// create k8s client from config file
func createApiserverClient(apiserverHost string, kubeConfig string) (*kubernetes.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(apiserverHost, kubeConfig)
	if err != nil {
		return nil, err
	}

	cfg.QPS = defaultQPS
	cfg.Burst = defaultBurst
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
		if _, hasExcludeLBRoleLabel := node.Labels[LabelNodeExcludeLB]; hasExcludeLBRoleLabel {
			return false
		}

		// Deprecated in favor of LabelNodeExcludeLB, kept for consistency and will be removed later
		if _, hasNodeRoleMasterLabel := node.Labels[DeprecatedLabelNodeRoleMaster]; hasNodeRoleMasterLabel {
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

// getStringFromIngressAnnotation searches a given Ingress for a specific annotationKey and either returns the
// annotation's value or a specified defaultSetting
func getStringFromIngressAnnotation(ingress *nwv1.Ingress, annotationKey string, defaultValue string) string {
	if annotationValue, ok := ingress.Annotations[annotationKey]; ok {
		return annotationValue
	}

	return defaultValue
}

// maybeGetIntFromIngressAnnotation searches a given Ingress for a specific annotationKey and either returns the
// annotation's value
func maybeGetIntFromIngressAnnotation(ingress *nwv1.Ingress, annotationKey string) *int {
	klog.V(4).Infof("maybeGetIntFromIngressAnnotation(%s/%s, %v33)", ingress.Namespace, ingress.Name, annotationKey)
	if annotationValue, ok := ingress.Annotations[annotationKey]; ok {
		klog.V(4).Infof("Found a Service Annotation for key: %v", annotationKey)
		returnValue, err := strconv.Atoi(annotationValue)
		if err != nil {
			klog.V(4).Infof("Invalid integer found on Service Annotation: %v = %v", annotationKey, annotationValue)
			return nil
		}
		return &returnValue
	}
	klog.V(4).Infof("Could not find a Service Annotation; falling back to default setting for annotation %v", annotationKey)
	return nil
}

// privateKeyFromPEM converts a PEM block into a crypto.PrivateKey.
func privateKeyFromPEM(pemData []byte) (crypto.PrivateKey, error) {
	var result *pem.Block
	rest := pemData
	for {
		result, rest = pem.Decode(rest)
		if result == nil {
			return nil, fmt.Errorf("cannot decode supplied PEM data")
		}

		switch result.Type {
		case "RSA PRIVATE KEY":
			return x509.ParsePKCS1PrivateKey(result.Bytes)
		case "EC PRIVATE KEY":
			return x509.ParseECPrivateKey(result.Bytes)
		case "PRIVATE KEY":
			return x509.ParsePKCS8PrivateKey(result.Bytes)
		}
	}
}

// parsePEMBundle parses a certificate bundle from top to bottom and returns
// a slice of x509 certificates. This function will error if no certificates are found.
func parsePEMBundle(bundle []byte) ([]*x509.Certificate, error) {
	var certificates []*x509.Certificate
	var certDERBlock *pem.Block

	for {
		certDERBlock, bundle = pem.Decode(bundle)
		if certDERBlock == nil {
			break
		}

		if certDERBlock.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(certDERBlock.Bytes)
			if err != nil {
				return nil, err
			}
			certificates = append(certificates, cert)
		}
	}

	if len(certificates) == 0 {
		return nil, fmt.Errorf("no certificates were found while parsing the bundle")
	}

	return certificates, nil
}

func getNodeAddressForLB(node *lCoreV1.Node) (string, error) {
	addrs := node.Status.Addresses
	if len(addrs) == 0 {
		return "", errors.NewErrNodeAddressNotFound(node.Name, "")
	}

	for _, addr := range addrs {
		if addr.Type == lCoreV1.NodeInternalIP {
			return addr.Address, nil
		}
	}

	return addrs[0].Address, nil
}

func popListener(pExistingListeners []*lObjects.Listener, pNewListenerID string) []*lObjects.Listener {
	var newListeners []*lObjects.Listener

	for _, existingListener := range pExistingListeners {
		if existingListener.UUID != pNewListenerID {
			newListeners = append(newListeners, existingListener)
		}
	}
	return newListeners
}

type LBMapping struct {
	listeners []*objects.Listener
	pools     []*objects.Pool
}

func MapIngressToVLB(ing *nwv1.Ingress) (*LBMapping, error) {

	return nil, nil
}
