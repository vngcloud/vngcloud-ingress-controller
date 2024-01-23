package controller

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/client-go/kubernetes/scheme"

	// "strings"
	"time"

	// "github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/l7policies"
	// "github.com/gophercloud/gophercloud/openstack/loadbalancer/v2/pools"
	"github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	nwlisters "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/cloud-provider-openstack/pkg/ingress/config"
	"k8s.io/cloud-provider-openstack/pkg/ingress/utils/errors"

	// "k8s.io/cloud-provider-openstack/pkg/ingress/controller/openstack"
	"k8s.io/cloud-provider-openstack/pkg/ingress/utils"
	// cpoerrors "k8s.io/cloud-provider-openstack/pkg/util/errors"
	klog "k8s.io/klog/v2"
)

// EventType type of event associated with an informer
type EventType string

// Event holds the context of an event
type Event struct {
	Type   EventType
	Obj    interface{}
	oldObj interface{}
}

// Controller ...
type Controller struct {
	// lbProvider ILBProvider
	lbProvider *VLBProvider

	config           *config.Config
	kubeClient       kubernetes.Interface
	eventBroadcaster record.EventBroadcaster
	eventRecorder    record.EventRecorder

	stopCh              chan struct{}
	knownNodes          []*apiv1.Node
	queue               workqueue.RateLimitingInterface
	informer            informers.SharedInformerFactory
	recorder            record.EventRecorder
	ingressLister       nwlisters.IngressLister
	ingressListerSynced cache.InformerSynced
	serviceLister       corelisters.ServiceLister
	serviceListerSynced cache.InformerSynced
	nodeLister          corelisters.NodeLister
	nodeListerSynced    cache.InformerSynced
}

// NewController creates a new OpenStack Ingress controller.
func NewController(conf config.Config) *Controller {
	// initialize k8s client
	kubeClient, err := createApiserverClient(conf.Kubernetes.ApiserverHost, conf.Kubernetes.KubeConfig)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"api_server":  conf.Kubernetes.ApiserverHost,
			"kube_config": conf.Kubernetes.KubeConfig,
			"error":       err,
		}).Fatal("failed to initialize kubernetes client")
	}

	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second*30)
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	nodeInformer := kubeInformerFactory.Core().V1().Nodes()
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{
		Interface: kubeClient.CoreV1().Events(""),
	})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, apiv1.EventSource{Component: "openstack-ingress-controller"})

	controller := &Controller{
		lbProvider: &VLBProvider{
			config: &conf,
		},

		config:     &conf,
		kubeClient: kubeClient,

		queue:               queue,
		stopCh:              make(chan struct{}),
		informer:            kubeInformerFactory,
		recorder:            recorder,
		serviceLister:       serviceInformer.Lister(),
		serviceListerSynced: serviceInformer.Informer().HasSynced,
		nodeLister:          nodeInformer.Lister(),
		nodeListerSynced:    nodeInformer.Informer().HasSynced,
		knownNodes:          []*apiv1.Node{},
	}

	ingInformer := kubeInformerFactory.Networking().V1().Ingresses()
	_, err = ingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addIng := obj.(*nwv1.Ingress)
			key := fmt.Sprintf("%s/%s", addIng.Namespace, addIng.Name)

			if !IsValid(addIng) {
				logrus.Infof("ignore ingress %s", key)
				return
			}

			recorder.Event(addIng, apiv1.EventTypeNormal, "Creating", fmt.Sprintf("Ingress %s", key))
			controller.queue.AddRateLimited(Event{Obj: addIng, Type: CreateEvent, oldObj: nil})
		},
		UpdateFunc: func(old, new interface{}) {
			newIng := new.(*nwv1.Ingress)
			oldIng := old.(*nwv1.Ingress)
			if newIng.ResourceVersion == oldIng.ResourceVersion {
				// Periodic resync will send update events for all known Ingresses.
				// Two different versions of the same Ingress will always have different RVs.
				return
			}
			newAnnotations := newIng.ObjectMeta.Annotations
			oldAnnotations := oldIng.ObjectMeta.Annotations
			delete(newAnnotations, "kubectl.kubernetes.io/last-applied-configuration")
			delete(oldAnnotations, "kubectl.kubernetes.io/last-applied-configuration")

			key := fmt.Sprintf("%s/%s", newIng.Namespace, newIng.Name)
			validOld := IsValid(oldIng)
			validCur := IsValid(newIng)
			if !validOld && validCur {
				recorder.Event(newIng, apiv1.EventTypeNormal, "Creating", fmt.Sprintf("Ingress %s", key))
				controller.queue.AddRateLimited(Event{Obj: newIng, Type: CreateEvent, oldObj: nil})
			} else if validOld && !validCur {
				recorder.Event(newIng, apiv1.EventTypeNormal, "Deleting", fmt.Sprintf("Ingress %s", key))
				controller.queue.AddRateLimited(Event{Obj: newIng, Type: DeleteEvent, oldObj: nil})
			} else if validCur && (!reflect.DeepEqual(newIng.Spec, oldIng.Spec) || !reflect.DeepEqual(newAnnotations, oldAnnotations)) {
				recorder.Event(newIng, apiv1.EventTypeNormal, "Updating", fmt.Sprintf("Ingress %s", key))
				controller.queue.AddRateLimited(Event{Obj: newIng, Type: UpdateEvent, oldObj: oldIng})
			} else {
				return
			}
		},
		DeleteFunc: func(obj interface{}) {
			delIng, ok := obj.(*nwv1.Ingress)
			if !ok {
				// If we reached here it means the ingress was deleted but its final state is unrecorded.
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					logrus.Errorf("couldn't get object from tombstone %#v", obj)
					return
				}
				delIng, ok = tombstone.Obj.(*nwv1.Ingress)
				if !ok {
					logrus.Errorf("Tombstone contained object that is not an Ingress: %#v", obj)
					return
				}
			}

			key := fmt.Sprintf("%s/%s", delIng.Namespace, delIng.Name)
			if !IsValid(delIng) {
				logrus.Infof("ignore ingress %s", key)
				return
			}

			recorder.Event(delIng, apiv1.EventTypeNormal, "Deleting", fmt.Sprintf("Ingress %s", key))
			controller.queue.AddRateLimited(Event{Obj: delIng, Type: DeleteEvent, oldObj: nil})
		},
	})

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("failed to initialize ingress")
	}

	controller.ingressLister = ingInformer.Lister()
	controller.ingressListerSynced = ingInformer.Informer().HasSynced

	return controller
}

