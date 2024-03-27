package consts

const (
	DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX = "vks.vngcloud.vn"
)

// Annotations
const (
	ServiceAnnotationSubnetID                  = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/subnet-id"  // both annotation and cloud-config
	ServiceAnnotationNetworkID                 = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/network-id" // both annotation and cloud-config
	ServiceAnnotationLoadBalancerID            = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/load-balancer-id"
	ServiceAnnotationCloudLoadBalancerName     = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/cloud-loadbalancer-name" // set via annotation
	ServiceAnnotationOwnedListeners            = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/owned-listeners"
	ServiceAnnotationLoadBalancerName          = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/load-balancer-name"      // only set via the annotation
	ServiceAnnotationPackageID                 = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/package-id"              // both annotation and cloud-config
	ServiceAnnotationEnableSecgroupDefault     = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/enable-secgroup-default" // set via annotation
	ServiceAnnotationLoadBalancerOwner         = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/load-balancer-owner"
	ServiceAnnotationIdleTimeoutClient         = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/idle-timeout-client"     // both annotation and cloud-config
	ServiceAnnotationIdleTimeoutMember         = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/idle-timeout-member"     // both annotation and cloud-config
	ServiceAnnotationIdleTimeoutConnection     = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/idle-timeout-connection" // both annotation and cloud-config
	ServiceAnnotationListenerAllowedCIDRs      = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/listener-allowed-cidrs"  // both annotation and cloud-config
	ServiceAnnotationPoolAlgorithm             = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/pool-algorithm"          // both annotation and cloud-config
	ServiceAnnotationHealthyThreshold          = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-healthy-threshold"
	ServiceAnnotationMonitorUnhealthyThreshold = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-unhealthy-threshold"
	ServiceAnnotationMonitorTimeout            = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-timeout"
	ServiceAnnotationMonitorInterval           = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-interval"
	ServiceAnnotationLoadBalancerInternal      = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/internal-load-balancer"
	ServiceAnnotationMonitorHttpMethod         = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-http-method"
	ServiceAnnotationMonitorHttpPath           = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-http-path"
	ServiceAnnotationMonitorHttpSuccessCode    = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-http-success-code"
	ServiceAnnotationMonitorHttpVersion        = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-http-version"
	ServiceAnnotationMonitorHttpDomainName     = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-http-domain-name"
	ServiceAnnotationMonitorProtocol           = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/monitor-protocol"
	ServiceAnnotationEnableStickySession       = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/enable-sticky-session"
	ServiceAnnotationEnableTLSEncryption       = DEFAULT_K8S_SERVICE_ANNOTATION_PREFIX + "/enable-tls-encryption"
)

const (
	// DeprecatedLabelNodeRoleMaster specifies that a node is a master
	// It's copied over to kubeadm until it's merged in core: https://github.com/kubernetes/kubernetes/pull/39112
	// Deprecated in favor of LabelNodeExcludeLB
	DeprecatedLabelNodeRoleMaster = "node-role.kubernetes.io/master"

	// LabelNodeExcludeLB specifies that a node should not be used to create a Loadbalancer on
	// https://github.com/kubernetes/cloud-provider/blob/25867882d509131a6fdeaf812ceebfd0f19015dd/controllers/service/controller.go#L673
	LabelNodeExcludeLB = "node.kubernetes.io/exclude-from-external-load-balancers"

	// IngressSecretCertName is certificate key name defined in the secret data.
	IngressSecretCertName = "tls.crt"
	// IngressSecretKeyName is private key name defined in the secret data.
	IngressSecretKeyName = "tls.key"

	// High enough QPS to fit all expected use cases. QPS=0 is not set here, because
	// client code is overriding it.
	DefaultQPS = 1e6
	// High enough Burst to fit all expected use cases. Burst=0 is not set here, because
	// client code is overriding it.
	DefaultBurst = 1e6

	MaxRetries = 5

	// IngressKey picks a specific "class" for the Ingress.
	// The controller only processes Ingresses with this annotation either
	// unset, or set to either the configured value or the empty string.
	IngressKey = "kubernetes.io/ingress.class"

	// IngressClass specifies which Ingress class we accept
	IngressClass = "vngcloud"
)
