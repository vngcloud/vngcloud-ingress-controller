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
var ErrLoadBalancerIDNotFoundAnnotation = errors.New("failed to find LoadBalancerID from Annotation")

// ErrNotFound is used to inform that the object is missing
var ErrNotFound = errors.New("failed to find object")
