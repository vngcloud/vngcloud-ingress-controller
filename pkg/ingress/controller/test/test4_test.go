package test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

// create 2 ingress with default backend then delete the first one
func TestDefaultBackend2(t *testing.T) {

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
  defaultBackend:
    service:
      name: goapp-debug
      port:
        number: 8888
`

	const tantm3 = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: tantm3
  annotations:
    kubernetes.io/ingress.class: "openstack"
    octavia.ingress.kubernetes.io/internal: "false"
    vngcloud.vngcloud.vn/load-balancer-id: "%s"
spec:
  defaultBackend:
    service:
      name: webserver
      port:
        number: 8080
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
	WaitLBActive(client, LB_ID)

	yaml1 := fmt.Sprintf(dangbh2, LB_ID)
	DeleteYAML(yaml1)
	WaitLBActive(client, LB_ID)

	yaml2 := fmt.Sprintf(tantm3, LB_ID)
	DeleteYAML(yaml2)
	WaitLBActive(client, LB_ID)

	ClearLB(client, LB_ID)
	WaitLBActive(client, LB_ID)

	ApplyYAML(yaml1)
	WaitLBActive(client, LB_ID)

	headers := http.Header{
		// "Host": []string{"https-example.foo.com"},
	}
	resp := MakeRequest(fmt.Sprintf("http://%s/345678", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Received request on port 8888, path: /345678\"}", resp)

	resp = MakeRequest(fmt.Sprintf("http://%s/sdfgh6", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Received request on port 8888, path: /sdfgh6\"}", resp)

	ApplyYAML(yaml2)
	WaitLBActive(client, LB_ID)

	resp = MakeRequest(fmt.Sprintf("http://%s/345678", IP), "", headers)
	assert.Equal(t, "webserver-6c7fb64575-lsxxn\n", resp)

	resp = MakeRequest(fmt.Sprintf("http://%s/sdfgh6", IP), "", headers)
	assert.Equal(t, "webserver-6c7fb64575-lsxxn\n", resp)

	DeleteYAML(yaml2)
	WaitLBActive(client, LB_ID)

	resp = MakeRequest(fmt.Sprintf("http://%s/345678", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Received request on port 8888, path: /345678\"}", resp)

	resp = MakeRequest(fmt.Sprintf("http://%s/sdfgh6", IP), "", headers)
	assert.Equal(t, "{\"received_path\":\"Received request on port 8888, path: /sdfgh6\"}", resp)

	DeleteYAML(yaml1)
	WaitLBActive(client, LB_ID)
}
