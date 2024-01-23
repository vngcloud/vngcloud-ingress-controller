package client

import (
	"k8s.io/klog/v2"
)

func LogCfg(pAuthOpts AuthOpts) {
	klog.V(5).Infof("Init client with config: %+v", pAuthOpts)
}
