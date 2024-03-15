package controller

import (
	"github.com/vngcloud/vngcloud-ingress-controller/pkg/ingress/client"
	"github.com/vngcloud/vngcloud-ingress-controller/pkg/ingress/utils/metadata"
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
	DEFAULT_PORTAL_NAME_LENGTH = 50
	ACTIVE_LOADBALANCER_STATUS = "ACTIVE"
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

	// IngressSecretCertName is certificate key name defined in the secret data.
	IngressSecretCertName = "tls.crt"
	// IngressSecretKeyName is private key name defined in the secret data.
	IngressSecretKeyName = "tls.key"
)

// Annotations
const (
	DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX = "vks.vngcloud.vn"

	// load balancer
	ServiceAnnotationLoadBalancerID       = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/load-balancer-id"   // set via annotation
	ServiceAnnotationLoadBalancerName     = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/load-balancer-name" // only set via the annotation
	ServiceAnnotationPackageID            = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/package-id"         // both annotation and cloud-config
	ServiceAnnotationLoadBalancerInternal = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/internal-load-balancer"

	// ServiceAnnotationEnableSecgroupDefault = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/enable-secgroup-default" // set via annotation

	// listener
	ServiceAnnotationIdleTimeoutClient     = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/idle-timeout-client"     // both annotation and cloud-config
	ServiceAnnotationIdleTimeoutMember     = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/idle-timeout-member"     // both annotation and cloud-config
	ServiceAnnotationIdleTimeoutConnection = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/idle-timeout-connection" // both annotation and cloud-config
	ServiceAnnotationListenerAllowedCIDRs  = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/listener-allowed-cidrs"  // both annotation and cloud-config

	// pool
	ServiceAnnotationPoolAlgorithm   = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/pool-algorithm" // both annotation and cloud-config "ROUND_ROBIN" "LEAST_CONNECTIONS" "SOURCE_IP"
	ServiceAnnotationMonitorProtocol = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-protocol"

	// health tcp protocol
	ServiceAnnotationHealthyThreshold          = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-healthy-threshold"
	ServiceAnnotationMonitorUnhealthyThreshold = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-unhealthy-threshold"
	ServiceAnnotationMonitorTimeout            = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-timeout"
	ServiceAnnotationMonitorInterval           = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-interval"

	// health http protocol
	ServiceAnnotationMonitorHttpMethod      = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-http-method"
	ServiceAnnotationMonitorHttpPath        = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-http-path"
	ServiceAnnotationMonitorHttpSuccessCode = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-http-success-code"
	ServiceAnnotationMonitorHttpVersion     = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-http-version"
	ServiceAnnotationMonitorHttpDomainName  = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-http-domain-name"

	// new
	ServiceAnnotationEnableStickySession = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/enable-sticky-session"
	ServiceAnnotationEnableTLSEncryption = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/enable-tls-encryption"
)
