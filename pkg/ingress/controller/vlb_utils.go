package controller

import (
	"fmt"
	client2 "github.com/vngcloud/vngcloud-go-sdk/client"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud"
	v1Portal "github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/portal/v1"
	"k8s.io/cloud-provider-openstack/pkg/ingress/utils/metadata"
	"k8s.io/klog/v2"
)

func getMetadataOption(pMetadata metadata.Opts) metadata.Opts {
	if pMetadata.SearchOrder == "" {
		pMetadata.SearchOrder = fmt.Sprintf("%s,%s", metadata.ConfigDriveID, metadata.MetadataID)
	}
	klog.Info("getMetadataOption; metadataOpts is ", pMetadata)
	return pMetadata
}

func setupPortalInfo(pProvider *client2.ProviderClient, pMedatata metadata.IMetadata, pPortalURL string) (*ExtraInfo, error) {
	projectID, err := pMedatata.GetProjectID()
	if err != nil {
		return nil, err
	}

	portalClient, _ := vngcloud.NewServiceClient(pPortalURL, pProvider, "portal")
	portalInfo, err := v1Portal.Get(portalClient, projectID)

	if err != nil {
		return nil, err
	}

	if portalInfo == nil {
		return nil, fmt.Errorf("can not get portal information")
	}

	return &ExtraInfo{
		ProjectID: portalInfo.ProjectID,
		UserID:    portalInfo.UserID,
	}, nil
}
