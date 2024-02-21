package test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func TestDefaultBackend(t *testing.T) {

	const dangbh2 = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dangbh2
  annotations:
    kubernetes.io/ingress.class: "vngcloud"
    vngcloud.vngcloud.vn/load-balancer-id: "%s"
spec:
  defaultBackend:
    service:
      name: goapp-debug
      port:
        number: 8888
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

	resp := MakeRequest(fmt.Sprintf("http://%s/345678", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Received request on port 8888, path: /345678\"}", resp)

	resp = MakeRequest(fmt.Sprintf("http://%s/sdfgh6", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Received request on port 8888, path: /sdfgh6\"}", resp)

	DeleteYAML(yaml)
	WaitLBActive(client, LB_ID)
}