// Start starts the openstack ingress controller.
func (c *Controller) Start() {
	klog.Infoln("------------ Start() ------------")
	defer close(c.stopCh)
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	logrus.Debug("starting Ingress controller")
	c.lbProvider.Init()
	go c.informer.Start(c.stopCh)

	// wait for the caches to synchronize before starting the worker
	if !cache.WaitForCacheSync(c.stopCh, c.ingressListerSynced, c.serviceListerSynced, c.nodeListerSynced) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}
	logrus.Info("ingress controller synced and ready")

	readyWorkerNodes, err := listWithPredicate(c.nodeLister, getNodeConditionPredicate())
	if err != nil {
		logrus.Errorf("Failed to retrieve current set of nodes from node lister: %v", err)
		return
	}
	c.knownNodes = readyWorkerNodes

	go wait.Until(c.runWorker, time.Second, c.stopCh)
	go wait.Until(c.nodeSyncLoop, 60*time.Second, c.stopCh)

	<-c.stopCh
}

// nodeSyncLoop handles updating the hosts pointed to by all load
// balancers whenever the set of nodes in the cluster changes.
func (c *Controller) nodeSyncLoop() {
	klog.Infoln("------------ nodeSyncLoop() ------------")
	readyWorkerNodes, err := listWithPredicate(c.nodeLister, getNodeConditionPredicate())
	if err != nil {
		logrus.Errorf("Failed to retrieve current set of nodes from node lister: %v", err)
		return
	}
	if utils.NodeSlicesEqual(readyWorkerNodes, c.knownNodes) {
		return
	}

	logrus.Infof("Detected change in list of current cluster nodes. New node set: %v", utils.NodeNames(readyWorkerNodes))

	// if no new nodes, then avoid update member
	if len(readyWorkerNodes) == 0 {
		c.knownNodes = readyWorkerNodes
		logrus.Info("Finished to handle node change, it's [] now")
		return
	}

	var ings *nwv1.IngressList
	// NOTE(lingxiankong): only take ingresses without ip address into consideration
	opts := apimetav1.ListOptions{}
	if ings, err = c.kubeClient.NetworkingV1().Ingresses("").List(context.TODO(), opts); err != nil {
		logrus.Errorf("Failed to retrieve current set of ingresses: %v", err)
		return
	}

	// Update each valid ingress
	for _, ing := range ings.Items {
		if !IsValid(&ing) {
			continue
		}

		logrus.WithFields(logrus.Fields{"ingress": ing.Name, "namespace": ing.Namespace}).Debug("Starting to handle ingress")

		// if user pass lb id in annotation, get the lbID from annotation
		lbID, err := c.lbProvider.GetLoadbalancerIDByIngress(&ing)
		if err != nil {
			if err == errors.ErrLoadBalancerIDNotFoundAnnotation {
				logrus.WithFields(logrus.Fields{"id": lbID}).Errorf("Failed to retrieve loadbalancer from VNGCLOUD: %v", err)
			}

			// If lb doesn't exist or error occurred, continue
			continue
		}
		loadbalancer, err := c.lbProvider.GetLoadbalancerByID(lbID)

		if err = c.lbProvider.UpdateLoadbalancerMembers(loadbalancer.ID, readyWorkerNodes); err != nil {
			logrus.WithFields(logrus.Fields{"ingress": ing.Name}).Error("Failed to handle ingress")
			continue
		}

		logrus.WithFields(logrus.Fields{"ingress": ing.Name, "namespace": ing.Namespace}).Info("Finished to handle ingress")
	}

	c.knownNodes = readyWorkerNodes

	logrus.Info("Finished to handle node change")
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
		// continue looping
	}
}

