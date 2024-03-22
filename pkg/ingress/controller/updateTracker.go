package controller

import (
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/objects"
	"k8s.io/klog/v2"
)

type update struct {
	updateAt string
	ingress  []string
}

type UpdateTracker struct {
	tracker map[string]*update
}

func NewUpdateTracker() *UpdateTracker {
	return &UpdateTracker{
		tracker: make(map[string]*update),
	}
}

func (c *UpdateTracker) AddUpdateTracker(lbID, ingressName, updateAt string) {
	if _, ok := c.tracker[lbID]; !ok {
		c.tracker[lbID] = &update{
			updateAt: updateAt,
			ingress:  []string{ingressName},
		}
	} else {
		c.tracker[lbID].updateAt = updateAt
		for _, ingress := range c.tracker[lbID].ingress {
			if ingress == ingressName {
				return
			}
		}
		c.tracker[lbID].ingress = append(c.tracker[lbID].ingress, ingressName)
	}
}

func (c *UpdateTracker) RemoveUpdateTracker(lbID, ingressName string) {
	if _, ok := c.tracker[lbID]; ok {
		for i, ingress := range c.tracker[lbID].ingress {
			if ingress == ingressName {
				c.tracker[lbID].ingress = append(c.tracker[lbID].ingress[:i], c.tracker[lbID].ingress[i+1:]...)
				if len(c.tracker[lbID].ingress) == 0 {
					delete(c.tracker, lbID)
				}
				return
			}
		}
	}
}

func (c *UpdateTracker) GetReapplyIngress(lbs []*objects.LoadBalancer) []string {
	lbNames := ""
	for key := range c.tracker {
		lbNames += " " + key + ","
	}
	klog.V(3).Infof("Watching these loadbalancers: %s", lbNames)
	var reapplyIngress []string
	for _, lb := range lbs {
		if _, ok := c.tracker[lb.UUID]; ok {
			if c.tracker[lb.UUID].updateAt != lb.UpdatedAt {
				klog.V(3).Infof("Loadbalancer %s has been updated, sync now.", lb.UUID)
				reapplyIngress = append(reapplyIngress, c.tracker[lb.UUID].ingress...)
			}
		}
	}
	return reapplyIngress
}
