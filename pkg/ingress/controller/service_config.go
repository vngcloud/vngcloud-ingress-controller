package controller

import (
	corev1 "k8s.io/api/core/v1"

	lClusterObjV2 "github.com/vngcloud/vngcloud-go-sdk/vngcloud/objects"
	lLoadBalancerV2 "github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/loadbalancer"
)

type serviceConfig struct {
	internal          bool
	lbID              string
	preferredIPFamily corev1.IPFamily // preferred (the first) IP family indicated in service's `spec.ipFamilies`
	flavorID          string
	scheme            lLoadBalancerV2.CreateOptsSchemeOpt
	lbType            lLoadBalancerV2.CreateOptsTypeOpt
	projectID         string
	subnetID          string
	cluster           *lClusterObjV2.Cluster
	isOwner           bool
}

func (s *serviceConfig) getClusterID() string {
	return s.cluster.ID
}

func (s *serviceConfig) getClusterSubnetID() string {
	return s.cluster.SubnetID
}
