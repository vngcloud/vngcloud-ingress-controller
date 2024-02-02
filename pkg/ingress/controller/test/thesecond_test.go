package test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestHttps(t *testing.T) {

	const dangbh2 = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dangbh2
  annotations:
    kubernetes.io/ingress.class: "openstack"
    octavia.ingress.kubernetes.io/internal: "false"
    vngcloud.vngcloud.vn/load-balancer-id: "%s"
spec:
  tls:
    - hosts:
        - https-example.foo.com
      secretName: secret-b699b868-ca06-4116-8f14-4ea6bedcfb73
  rules:
    - host: https-example.foo.com
      http:
        paths:
          - path: /webserver
            pathType: Exact
            backend:
              service:
                name: webserver
                port:
                  number: 8080
`
	const (
		LB_ID = "lb-67cd0bbc-4c27-4e5d-b728-9f416a509577"
		IP    = "180.93.181.81"
	)

	client, _ := NewVNGCLOUDClient()
	WaitLBActive(client, LB_ID)

	yaml := fmt.Sprintf(dangbh2, LB_ID)
	DeleteYAML(yaml)
	WaitLBActive(client, LB_ID)

	ClearLB(client, LB_ID)
	WaitLBActive(client, LB_ID)

	ApplyYAML(yaml)
	WaitLBActive(client, LB_ID)

	headers := http.Header{
		// "Host": []string{"https-example.foo.com"},
	}

	resp := MakeRequest(fmt.Sprintf("https://%s/webb", IP), "https-example.foo.com", headers)
	assert.Equal(t, SERVICE_UNAVAILABLE, resp)

	resp = MakeRequest(fmt.Sprintf("https://%s/webserver", IP), "https-example.foo.com", headers)
	assert.Equal(t, "webserver-6c7fb64575-lsxxn\n", resp)
}
