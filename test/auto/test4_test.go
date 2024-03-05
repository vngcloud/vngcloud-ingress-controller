package test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// create 2 ingress with default backend then delete the first one
func TestDefaultBackend2(t *testing.T) {

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

	const tantm3 = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: tantm3
  annotations:
    kubernetes.io/ingress.class: "vngcloud"
    vks.vngcloud.vn/load-balancer-id: "%s"
spec:
  defaultBackend:
    service:
      name: goapp-debug
      port:
        number: 2222
`
	// const (
	// 	LB_ID = "lb-67cd0bbc-4c27-4e5d-b728-9f416a509577"
	// 	IP    = "180.93.181.81"
	// )
	const (
		LB_ID = "lb-17ee809f-fe17-4881-aea5-3cba65eac326"
		IP    = "180.93.181.208"
	)

	client, _ := NewVNGCLOUDClient()
	WaitLBActive(LB_ID)

	yaml1 := fmt.Sprintf(dangbh2, LB_ID)
	DeleteYAML(yaml1)
	WaitLBActive(LB_ID)

	yaml2 := fmt.Sprintf(tantm3, LB_ID)
	DeleteYAML(yaml2)
	WaitLBActive(LB_ID)

	ClearLB(client, LB_ID)
	WaitLBActive(LB_ID)

	ApplyYAML(yaml1)
	WaitLBActive(LB_ID)

	headers := http.Header{
		// "Host": []string{"https-example.foo.com"},
	}
	resp := MakeRequest(fmt.Sprintf("http://%s/345678", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 1111, path: /345678\"}", resp)

	resp = MakeRequest(fmt.Sprintf("http://%s/sdfgh6", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 1111, path: /sdfgh6\"}", resp)

	ApplyYAML(yaml2)
	WaitLBActive(LB_ID)

	resp = MakeRequest(fmt.Sprintf("http://%s/345678", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 2222, path: /345678\"}", resp)

	resp = MakeRequest(fmt.Sprintf("http://%s/sdfgh6", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 2222, path: /sdfgh6\"}", resp)

	DeleteYAML(yaml2)
	WaitLBActive(LB_ID)

	resp = MakeRequest(fmt.Sprintf("http://%s/345678", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 1111, path: /345678\"}", resp)

	resp = MakeRequest(fmt.Sprintf("http://%s/sdfgh6", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Port: 1111, path: /sdfgh6\"}", resp)

	// DeleteYAML(yaml1)
	// WaitLBActive(LB_ID)
}
