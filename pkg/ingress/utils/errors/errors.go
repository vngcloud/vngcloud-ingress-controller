package errors

import (
	"errors"
	"fmt"

	vconError "github.com/vngcloud/vngcloud-go-sdk/error"
)

// ********************************************** ErrNodeAddressNotFound **********************************************

func NewErrNodeAddressNotFound(pNodeName, pInfo string) vconError.IErrorBuilder {
	err := new(ErrNodeAddressNotFound)
	err.NodeName = pNodeName
	if pInfo != "" {
		err.Info = pInfo
	}
	return err
}

func IsErrNodeAddressNotFound(pErr error) bool {
	_, ok := pErr.(*ErrNodeAddressNotFound)
	return ok
}

func (s *ErrNodeAddressNotFound) Error() string {
	s.DefaultError = fmt.Sprintf("can not find address of node [NodeName: %s]", s.NodeName)
	return s.ChoseErrString()
}

type ErrNodeAddressNotFound struct {
	NodeName string
	vconError.BaseError
}

// ErrNotFound is used to inform that the object is missing
var ErrNotFound = errors.New("failed to find object")

var ErrLoadBalancerIDNotFoundAnnotation = errors.New("failed to find LoadBalancerID from Annotation")
var ErrLoadBalancerNameNotFoundAnnotation = errors.New("failed to find LoadBalancerName from Annotation")

var (
	ErrNoNodeAvailable                    = errors.New("no node available in the cluster")
	ErrNoPortIsConfigured                 = errors.New("no port is configured for the Kubernetes service")
	ErrSpecAndExistingLbNotMatch          = errors.New("the spec and the existing load balancer do not match")
	ErrNodesAreNotSameSubnet              = errors.New("nodes are not in the same subnet")
	ErrServiceConfigIsNil                 = errors.New("service config is nil")
	ErrEmptyLoadBalancerAnnotation        = errors.New("load balancer annotation is empty")
	ErrTimeoutWaitingLoadBalancerReady    = errors.New("timeout waiting for load balancer to be ready")
	ErrLoadBalancerIdNotInAnnotation      = errors.New("loadbalancer ID has been not set in service annotation")
	ErrServerObjectIsNil                  = errors.New("server object is nil")
	ErrVngCloudNotReturnNilPoolObject     = errors.New("VNG CLOUD should not return nil pool object")
	ErrVngCloudNotReturnNilListenerObject = errors.New("VNG CLOUD should not return nil listener object")
)
