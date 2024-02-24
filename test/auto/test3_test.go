package test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultBackend(t *testing.T) {

	const dangbh2 = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dangbh2
  annotations:
    kubernetes.io/ingress.class: "vngcloud"
    vks.vngcloud.vn/load-balancer-id: "%s"
spec:
  defaultBackend:
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

	resp := MakeRequest(fmt.Sprintf("http://%s/345678", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 1111, path: /345678\"}", resp)

	resp = MakeRequest(fmt.Sprintf("http://%s/sfsrr", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 1111, path: /sfsrr\"}", resp)

	// DeleteYAML(yaml)
	// WaitLBActive(LB_ID)
}
