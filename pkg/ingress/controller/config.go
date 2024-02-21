package controller

import (
	"k8s.io/cloud-provider-openstack/pkg/ingress/client"
	"time"

	"k8s.io/cloud-provider-openstack/pkg/ingress/utils/metadata"
)

type (
	Config struct {
		Global   client.AuthOpts
		VLB      VLBOpts
		Metadata metadata.Opts
	}
)

type VLBOpts struct {
	Enabled        bool   `gcfg:"enabled"`         // if false, disables the controller
	InternalLB     bool   `gcfg:"internal-lb"`     // default false
	FlavorID       string `gcfg:"flavor-id"`       // flavor id of load balancer
	MaxSharedLB    int    `gcfg:"max-shared-lb"`   //  Number of Services in maximum can share a single load balancer. Default 2
	LBMethod       string `gcfg:"lb-method"`       // default to ROUND_ROBIN.
	EnableVMonitor bool   `gcfg:"enable-vmonitor"` // default to false
}

const (
	waitLoadbalancerInitDelay   = 5 * time.Second
	waitLoadbalancerFactor      = 1.2
	waitLoadbalancerActiveSteps = 30
	waitLoadbalancerDeleteSteps = 12
)

const (
	DEFAULT_PORTAL_NAME_LENGTH = 50
	ACTIVE_LOADBALANCER_STATUS = "ACTIVE"
)

const (
	healthMonitorHealthyThreshold   = 3
	healthMonitorUnhealthyThreshold = 3
	healthMonitorInterval           = 30
	healthMonitorTimeout            = 5
)

const (
	listenerDefaultCIDR       = "0.0.0.0/0"
	listenerTimeoutClient     = 50
	listenerTimeoutConnection = 5
	listenerTimeoutMember     = 50
)

const (
	// High enough QPS to fit all expected use cases. QPS=0 is not set here, because
	// client code is overriding it.
	defaultQPS = 1e6
	// High enough Burst to fit all expected use cases. Burst=0 is not set here, because
	// client code is overriding it.
	defaultBurst = 1e6

	maxRetries = 5

	// CreateEvent event associated with new objects in an informer
	CreateEvent EventType = "CREATE"
	// UpdateEvent event associated with an object update in an informer
	UpdateEvent EventType = "UPDATE"
	// DeleteEvent event associated when an object is removed from an informer
	DeleteEvent EventType = "DELETE"

	// IngressKey picks a specific "class" for the Ingress.
	// The controller only processes Ingresses with this annotation either
	// unset, or set to either the configured value or the empty string.
	IngressKey = "kubernetes.io/ingress.class"

	// IngressClass specifies which Ingress class we accept
	IngressClass = "vngcloud"

	// LabelNodeExcludeLB specifies that a node should not be used to create a Loadbalancer on
	// https://github.com/kubernetes/cloud-provider/blob/25867882d509131a6fdeaf812ceebfd0f19015dd/controllers/service/controller.go#L673
	LabelNodeExcludeLB = "node.kubernetes.io/exclude-from-external-load-balancers"

	// DeprecatedLabelNodeRoleMaster specifies that a node is a master
	// It's copied over to kubeadm until it's merged in core: https://github.com/kubernetes/kubernetes/pull/39112
	// Deprecated in favor of LabelNodeExcludeLB
	DeprecatedLabelNodeRoleMaster = "node-role.kubernetes.io/master"

	// IngressAnnotationInternal is the annotation used on the Ingress
	// to indicate that we want an internal loadbalancer service so that vngcloud-ingress-controller won't associate
	// floating ip to the load balancer VIP.
	// Default to true.
	IngressAnnotationInternal = "vngcloud.ingress.kubernetes.io/internal"

	// IngressAnnotationSourceRangesKey is the key of the annotation on an ingress to set allowed IP ranges on their LoadBalancers.
	// It should be a comma-separated list of CIDRs.
	IngressAnnotationSourceRangesKey = "vngcloud.ingress.kubernetes.io/whitelist-source-range"

	// IngressControllerTag is added to the related resources.
	IngressControllerTag = "vngcloud.ingress.kubernetes.io"

	// IngressAnnotationTimeoutClientData is the timeout for frontend client inactivity.
	// If not set, this value defaults to the vngcloud configuration key `timeout_client_data`.
	// Refer to https://docs.openstack.org/vngcloud/latest/configuration/configref.html#haproxy_amphora.timeout_client_data
	IngressAnnotationTimeoutClientData = "vngcloud.ingress.kubernetes.io/timeout-client-data"

	// IngressAnnotationTimeoutMemberData is the timeout for backend member inactivity.
	// If not set, this value defaults to the vngcloud configuration key `timeout_member_data`.
	// Refer to https://docs.openstack.org/vngcloud/latest/configuration/configref.html#haproxy_amphora.timeout_member_data
	IngressAnnotationTimeoutMemberData = "vngcloud.ingress.kubernetes.io/timeout-member-data"

	// IngressAnnotationTimeoutMemberConnect is the timeout for backend member connection.
	// If not set, this value defaults to the vngcloud configuration key `timeout_member_connect`.
	// Refer to https://docs.openstack.org/vngcloud/latest/configuration/configref.html#haproxy_amphora.timeout_member_connect
	IngressAnnotationTimeoutMemberConnect = "vngcloud.ingress.kubernetes.io/timeout-member-connect"

	// IngressAnnotationTimeoutTCPInspect is the time to wait for TCP packets for content inspection.
	// If not set, this value defaults to the vngcloud configuration key `timeout_tcp_inspect`.
	// Refer to https://docs.openstack.org/vngcloud/latest/configuration/configref.html#haproxy_amphora.timeout_tcp_inspect
	IngressAnnotationTimeoutTCPInspect = "vngcloud.ingress.kubernetes.io/timeout-tcp-inspect"

	// IngressSecretCertName is certificate key name defined in the secret data.
	IngressSecretCertName = "tls.crt"
	// IngressSecretKeyName is private key name defined in the secret data.
	IngressSecretKeyName = "tls.key"
)
