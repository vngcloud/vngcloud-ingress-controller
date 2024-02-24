package test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHttpAndHttps(t *testing.T) {

	const dangbh2 = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dangbh2
  annotations:
    kubernetes.io/ingress.class: "vngcloud"
    vks.vngcloud.vn/load-balancer-id: "%s"
spec:
  # ingressClassName: "vngcloud"
  defaultBackend:
    service:
      name: goapp-debug
      port:
        number: 1111
  tls:
    - hosts:
        - example.com
      secretName: secret-annd2
    - hosts:
        - kkk.example.com
        - hhh.example.com
      secretName: secret-wildcard-annd2
  rules:
    - http:
        paths:
          - path: /2
            pathType: Exact
            backend:
              service:
                name: goapp-debug
                port:
                  number: 2222
    - host: example.com
      http:
        paths:
          - path: /3
            pathType: Exact
            backend:
              service:
                name: goapp-debug
                port:
                  number: 3333
    - host: kkk.example.com
      http:
        paths:
          - path: /4
            pathType: Exact
            backend:
              service:
                name: goapp-debug
                port:
                  number: 4444
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

	headers := http.Header{}

	// to http 1
	resp := MakeRequest(fmt.Sprintf("http://%s/2", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 2222, path: /2\"}", resp)

	resp = MakeRequest(fmt.Sprintf("http://%s/22", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 1111, path: /22\"}", resp)

	//to default backend
	resp = MakeRequest(fmt.Sprintf("http://%s/webb", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 1111, path: /webb\"}", resp)

	resp = MakeRequest(fmt.Sprintf("https://%s/webb", IP), "example.com", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 1111, path: /webb\"}", resp)

	resp = MakeRequest(fmt.Sprintf("https://%s/webb", IP), "example2.com", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 1111, path: /webb\"}", resp)

	// to https 1
	resp = MakeRequest(fmt.Sprintf("https://%s/3", IP), "example.com", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 3333, path: /3\"}", resp)

	// to https 2
	resp = MakeRequest(fmt.Sprintf("https://%s/4", IP), "kkk.example.com", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 4444, path: /4\"}", resp)

	// DeleteYAML(yaml)
	// WaitLBActive(LB_ID)
}
