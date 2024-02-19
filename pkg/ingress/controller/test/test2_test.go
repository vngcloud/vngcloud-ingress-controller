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
      secretName: secret-annd2
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
		LB_ID = "lb-17ee809f-fe17-4881-aea5-3cba65eac326"
		IP    = "180.93.181.208"
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

	resp = MakeRequest(fmt.Sprintf("https://%s/webserver", IP), "example.foo.com", headers)
	assert.Equal(t, SERVICE_UNAVAILABLE, resp)

	DeleteYAML(yaml)
	WaitLBActive(client, LB_ID)
}
