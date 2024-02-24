package test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHttps(t *testing.T) {
	const dangbh2 = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dangbh2
  annotations:
    kubernetes.io/ingress.class: "vngcloud"
    vks.vngcloud.vn/load-balancer-id: "%s"
spec:
  tls:
    - hosts:
        - https-example.foo.com 
      secretName: secret-annd2
    - hosts:
        - hhh.example.com
      secretName: secret-wildcard-annd2
  rules:
    - host: https-example.foo.com
      http:
        paths:
          - path: /1
            pathType: Exact
            backend:
              service:
                name: goapp-debug
                port:
                  number: 1111
    - host: hhh.example.com
      http:
        paths:
          - path: /3
            pathType: Exact
            backend:
              service:
                name: goapp-debug
                port:
                  number: 3333
`
	const (
		LB_ID = "lb-17ee809f-fe17-4881-aea5-3cba65eac326"
		IP    = "180.93.181.208"
	)

	client, _ := NewVNGCLOUDClient()
	WaitLBActive(LB_ID)

	yaml := fmt.Sprintf(dangbh2, LB_ID)
	DeleteYAML(yaml)
	WaitLBActive(LB_ID)

	ClearLB(client, LB_ID)
	WaitLBActive(LB_ID)

	ApplyYAML(yaml)
	WaitLBActive(LB_ID)

	headers := http.Header{
		// "Host": []string{"https-example.foo.com"},
	}

	resp := MakeRequest(fmt.Sprintf("https://%s/1", IP), "https-example.foo.com", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 1111, path: /1\"}", resp)

	resp = MakeRequest(fmt.Sprintf("https://%s/2", IP), "https-example.foo.com", headers)
	assert.Equal(t, SERVICE_UNAVAILABLE, resp)

	resp = MakeRequest(fmt.Sprintf("https://%s/1", IP), "example.foo.com", headers)
	assert.Equal(t, SERVICE_UNAVAILABLE, resp)

	resp = MakeRequest(fmt.Sprintf("https://%s/3", IP), "example.com", headers)
	assert.Equal(t, SERVICE_UNAVAILABLE, resp)

	resp = MakeRequest(fmt.Sprintf("https://%s/3", IP), "hhh.example.com", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 3333, path: /3\"}", resp)

	resp = MakeRequest(fmt.Sprintf("https://%s/1", IP), "hhh.example.com", headers)
	assert.Equal(t, SERVICE_UNAVAILABLE, resp)

	resp = MakeRequest(fmt.Sprintf("https://%s/1", IP), "example.com", headers)
	assert.Equal(t, SERVICE_UNAVAILABLE, resp)

	// DeleteYAML(yaml)
	// WaitLBActive( LB_ID)
}
