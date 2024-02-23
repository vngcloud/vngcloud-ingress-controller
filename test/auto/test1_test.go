package test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHttp(t *testing.T) {

	const dangbh2 = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dangbh2
  annotations:
    kubernetes.io/ingress.class: "vngcloud"
    vks.vngcloud.vn/load-balancer-id: "%s"
spec:
  rules:
    - http:
        paths:
          - path: /1
            pathType: Exact
            backend:
              service:
                name: goapp-debug
                port:
                  number: 1111
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

	headers := http.Header{}

	resp := MakeRequest(fmt.Sprintf("http://%s/2", IP), "", headers)
	assert.Equal(t, SERVICE_UNAVAILABLE, resp)

	resp = MakeRequest(fmt.Sprintf("http://%s/1", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 1111, path: /1\"}", resp)

	// DeleteYAML(yaml)
	// WaitLBActive(client, LB_ID)
}