func (c *Controller) processNextItem() bool {
	klog.Infoln("--------------------- processNextItem() ---------------------")
	obj, quit := c.queue.Get()

	if quit {
		return false
	}
	defer c.queue.Done(obj)

	err := c.processItem(obj.(Event))
	if err == nil {
		// No error, reset the ratelimit counters
		c.queue.Forget(obj)
	} else if c.queue.NumRequeues(obj) < maxRetries {
		logrus.WithFields(logrus.Fields{"obj": obj, "error": err}).Error("Failed to process obj (will retry)")
		c.queue.AddRateLimited(obj)
	} else {
		// err != nil and too many retries
		logrus.WithFields(logrus.Fields{"obj": obj, "error": err}).Error("Failed to process obj (giving up)")
		c.queue.Forget(obj)
		utilruntime.HandleError(err)
	}

	return true
}

func (c *Controller) processItem(event Event) error {
	klog.Infoln("---------------- processItem() ----------------")
	klog.Infoln("EVENT:", event)
	// time.Sleep(90 * time.Second)
	// return nil

	ing := event.Obj.(*nwv1.Ingress)
	var oldIng *nwv1.Ingress
	oldIng = nil
	if event.oldObj != nil {
		oldIng = event.oldObj.(*nwv1.Ingress)
	}
	key := fmt.Sprintf("%s/%s", ing.Namespace, ing.Name)
	logger := logrus.WithFields(logrus.Fields{"ingress": key})

	switch event.Type {
	case CreateEvent:
		logger.Info("creating ingress")

		if err := c.ensureIngress(oldIng, ing); err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to create openstack resources for ingress %s: %v", key, err))
			c.recorder.Event(ing, apiv1.EventTypeWarning, "Failed", fmt.Sprintf("Failed to create openstack resources for ingress %s: %v", key, err))
		} else {
			c.recorder.Event(ing, apiv1.EventTypeNormal, "Created", fmt.Sprintf("Ingress %s", key))
		}
	case UpdateEvent:
		logger.Info("updating ingress")

		if err := c.ensureIngress(oldIng, ing); err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to update openstack resources for ingress %s: %v", key, err))
			c.recorder.Event(ing, apiv1.EventTypeWarning, "Failed", fmt.Sprintf("Failed to update openstack resources for ingress %s: %v", key, err))
		} else {
			c.recorder.Event(ing, apiv1.EventTypeNormal, "Updated", fmt.Sprintf("Ingress %s", key))
		}
	case DeleteEvent:
		logger.Info("deleting ingress")

		if err := c.deleteIngress(ing); err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to delete openstack resources for ingress %s: %v", key, err))
			c.recorder.Event(ing, apiv1.EventTypeWarning, "Failed", fmt.Sprintf("Failed to delete openstack resources for ingress %s: %v", key, err))
		} else {
			c.recorder.Event(ing, apiv1.EventTypeNormal, "Deleted", fmt.Sprintf("Ingress %s", key))
		}
	}

	klog.Infoln("DONEEEEEEEEEEEEEEEEEEEEEEE")

	return nil
}

