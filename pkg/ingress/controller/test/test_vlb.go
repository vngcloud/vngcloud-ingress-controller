package test

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vngcloud/vngcloud-go-sdk/client"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud"
	lObjects "github.com/vngcloud/vngcloud-go-sdk/vngcloud/objects"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/identity/v2/extensions/oauth2"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/identity/v2/tokens"
	"k8s.io/cloud-provider-openstack/pkg/ingress/controller"
)

var (
	IDENTITY_URL  = ""
	vLbURL        = ""
	CLIENT_ID     = ""
	CLIENT_SECRET = ""
	PROJECT_ID    = ""

	API = controller.API{}
)

func NewVNGCLOUDClient() (*client.ServiceClient, *client.ServiceClient) {
	provider, err := vngcloud.NewClient(IDENTITY_URL)
	if err != nil {
		logrus.Errorf("failed to init VNGCLOUD client")
		return nil, nil
	}
	err = vngcloud.Authenticate(provider, &oauth2.AuthOptions{
		ClientID:     CLIENT_ID,
		ClientSecret: CLIENT_SECRET,
		AuthOptionsBuilder: &tokens.AuthOptions{
			IdentityEndpoint: IDENTITY_URL,
		},
	})
	if err != nil {
		logrus.Errorf("failed to Authenticate VNGCLOUD client")
		return nil, nil
	}

	vlbSC, err := vngcloud.NewServiceClient(
		"https://hcm-3.api.vngcloud.vn/vserver/vlb-gateway/v2",
		provider, "vlb-gateway")
	if err != nil {
		logrus.Errorf("failed to init VLB VNGCLOUD client")
		return nil, nil
	}

	vserverSC, err := vngcloud.NewServiceClient(
		"https://hcm-3.api.vngcloud.vn/vserver/vserver-gateway/v2",
		provider, "vserver-gateway")
	if err != nil {
		logrus.Errorf("failed to init VSERVER VNGCLOUD client")
		return nil, nil
	}

	return vlbSC, vserverSC
}

func ClearLB(client *client.ServiceClient, lbID string) {
	logrus.Infoln("####################### CLEAR LB #######################")
	// clear all the listeners, pools
	WaitLBActive(client, lbID)
	lis, err := API.ListListenerOfLB(client, PROJECT_ID, lbID)
	if err != nil {
		logrus.Errorf("Error getting listeners of LB: %v\n", err)
		return
	}
	for _, li := range lis {
		pols, err := API.ListPolicyOfListener(client, PROJECT_ID, lbID, li.UUID)
		if err != nil {
			logrus.Errorf("Error getting policies of listener: %v\n", err)
			return
		}
		for _, pol := range pols {
			err = API.DeletePolicy(client, PROJECT_ID, lbID, li.UUID, pol.UUID)
			if err != nil {
				logrus.Errorf("Error deleting policy: %v\n", err)
				return
			}
			WaitLBActive(client, lbID)
		}
		err = API.DeleteListener(client, PROJECT_ID, lbID, li.UUID)
		if err != nil {
			logrus.Errorf("Error deleting listener: %v\n", err)
			return
		}
		WaitLBActive(client, lbID)
	}

	pools, err := API.ListPoolOfLB(client, PROJECT_ID, lbID)
	if err != nil {
		logrus.Errorf("Error getting pools of LB: %v\n", err)
		return
	}
	for _, pool := range pools {
		err = API.DeletePool(client, PROJECT_ID, lbID, pool.UUID)
		if err != nil {
			logrus.Errorf("Error deleting pool: %v\n", err)
			return
		}
		WaitLBActive(client, lbID)
	}
}

func WaitLBActive(client *client.ServiceClient, lbID string) *lObjects.LoadBalancer {
	count := 0
	for {
		lb, err := API.GetLB(client, PROJECT_ID, lbID)
		if err != nil {
			logrus.Errorln("error when get lb status: ", err)
		} else if lb.Status == "ACTIVE" {
			count++
		} else {
			count = 0
		}

		if count == 4 {
			return lb
		}
		logrus.Infoln("------- wait for lb active:", lb.Status, "-------")
		time.Sleep(5 * time.Second)
	}
}
