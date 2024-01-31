package consts

import (
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/listener"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/pool"
)

const (
	DEFAULT_PORTAL_NAME_LENGTH        = 50      // All the name must be less than 50 characters
	DEFAULT_PORTAL_DESCRIPTION_LENGTH = 255     // All the description must be less than 255 characters
	DEFAULT_LB_PREFIX_NAME            = "annd2" // "clu" is abbreviated of "cluster"
	DEFAULT_NAME_DEFAULT_POOL         = "annd2_default_pool"
	DEFAULT_PACKAGE_ID                = "lbp-f562b658-0fd4-4fa6-9c57-c1a803ccbf86"
	DEFAULT_HTTPS_LISTENER_NAME       = "annd2_https_listener"
	DEFAULT_HTTP_LISTENER_NAME        = "annd2_http_listener"
)

var OPT_POOL_DEFAULT = pool.CreateOpts{
	Algorithm:     pool.CreateOptsAlgorithmOptSourceIP, // CreateOptsAlgorithmOptLeastConn, CreateOptsAlgorithmOptRoundRobin
	PoolName:      DEFAULT_NAME_DEFAULT_POOL,
	PoolProtocol:  pool.CreateOptsProtocolOptHTTP,
	Stickiness:    PointerOf[bool](false),
	TLSEncryption: PointerOf[bool](false),
	HealthMonitor: pool.HealthMonitor{
		HealthyThreshold:    3,
		UnhealthyThreshold:  3,
		Interval:            30,
		Timeout:             5,
		HealthCheckProtocol: pool.CreateOptsHealthCheckProtocolOptTCP,
	},
	Members: []pool.Member{},
}

var OPT_LISTENER_HTTP_DEFAULT = listener.CreateOpts{
	ListenerName:         DEFAULT_HTTP_LISTENER_NAME,
	DefaultPoolId:        "",
	AllowedCidrs:         "0.0.0.0/0",
	ListenerProtocol:     listener.CreateOptsListenerProtocolOptHTTP,
	ListenerProtocolPort: 80,
	TimeoutClient:        50,
	TimeoutConnection:    5,
	TimeoutMember:        50,
}

var OPT_LISTENER_HTTPS_DEFAULT = listener.CreateOpts{
	ListenerName:                DEFAULT_HTTPS_LISTENER_NAME,
	DefaultPoolId:               "",
	AllowedCidrs:                "0.0.0.0/0",
	ListenerProtocol:            listener.CreateOptsListenerProtocolOptHTTPS,
	ListenerProtocolPort:        443,
	TimeoutClient:               50,
	TimeoutConnection:           5,
	TimeoutMember:               50,
	CertificateAuthorities:      nil,
	ClientCertificate:           nil,
	DefaultCertificateAuthority: nil,
}

func PointerOf[T any](t T) *T {
	return &t
}