func (c *Controller) deleteIngress(ing *nwv1.Ingress) error {
	klog.Infoln("------------ deleteIngress() ------------")
	err := c.lbProvider.DeleteLoadbalancer(c, ing)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) ensureIngress(oldIng, ing *nwv1.Ingress) error {
	klog.Infoln("------------ ensureIngress() ------------")
	lb, err := c.lbProvider.EnsureLoadBalancer(c, oldIng, ing)
	if err != nil {
		return err
	}
	klog.Infoln(lb)
	return nil
}

// for vLB callback

func (c *Controller) GetNodeMembersAddr() ([]string, error) {
	nodeObjs, err := listWithPredicate(c.nodeLister, getNodeConditionPredicate())
	if err != nil {
		klog.Errorf("Failed to retrieve current set of nodes from node lister: %v", err)
		return nil, err
	}
	var updateMemberOpts []string
	for _, node := range nodeObjs {
		addr, err := getNodeAddressForLB(node)
		if err != nil {
			// Node failure, do not create member
			logrus.WithFields(logrus.Fields{"node": node.Name, "error": err}).Warn("failed to get node address")
			continue
		}
		updateMemberOpts = append(updateMemberOpts, addr)
	}
	return updateMemberOpts, nil
}

func (c *Controller) updateIngressStatus(ing *nwv1.Ingress, vip string) (*nwv1.Ingress, error) {
	newState := new(nwv1.IngressLoadBalancerStatus)
	newState.Ingress = []nwv1.IngressLoadBalancerIngress{{IP: vip}}
	newIng := ing.DeepCopy()
	newIng.Status.LoadBalancer = *newState

	newObj, err := c.kubeClient.NetworkingV1().Ingresses(newIng.Namespace).UpdateStatus(context.TODO(), newIng, apimetav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return newObj, nil
}

func (c *Controller) getService(key string) (*apiv1.Service, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	service, err := c.serviceLister.Services(namespace).Get(name)
	if err != nil {
		return nil, err
	}

	return service, nil
}

func (c *Controller) getServiceNodePort(name string, serviceBackend *nwv1.IngressServiceBackend) (int, error) {
	var portInfo intstr.IntOrString
	if serviceBackend.Port.Name != "" {
		portInfo.Type = intstr.String
		portInfo.StrVal = serviceBackend.Port.Name
	} else {
		portInfo.Type = intstr.Int
		portInfo.IntVal = serviceBackend.Port.Number
	}

	logger := logrus.WithFields(logrus.Fields{"service": name, "port": portInfo.String()})

	logger.Debug("getting service nodeport")

	svc, err := c.getService(name)
	if err != nil {
		return 0, err
	}

	var nodePort int
	ports := svc.Spec.Ports
	for _, p := range ports {
		if portInfo.Type == intstr.Int && int(p.Port) == portInfo.IntValue() {
			nodePort = int(p.NodePort)
			break
		}
		if portInfo.Type == intstr.String && p.Name == portInfo.StrVal {
			nodePort = int(p.NodePort)
			break
		}
	}

	if nodePort == 0 {
		return 0, fmt.Errorf("failed to find nodeport for service %s", name)
	}

	logger.Debug("found service nodeport")

	return nodePort, nil
}
