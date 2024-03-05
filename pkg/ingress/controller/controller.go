package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	vconSdkClient "github.com/vngcloud/vngcloud-go-sdk/client"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud"
	lObjects "github.com/vngcloud/vngcloud-go-sdk/vngcloud/objects"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/identity/v2/extensions/oauth2"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/identity/v2/tokens"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/certificates"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/listener"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/loadbalancer"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/policy"
	"github.com/vngcloud/vngcloud-go-sdk/vngcloud/services/loadbalancer/v2/pool"
	"github.com/vngcloud/vngcloud-ingress-controller/pkg/ingress/config"
	"github.com/vngcloud/vngcloud-ingress-controller/pkg/ingress/consts"
	"github.com/vngcloud/vngcloud-ingress-controller/pkg/ingress/utils/errors"
	"github.com/vngcloud/vngcloud-ingress-controller/pkg/ingress/utils/metadata"
	apiv1 "k8s.io/api/core/v1"
	nwv1 "k8s.io/api/networking/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	nwlisters "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
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
	config     *config.Config
	kubeClient kubernetes.Interface

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

	// vks
	provider  *vconSdkClient.ProviderClient
	vLBSC     *vconSdkClient.ServiceClient
	vServerSC *vconSdkClient.ServiceClient
	cluster   *lObjects.Cluster
	extraInfo *ExtraInfo
	api       API

	SecretTrackers      *SecretTracker
	isUpdateDefaultPool bool // it have a bug when update default pool member, set this to reapply when update pool member
}

// NewController creates a new VngCloud Ingress controller.
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
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, apiv1.EventSource{Component: "vngcloud-ingress-controller"})

	controller := &Controller{
		config:         &conf,
		kubeClient:     kubeClient,
		SecretTrackers: NewSecretTracker(),

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

// Start starts the vngcloud ingress controller.
func (c *Controller) Start() {
	klog.Infoln("------------ Start() ------------")
	defer close(c.stopCh)
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	logrus.Debug("starting Ingress controller")
	err := c.Init()
	if err != nil {
		logrus.Fatal("failed to init controller: ", err)
	}

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

	isReApply := false
	if !NodeSlicesEqual(readyWorkerNodes, c.knownNodes) {
		isReApply = true
		logrus.Infof("Detected change in list of current cluster nodes. Node set: %v", NodeNames(readyWorkerNodes))
	}
	if c.isUpdateDefaultPool {
		c.isUpdateDefaultPool = false
		isReApply = true
	}
	if c.SecretTrackers.CheckSecretTrackerChange(c.kubeClient) {
		isReApply = true
		logrus.Infof("Detected change in secret tracker")
	}
	if !isReApply {
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
		err := c.ensureIngress(nil, &ing)
		if err != nil {
			logrus.WithFields(logrus.Fields{"ingress": ing.Name, "namespace": ing.Namespace}).Error("Failed to handle ingress:", err)
			continue
		}
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
	klog.Infoln("EVENT:", event.Type)
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
			utilruntime.HandleError(fmt.Errorf("failed to create vngcloud resources for ingress %s: %v", key, err))
			c.recorder.Event(ing, apiv1.EventTypeWarning, "Failed", fmt.Sprintf("Failed to create vngcloud resources for ingress %s: %v", key, err))
		} else {
			c.recorder.Event(ing, apiv1.EventTypeNormal, "Created", fmt.Sprintf("Ingress %s", key))
		}
	case UpdateEvent:
		logger.Info("updating ingress")

		if err := c.ensureIngress(oldIng, ing); err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to update vngcloud resources for ingress %s: %v", key, err))
			c.recorder.Event(ing, apiv1.EventTypeWarning, "Failed", fmt.Sprintf("Failed to update vngcloud resources for ingress %s: %v", key, err))
		} else {
			c.recorder.Event(ing, apiv1.EventTypeNormal, "Updated", fmt.Sprintf("Ingress %s", key))
		}
	case DeleteEvent:
		logger.Info("deleting ingress")

		if err := c.deleteIngress(ing); err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to delete vngcloud resources for ingress %s: %v", key, err))
			c.recorder.Event(ing, apiv1.EventTypeWarning, "Failed", fmt.Sprintf("Failed to delete vngcloud resources for ingress %s: %v", key, err))
		} else {
			c.recorder.Event(ing, apiv1.EventTypeNormal, "Deleted", fmt.Sprintf("Ingress %s", key))
		}
	}
	klog.Infoln("DONE processItem()")
	return nil
}

func (c *Controller) deleteIngress(ing *nwv1.Ingress) error {
	klog.Infoln("------------ deleteIngress() ------------")
	err := c.DeleteLoadbalancer(ing)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) ensureIngress(oldIng, ing *nwv1.Ingress) error {
	klog.Infoln("------------ ensureIngress() ------------")
	lb, err := c.ensureCompareIngress(oldIng, ing)
	if err != nil {
		return err
	}
	vip := lb.Address
	newIng, err := c.updateIngressStatus(ing, vip)
	if err != nil {
		return err
	}
	c.recorder.Event(ing, apiv1.EventTypeNormal, "Updated", fmt.Sprintf("Successfully associated IP address %s to ingress %s", vip, newIng.Name))
	klog.Infoln(lb)
	return nil
}

// for vLB callback

func (c *Controller) getNodeMembersAddr() ([]string, error) {
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
	klog.Infoln("------------ updateIngressStatus() ------------")
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

///////////////////////////////////////////////////////////////////
//////////////                                       //////////////
//////////////                  VKS                  //////////////
//////////////                                       //////////////
///////////////////////////////////////////////////////////////////

type (
	ExtraInfo struct {
		ProjectID string
		UserID    int64
	}
)

func (c *Controller) Init() error {
	provider, err := vngcloud.NewClient(c.config.Global.IdentityURL)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("failed to init VNGCLOUD client")
	}
	err = vngcloud.Authenticate(provider, &oauth2.AuthOptions{
		ClientID:     c.config.Global.ClientID,
		ClientSecret: c.config.Global.ClientSecret,
		AuthOptionsBuilder: &tokens.AuthOptions{
			IdentityEndpoint: c.config.Global.IdentityURL,
		},
	})
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("failed to Authenticate VNGCLOUD client")
	}
	c.provider = provider

	vlbSC, err := vngcloud.NewServiceClient(
		"https://hcm-3.api.vngcloud.vn/vserver/vlb-gateway/v2",
		provider, "vlb-gateway")
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("failed to init VLB VNGCLOUD client")
	}
	c.vLBSC = vlbSC

	vserverSC, err := vngcloud.NewServiceClient(
		"https://hcm-3.api.vngcloud.vn/vserver/vserver-gateway/v2",
		provider, "vserver-gateway")
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("failed to init VSERVER VNGCLOUD client")
	}
	c.vServerSC = vserverSC

	c.setUpPortalInfo()
	c.api = API{
		VLBSC:     c.vLBSC,
		VServerSC: c.vServerSC,
		ProjectID: c.getProjectID(),
	}
	c.cluster, err = c.api.GetClusterInfo(c.config.ClusterID)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("failed to get cluster info")
	}

	return nil
}

func (c *Controller) setUpPortalInfo() {
	c.config.Metadata = getMetadataOption(metadata.Opts{})
	metadator := metadata.GetMetadataProvider(c.config.Metadata.SearchOrder)
	extraInfo, err := setupPortalInfo(
		c.provider,
		metadator,
		"https://hcm-3.api.vngcloud.vn/vserver/vserver-gateway/v1")
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Fatal("failed to setup portal info")
	}
	c.extraInfo = extraInfo
}

func (c *Controller) GetLoadbalancerIDByIngress(ing *nwv1.Ingress) (string, error) {
	klog.Infof("----------------- GetLoadbalancerIDByIngress(%s/%s) ------------------", ing.Namespace, ing.Name)
	lbsInSubnet, err := c.api.ListLBBySubnetID(c.getSubnetID())
	if err != nil {
		logrus.Errorf("error when list lb by subnet id: %v", err)
		return "", err
	}

	// check in annotation
	if lbID, ok := ing.Annotations[ServiceAnnotationLoadBalancerID]; ok {
		logrus.Infof("have annotation lbID: %s", lbID)
		for _, lb := range lbsInSubnet {
			if lb.UUID == lbID {
				logrus.Infof("found lbID: %s", lbID)
				return lb.UUID, nil
			}
		}
		logrus.Infof("have annotation but not found lbID: %s", lbID)
		return "", errors.ErrLoadBalancerIDNotFoundAnnotation
	}

	// check in list lb name
	lbName := GetResourceName(ing, c.getClusterID())
	for _, lb := range lbsInSubnet {
		if lb.Name == lbName {
			logrus.Infof("Found lb match Name: %s", lbName)
			return lb.UUID, nil
		}
	}
	logrus.Infof("Not found lb match Name: %s", lbName)
	return "", nil
}

func (c *Controller) DeleteLoadbalancer(ing *nwv1.Ingress) error {
	klog.Infof("----------------- DeleteLoadbalancer(%s/%s) ------------------", ing.Namespace, ing.Name)
	lbID, err := c.ensureLoadBalancer(ing)
	if err != nil {
		logrus.Errorln("error when ensure loadbalancer", err)
		return err
	}

	oldIngExpander, err := c.inspectIngress(ing)
	if err != nil {
		logrus.Errorln("error when inspect old ingress", err)
		return err
	}
	newIngExpander, err := c.inspectIngress(nil)
	if err != nil {
		logrus.Errorln("error when inspect new ingress", err)
		return err
	}

	_, err = c.actionCompareIngress(lbID, oldIngExpander, newIngExpander)
	if err != nil {
		logrus.Errorln("error when compare ingress", err)
		return err
	}
	return nil
}

// /////////////////////////////////// PRIVATE METHOD /////////////////////////////////////////
func (c *Controller) mapHostTLS(ing *nwv1.Ingress) (map[string]bool, []string) {
	m := make(map[string]bool)
	certArr := make([]string, 0)
	for _, tls := range ing.Spec.TLS {
		for _, host := range tls.Hosts {
			certArr = append(certArr, strings.TrimSpace(tls.SecretName))
			m[host] = true
		}
	}
	return m, certArr
}

// inspectCurrentLB inspects the current load balancer (LB) identified by lbID.
// It retrieves information about the listeners, pools, and policies associated with the LB.
// The function returns an IngressInspect struct containing the inspected data, or an error if the inspection fails.
func (c *Controller) inspectCurrentLB(lbID string) (*IngressInspect, error) {
	expectPolicyName := make([]*PolicyExpander, 0)
	expectPoolName := make([]*PoolExpander, 0)
	expectListenerName := make([]*ListenerExpander, 0)
	ingressInspect := &IngressInspect{}

	liss, err := c.api.ListListenerOfLB(lbID)
	if err != nil {
		logrus.Errorln("error when list listener of lb", err)
		return nil, err
	}
	for _, lis := range liss {
		listenerOpts := consts.OPT_LISTENER_HTTP_DEFAULT
		if lis.Protocol == "HTTPS" {
			listenerOpts = consts.OPT_LISTENER_HTTPS_DEFAULT
		}
		listenerOpts.DefaultPoolId = lis.DefaultPoolId
		expectListenerName = append(expectListenerName, &ListenerExpander{
			UUID:       lis.UUID,
			CreateOpts: listenerOpts,
		})
		ingressInspect.defaultPoolName = lis.DefaultPoolName
		ingressInspect.defaultPoolId = lis.DefaultPoolId
	}

	getPools, err := c.api.ListPoolOfLB(lbID)
	if err != nil {
		logrus.Errorln("error when list pool of lb", err)
		return nil, err
	}
	for _, p := range getPools {
		poolMembers := make([]*pool.Member, 0)
		for _, m := range p.Members {
			poolMembers = append(poolMembers, &pool.Member{
				IpAddress:   m.Address,
				Port:        m.ProtocolPort,
				Backup:      m.Backup,
				Weight:      m.Weight,
				Name:        m.Name,
				MonitorPort: m.MonitorPort,
			})
		}
		expectPoolName = append(expectPoolName, &PoolExpander{
			Name:    p.Name,
			UUID:    p.UUID,
			Members: poolMembers,
		})
	}

	for _, lis := range liss {
		pols, err := c.api.ListPolicyOfListener(lbID, lis.UUID)
		if err != nil {
			logrus.Errorln("error when list policy of listener", err)
			return nil, err
		}
		for _, pol := range pols {
			l7Rules := make([]policy.Rule, 0)
			for _, r := range pol.L7Rules {
				l7Rules = append(l7Rules, policy.Rule{
					CompareType: policy.PolicyOptsCompareTypeOpt(r.CompareType),
					RuleValue:   r.RuleValue,
					RuleType:    policy.PolicyOptsRuleTypeOpt(r.RuleType),
				})
			}
			expectPolicyName = append(expectPolicyName, &PolicyExpander{
				isInUse:          false,
				listenerID:       lis.UUID,
				UUID:             pol.UUID,
				Name:             pol.Name,
				RedirectPoolID:   pol.RedirectPoolID,
				RedirectPoolName: pol.RedirectPoolName,
				Action:           policy.PolicyOptsActionOpt(pol.Action),
				L7Rules:          l7Rules,
			})
		}
	}
	ingressInspect.PolicyExpander = expectPolicyName
	ingressInspect.PoolExpander = expectPoolName
	ingressInspect.ListenerExpander = expectListenerName
	return ingressInspect, nil
}

func (c *Controller) inspectIngress(ing *nwv1.Ingress) (*IngressInspect, error) {
	if ing == nil {
		return &IngressInspect{
			PolicyExpander:      make([]*PolicyExpander, 0),
			PoolExpander:        make([]*PoolExpander, 0),
			ListenerExpander:    make([]*ListenerExpander, 0),
			CertificateExpander: make([]*CertificateExpander, 0),
		}, nil
	}
	klog.Infof("----------------- inspectIngress(%s/%s) ------------------", ing.Namespace, ing.Name)
	lb_prefix_name := TrimString(GetResourceHashName(ing, c.getClusterID()), consts.DEFAULT_HASH_NAME_LENGTH)
	mapTLS, certArr := c.mapHostTLS(ing)

	expectPolicyName := make([]*PolicyExpander, 0)
	expectPoolName := make([]*PoolExpander, 0)
	expectListenerName := make([]*ListenerExpander, 0)
	expectCertificateName := make([]*CertificateExpander, 0)

	// convert to vngcloud certificate
	for _, tls := range ing.Spec.TLS {
		// check if certificate already exist
		secret, err := c.kubeClient.CoreV1().Secrets(ing.Namespace).Get(context.TODO(), tls.SecretName, apimetav1.GetOptions{})
		if err != nil {
			logrus.Errorf("error when get secret: %s in ns %s: %v", tls.SecretName, ing.Namespace, err)
			return nil, err
		}
		version := secret.ObjectMeta.ResourceVersion
		name := GetCertificateName(c.getClusterID(), ing.Namespace, tls.SecretName)
		secretName := tls.SecretName
		expectCertificateName = append(expectCertificateName, &CertificateExpander{
			Name:       name,
			Version:    version,
			SecretName: secretName,
			UUID:       "",
		})
	}

	GetPoolExpander := func(service *nwv1.IngressServiceBackend) (*PoolExpander, error) {
		serviceName := fmt.Sprintf("%s/%s", ing.ObjectMeta.Namespace, service.Name)
		poolName := GetPoolName(lb_prefix_name, serviceName, int(service.Port.Number))
		nodePort, err := c.getServiceNodePort(serviceName, service)
		if err != nil {
			klog.Errorf("error when get node port: %v", err)
			return nil, err
		}

		membersAddr, _ := c.getNodeMembersAddr()
		members := make([]*pool.Member, 0)
		for _, addr := range membersAddr {
			members = append(members, &pool.Member{
				IpAddress:   addr,
				Port:        nodePort,
				Backup:      false,
				Weight:      1,
				Name:        poolName,
				MonitorPort: nodePort,
			})
		}
		return &PoolExpander{
			Name:    poolName,
			UUID:    "",
			Members: members,
		}, nil
	}

	// check if have default pool
	ingressInspect := &IngressInspect{
		name:               ing.Name,
		namespace:          ing.Namespace,
		defaultPoolId:      "",
		defaultPoolName:    consts.DEFAULT_NAME_DEFAULT_POOL,
		defaultPoolMembers: make([]*pool.Member, 0),
	}
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		defaultPoolExpander, err := GetPoolExpander(ing.Spec.DefaultBackend.Service)
		if err != nil {
			logrus.Errorln("error when get default pool expander", err)
			return nil, err
		}
		ingressInspect.defaultPoolMembers = defaultPoolExpander.Members
	}
	// ensure http listener and https listener
	AddDefaultListener := func() {
		if len(certArr) > 0 {
			listenerHttpsOpts := consts.OPT_LISTENER_HTTPS_DEFAULT
			listenerHttpsOpts.CertificateAuthorities = &certArr
			listenerHttpsOpts.DefaultCertificateAuthority = &certArr[0]
			listenerHttpsOpts.ClientCertificate = consts.PointerOf[string]("")
			expectListenerName = append(expectListenerName, &ListenerExpander{
				CreateOpts: listenerHttpsOpts,
			})
		}

		expectListenerName = append(expectListenerName, &ListenerExpander{
			CreateOpts: consts.OPT_LISTENER_HTTP_DEFAULT,
		})
	}
	AddDefaultListener()

	for ruleIndex, rule := range ing.Spec.Rules {
		_, isHttpsListener := mapTLS[rule.Host]

		for pathIndex, path := range rule.HTTP.Paths {
			policyName := GetPolicyName(lb_prefix_name, isHttpsListener, ruleIndex, pathIndex)

			poolExpander, err := GetPoolExpander(path.Backend.Service)
			if err != nil {
				logrus.Errorln("error when get pool expander", err)
				return nil, err
			}
			expectPoolName = append(expectPoolName, poolExpander)

			// ensure policy
			newRules := []policy.Rule{
				{
					RuleType:    policy.PolicyOptsRuleTypeOptPATH,
					CompareType: policy.PolicyOptsCompareTypeOptEQUALS,
					RuleValue:   path.Path,
				},
			}
			if rule.Host != "" {
				newRules = append(newRules, policy.Rule{
					RuleType:    policy.PolicyOptsRuleTypeOptHOSTNAME,
					CompareType: policy.PolicyOptsCompareTypeOptEQUALS,
					RuleValue:   rule.Host,
				})
			}
			expectPolicyName = append(expectPolicyName, &PolicyExpander{
				isHttpsListener:  isHttpsListener,
				isInUse:          false,
				UUID:             "",
				Name:             policyName,
				RedirectPoolID:   "",
				RedirectPoolName: poolExpander.Name,
				Action:           policy.PolicyOptsActionOptREDIRECTTOPOOL,
				L7Rules:          newRules,
			})
		}
	}
	ingressInspect.PolicyExpander = expectPolicyName
	ingressInspect.PoolExpander = expectPoolName
	ingressInspect.ListenerExpander = expectListenerName
	ingressInspect.CertificateExpander = expectCertificateName
	return ingressInspect, nil
}

func (c *Controller) ensureCompareIngress(oldIng, ing *nwv1.Ingress) (*lObjects.LoadBalancer, error) {
	klog.Infof("----------------- ensureCompareIngress(%s/%s) ------------------", ing.Namespace, ing.Name)
	lbID, err := c.ensureLoadBalancer(ing)
	if err != nil {
		logrus.Errorln("error when ensure loadbalancer", err)
		return nil, err
	}

	oldIngExpander, err := c.inspectIngress(oldIng)
	if err != nil {
		logrus.Errorln("error when inspect old ingress", err)
		return nil, err
	}
	newIngExpander, err := c.inspectIngress(ing)
	if err != nil {
		logrus.Errorln("error when inspect new ingress", err)
		return nil, err
	}

	lb, err := c.actionCompareIngress(lbID, oldIngExpander, newIngExpander)
	if err != nil {
		logrus.Errorln("error when compare ingress", err)
		return nil, err
	}
	return lb, nil
}

// find or create lb
func (c *Controller) ensureLoadBalancer(ing *nwv1.Ingress) (string, error) {
	lbID, err := c.GetLoadbalancerIDByIngress(ing)
	if err != nil {
		if err == errors.ErrLoadBalancerIDNotFoundAnnotation {
			return "", err
		}

		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Error("error not handled when list loadbalancer by subnet id")
		return "", fmt.Errorf("error not handled when list loadbalancer by subnet id: %v", err)
	}

	if lbID == "" {
		klog.Infof("--------------- create new lb for ingress %s/%s -------------------", ing.Namespace, ing.Name)
		lbName := GetResourceName(ing, c.getClusterID())
		packageID := getStringFromIngressAnnotation(ing, ServiceAnnotationPackageID, consts.DEFAULT_PACKAGE_ID)

		lb, err := c.api.CreateLB(lbName, packageID, c.getSubnetID(),
			loadbalancer.CreateOptsSchemeOptInternet,
			loadbalancer.CreateOptsTypeOptLayer7)
		if err != nil {
			klog.Errorf("error when create new lb: %v", err)
			return "", err
		}
		lbID = lb.UUID
	}
	return lbID, nil
}

func (c *Controller) actionCompareIngress(lbID string, oldIngExpander, newIngExpander *IngressInspect) (*lObjects.LoadBalancer, error) {
	lb := c.api.WaitForLBActive(lbID)

	curLBExpander, err := c.inspectCurrentLB(lbID)
	if err != nil {
		logrus.Errorln("error when inspect current lb", err)
		return nil, err
	}

	MapIDExpander(oldIngExpander, curLBExpander) // ..........................................
	for _, cert := range oldIngExpander.CertificateExpander {
		c.SecretTrackers.RemoveSecretTracker(oldIngExpander.namespace, cert.SecretName)
	}
	for _, cert := range newIngExpander.CertificateExpander {
		c.SecretTrackers.AddSecretTracker(newIngExpander.namespace, cert.SecretName, cert.UUID, cert.Version)
	}
	// ensure certificate
	EnsureCertificate := func(ing *IngressInspect) error {
		lCert, _ := c.api.ListCertificate()
		for _, cert := range ing.CertificateExpander {
			// check if certificate already exist
			for _, lc := range lCert {
				if lc.Name == cert.Name+cert.Version {
					cert.UUID = lc.UUID
					break
				}
			}
			if cert.UUID != "" {
				continue
			}
			importOpts, err := c.toVngCloudCertificate(cert.SecretName, ing.namespace, cert.Name+cert.Version)
			if err != nil {
				logrus.Errorln("error when toVngCloudCertificate", err)
				return err
			}
			newCert, err := c.api.ImportCertificate(importOpts)
			if err != nil {
				logrus.Errorln("error when import certificate", err)
				return err
			}
			cert.UUID = newCert.UUID
		}
		return nil
	}
	err = EnsureCertificate(newIngExpander)
	if err != nil {
		logrus.Errorln("error when ensure certificate", err)
		return nil, err
	}

	// ensure default pool
	defaultPool, err := c.ensurePool(lb.UUID, consts.DEFAULT_NAME_DEFAULT_POOL)
	if err != nil {
		logrus.Errorln("error when ensure default pool", err)
		return nil, err
	}
	// ensure default pool member
	_, err = c.ensureDefaultPoolMember(lb.UUID, defaultPool.UUID, oldIngExpander.defaultPoolMembers, newIngExpander.defaultPoolMembers)
	if err != nil {
		logrus.Errorln("error when ensure default pool member", err)
		return nil, err
	}
	// ensure all from newIngExpander
	mapPoolNameIndex := make(map[string]int)
	for poolIndex, ipool := range newIngExpander.PoolExpander {
		newPool, err := c.ensurePool(lb.UUID, ipool.Name)
		if err != nil {
			logrus.Errorln("error when ensure pool", err)
			return nil, err
		}
		ipool.UUID = newPool.UUID
		mapPoolNameIndex[ipool.Name] = poolIndex
		_, err = c.ensurePoolMember(lb.UUID, newPool.UUID, ipool.Members)
		if err != nil {
			logrus.Errorln("error when ensure pool member", err)
			return nil, err
		}
	}
	mapListenerNameIndex := make(map[string]int)
	for listenerIndex, ilistener := range newIngExpander.ListenerExpander {
		ilistener.CreateOpts.DefaultPoolId = defaultPool.UUID
		// change cert name by uuid
		if ilistener.CreateOpts.ListenerProtocol == consts.OPT_LISTENER_HTTPS_DEFAULT.ListenerProtocol {
			mapCertNameUUID := make(map[string]string)
			for _, cert := range newIngExpander.CertificateExpander {
				mapCertNameUUID[cert.SecretName] = cert.UUID
			}
			uuidArr := []string{}
			for _, certName := range *ilistener.CreateOpts.CertificateAuthorities {
				uuidArr = append(uuidArr, mapCertNameUUID[certName])
			}
			ilistener.CreateOpts.CertificateAuthorities = &uuidArr
			ilistener.CreateOpts.DefaultCertificateAuthority = &uuidArr[0]
		}
		lis, err := c.ensureListener(lb.UUID, ilistener.CreateOpts.ListenerName, ilistener.CreateOpts)
		if err != nil {
			logrus.Errorln("error when ensure listener:", ilistener.CreateOpts.ListenerName, err)
			return nil, err
		}
		ilistener.UUID = lis.UUID
		mapListenerNameIndex[ilistener.CreateOpts.ListenerName] = listenerIndex
	}
	logrus.Infof("newIngExpander: %v", newIngExpander.PolicyExpander)
	for _, ipolicy := range newIngExpander.PolicyExpander {
		// get pool name from redirect pool name
		poolIndex, isHave := mapPoolNameIndex[ipolicy.RedirectPoolName]
		if !isHave {
			logrus.Errorf(".........pool not found in policy: %v", ipolicy.RedirectPoolName)
			return nil, err
		}
		poolID := newIngExpander.PoolExpander[poolIndex].UUID
		listenerName := consts.DEFAULT_HTTP_LISTENER_NAME
		if ipolicy.isHttpsListener {
			listenerName = consts.DEFAULT_HTTPS_LISTENER_NAME
		}
		listenerIndex, isHave := mapListenerNameIndex[listenerName]
		if !isHave {
			logrus.Errorf("listener index not found: %v", listenerName)
			return nil, err
		}
		listenerID := newIngExpander.ListenerExpander[listenerIndex].UUID
		if listenerID == "" {
			logrus.Errorf("listenerID not found: %v", listenerName)
			return nil, err
		}

		policyOpts := &policy.CreateOptsBuilder{
			Name:           ipolicy.Name,
			Action:         ipolicy.Action,
			RedirectPoolID: poolID,
			Rules:          ipolicy.L7Rules,
		}
		_, err := c.ensurePolicy(lb.UUID, listenerID, ipolicy.Name, policyOpts)
		if err != nil {
			logrus.Errorln("error when ensure policy", err)
			return nil, err
		}
	}

	// delete redundant policy and pool if in oldIng
	// with id from curLBExpander
	klog.Infof("*************** DELETE REDUNDANT POLICY AND POOL *****************")
	policyWillUse := make(map[string]int)
	for policyIndex, pol := range newIngExpander.PolicyExpander {
		policyWillUse[pol.Name] = policyIndex
	}
	logrus.Infof("policyWillUse: %v", policyWillUse)
	for _, oldIngPolicy := range oldIngExpander.PolicyExpander {
		_, isPolicyWillUse := policyWillUse[oldIngPolicy.Name]
		if !isPolicyWillUse {
			logrus.Warnf("policy not in use: %v, delete", oldIngPolicy.Name)
			_, err := c.deletePolicy(lb.UUID, oldIngPolicy.listenerID, oldIngPolicy.Name)
			if err != nil {
				logrus.Errorln("error when ensure policy", err)
				// maybe it's already deleted
				// return nil, err
			}
		} else {
			logrus.Infof("policy in use: %v, not delete", oldIngPolicy.Name)
		}
	}

	poolWillUse := make(map[string]bool)
	for _, pool := range newIngExpander.PoolExpander {
		poolWillUse[pool.Name] = true
	}
	for _, oldIngPool := range oldIngExpander.PoolExpander {
		_, isPoolWillUse := poolWillUse[oldIngPool.Name]
		if !isPoolWillUse && oldIngPool.Name != consts.DEFAULT_NAME_DEFAULT_POOL {
			logrus.Warnf("pool not in use: %v, delete", oldIngPool.Name)
			_, err := c.deletePool(lb.UUID, oldIngPool.Name)
			if err != nil {
				logrus.Errorln("error when ensure pool", err)
				// maybe it's already deleted
				// return nil, err
			}
		} else {
			logrus.Infof("pool in use: %v, not delete", oldIngPool.Name)
		}
	}

	// ensure certificate
	DeleteRedundantCertificate := func(ing *IngressInspect) error {
		lCert, _ := c.api.ListCertificate()
		for _, lc := range lCert {
			for _, cert := range ing.CertificateExpander {
				if strings.HasPrefix(lc.Name, cert.Name) && !lc.InUse {
					err := c.api.DeleteCertificate(lc.UUID)
					if err != nil {
						logrus.Errorln("error when delete certificate:", lc.UUID, err)
					}
				}
			}
		}
		return nil
	}
	DeleteRedundantCertificate(oldIngExpander)
	DeleteRedundantCertificate(newIngExpander)
	return lb, nil
}

func (c *Controller) ensurePool(lbID, poolName string) (*lObjects.Pool, error) {
	klog.Infof("------------ ensurePool: %s", poolName)
	pool, err := c.api.FindPoolByName(lbID, poolName)
	if err != nil {
		if err == errors.ErrNotFound {
			newPoolOpts := consts.OPT_POOL_DEFAULT
			newPoolOpts.PoolName = poolName
			newPool, err := c.api.CreatePool(lbID, &newPoolOpts)
			if err != nil {
				logrus.Errorln("error when create new pool", err)
				return nil, err
			}
			pool = newPool
		} else {
			logrus.Errorln("error when find pool", err)
			return nil, err
		}
	}
	c.api.WaitForLBActive(lbID)
	return pool, nil
}

func (c *Controller) deletePool(lbID, poolName string) (*lObjects.Pool, error) {
	klog.Infof("------------ deletePool: %s", poolName)
	pool, err := c.api.FindPoolByName(lbID, poolName)
	if err != nil {
		if err == errors.ErrNotFound {
			logrus.Infof("pool not found: %s, maybe deleted", poolName)
			return nil, nil
		} else {
			logrus.Errorln("error when find pool", err)
			return nil, err
		}
	}

	if pool.Name == consts.DEFAULT_NAME_DEFAULT_POOL {
		logrus.Info("pool is default pool, not delete")
		return nil, nil
	}
	err = c.api.DeletePool(lbID, pool.UUID)
	if err != nil {
		logrus.Errorln("error when delete pool", err)
		return nil, err
	}

	c.api.WaitForLBActive(lbID)
	return pool, nil
}

func (c *Controller) ensureDefaultPoolMember(lbID, poolID string, oldMembers, newMembers []*pool.Member) (*lObjects.Pool, error) {
	klog.Infof("------------ ensureDefaultPoolMember: %s", poolID)
	memsGet, err := c.api.GetMemberPool(lbID, poolID)
	memsGetConvert := ConvertObjectToPoolMemberArray(memsGet)
	if err != nil {
		logrus.Errorln("error when get pool members", err)
		return nil, err
	}
	getRedundant := func(old, new []*pool.Member) []*pool.Member {
		redundant := make([]*pool.Member, 0)
		for _, o := range old {
			isHave := false
			for _, n := range new {
				if o.IpAddress == n.IpAddress &&
					o.MonitorPort == n.MonitorPort &&
					o.Weight == n.Weight &&
					o.Backup == n.Backup &&
					// o.Name == n.Name &&
					o.Port == n.Port {
					isHave = true
					break
				}
			}
			if !isHave {
				redundant = append(redundant, o)
			}
		}
		return redundant
	}
	needDelete := getRedundant(oldMembers, newMembers)
	needCreate := newMembers // need ensure

	klog.Infof("memGets: %v", memsGet)
	for _, m := range memsGetConvert {
		klog.Infof("-----: %s, %s, %d", m.Name, m.IpAddress, m.Port)
	}
	klog.Infof("needDelete: %v", needDelete)
	for _, m := range needDelete {
		klog.Infof("-----: %s, %s, %d", m.Name, m.IpAddress, m.Port)
	}
	klog.Infof("needCreate: %v", needCreate)
	for _, m := range needCreate {
		klog.Infof("-----: %s, %s, %d", m.Name, m.IpAddress, m.Port)
	}

	updateMember := make([]*pool.Member, 0)
	for _, m := range memsGetConvert {
		// remove all member in needCreate and add later (maybe member is scale down, then redundant)
		isAddLater := false
		for _, nc := range needCreate {
			if strings.HasPrefix(m.Name, nc.Name) {
				isAddLater = true
				break
			}
		}
		if isAddLater {
			continue
		}

		if !CheckIfPoolMemberExist(needDelete, m) {
			updateMember = append(updateMember, m)
		}
	}
	for _, m := range needCreate {
		if !CheckIfPoolMemberExist(updateMember, m) {
			updateMember = append(updateMember, m)
		}
	}

	if ComparePoolMembers(updateMember, memsGetConvert) {
		logrus.Infof("no need update default pool member")
		return nil, nil
	}

	c.isUpdateDefaultPool = true
	err = c.api.UpdatePoolMember(lbID, poolID, updateMember)
	if err != nil {
		logrus.Errorln("error when update pool members", err)
		return nil, err
	}
	c.api.WaitForLBActive(lbID)
	return nil, nil
}

func (c *Controller) ensurePoolMember(lbID, poolID string, members []*pool.Member) (*lObjects.Pool, error) {
	klog.Infof("------------ ensurePoolMember: %s", poolID)
	memsGet, err := c.api.GetMemberPool(lbID, poolID)
	memsGetConvert := ConvertObjectToPoolMemberArray(memsGet)
	if err != nil {
		logrus.Errorln("error when get pool members", err)
		return nil, err
	}
	if !ComparePoolMembers(members, memsGetConvert) {
		err := c.api.UpdatePoolMember(lbID, poolID, members)
		if err != nil {
			logrus.Errorln("error when update pool members", err)
			return nil, err
		}
	}
	c.api.WaitForLBActive(lbID)
	return nil, nil
}

func (c *Controller) ensureListener(lbID, lisName string, listenerOpts listener.CreateOpts) (*lObjects.Listener, error) {
	klog.Infof("------------ ensureListener ----------")
	lis, err := c.api.FindListenerByPort(lbID, listenerOpts.ListenerProtocolPort)
	if err != nil {
		if err == errors.ErrNotFound {
			// create listener point to default pool
			listenerOpts.ListenerName = lisName
			_, err := c.api.CreateListener(lbID, &listenerOpts)
			if err != nil {
				logrus.Errorln("error when create listener", err)
				return nil, err
			}
			c.api.WaitForLBActive(lbID)
			lis, err = c.api.FindListenerByPort(lbID, listenerOpts.ListenerProtocolPort)
			if err != nil {
				logrus.Errorln("error when find listener", err)
				return nil, err
			}
		} else {
			logrus.Errorln("error when find listener", err)
			return nil, err
		}
	}

	updateOpts := &listener.UpdateOpts{
		AllowedCidrs:                lis.AllowedCidrs,
		DefaultPoolId:               lis.DefaultPoolId,
		TimeoutClient:               lis.TimeoutClient,
		TimeoutConnection:           lis.TimeoutConnection,
		TimeoutMember:               lis.TimeoutMember,
		Headers:                     lis.Headers,
		DefaultCertificateAuthority: lis.DefaultCertificateAuthority,
		ClientCertificate:           lis.ClientCertificateAuthentication,
	}
	isUpdate := false

	if lis.DefaultPoolId != listenerOpts.DefaultPoolId && listenerOpts.DefaultPoolId != "" {
		updateOpts.DefaultPoolId = listenerOpts.DefaultPoolId
		isUpdate = true
		klog.Infof("listener need update default pool id: %s", listenerOpts.DefaultPoolId)
	}

	if listenerOpts.DefaultCertificateAuthority != nil && (lis.DefaultCertificateAuthority == nil || *(lis.DefaultCertificateAuthority) != *(listenerOpts.DefaultCertificateAuthority)) {
		updateOpts.DefaultCertificateAuthority = listenerOpts.DefaultCertificateAuthority
		isUpdate = true
		klog.Infof("listener need update default certificate authority: %s", *listenerOpts.DefaultCertificateAuthority)
	}

	// update cert SNI here .......................................................

	if isUpdate {
		err := c.api.UpdateListener(lbID, lis.UUID, updateOpts)
		if err != nil {
			logrus.Error("error when update listener: ", err)
			return nil, err
		}
	}

	c.api.WaitForLBActive(lbID)
	return lis, nil
}

func (c *Controller) ensurePolicy(lbID, listenerID, policyName string, policyOpt *policy.CreateOptsBuilder) (*lObjects.Policy, error) {
	klog.Infof("------------ ensurePolicy: %s", policyName)
	pol, err := c.api.FindPolicyByName(lbID, listenerID, policyName)
	if err != nil {
		if err == errors.ErrNotFound {
			newPolicy, err := c.api.CreatePolicy(lbID, listenerID, policyOpt)
			if err != nil {
				logrus.Errorln("error when create policy", err)
				return nil, err
			}
			pol = newPolicy
		} else {
			logrus.Errorln("error when find policy", err)
			return nil, err
		}
	}
	// get policy and update policy
	newpolicy, err := c.api.GetPolicy(lbID, listenerID, pol.UUID)
	if err != nil {
		logrus.Errorln("error when get policy", err)
		return nil, err
	}
	comparePolicy := func(p2 *lObjects.Policy) bool {
		if string(policyOpt.Action) != p2.Action ||
			policyOpt.RedirectPoolID != p2.RedirectPoolID ||
			policyOpt.Name != p2.Name {
			return false
		}
		if len(policyOpt.Rules) != len(p2.L7Rules) {
			return false
		}

		checkIfExist := func(rules []*lObjects.L7Rule, rule policy.Rule) bool {
			for _, r := range rules {
				if r.CompareType == string(rule.CompareType) &&
					r.RuleType == string(rule.RuleType) &&
					r.RuleValue == rule.RuleValue {
					return true
				}
			}
			return false
		}
		for _, rule := range policyOpt.Rules {
			if !checkIfExist(p2.L7Rules, rule) {
				logrus.Infof("rule not exist: %v", rule)
				return false
			}
		}
		return true
	}
	if !comparePolicy(newpolicy) {
		updateOpts := &policy.UpdateOptsBuilder{
			Action:         policyOpt.Action,
			RedirectPoolID: policyOpt.RedirectPoolID,
			Rules:          policyOpt.Rules,
		}
		err := c.api.UpdatePolicy(lbID, listenerID, pol.UUID, updateOpts)
		if err != nil {
			logrus.Errorln("error when update policy", err)
			return nil, err
		}
	}

	c.api.WaitForLBActive(lbID)
	return pol, nil
}

func (c *Controller) deletePolicy(lbID, listenerID, policyName string) (*lObjects.Policy, error) {
	klog.Infof("------------ deletePolicy: %s", policyName)
	pol, err := c.api.FindPolicyByName(lbID, listenerID, policyName)
	if err != nil {
		if err == errors.ErrNotFound {
			logrus.Infof("policy not found: %s, maybe deleted", policyName)
			return nil, nil
		} else {
			logrus.Errorln("error when find policy", err)
			return nil, err
		}
	}
	err = c.api.DeletePolicy(lbID, listenerID, pol.UUID)
	if err != nil {
		logrus.Errorln("error when delete policy", err)
		return nil, err
	}
	c.api.WaitForLBActive(lbID)
	return pol, nil
}

func (c *Controller) getProjectID() string {
	return c.extraInfo.ProjectID
}

func (c *Controller) getClusterID() string {
	return c.config.ClusterID
}

func (c *Controller) getSubnetID() string {
	return c.cluster.SubnetID
}

func (c *Controller) toVngCloudCertificate(secretName string, namespace string, generateName string) (*certificates.ImportOpts, error) {
	secret, err := c.kubeClient.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, apimetav1.GetOptions{})
	if err != nil {
		logrus.Errorln("error when get secret", err)
		return nil, err
	}

	var keyDecode []byte
	if keyBytes, isPresent := secret.Data[IngressSecretKeyName]; isPresent {
		keyDecode = keyBytes
		if err != nil {
			logrus.Errorln("error when decode key", err, string(keyBytes))
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("%s key doesn't exist in the secret %s", IngressSecretKeyName, secretName)
	}

	var certDecode []byte
	if certBytes, isPresent := secret.Data[IngressSecretCertName]; isPresent {
		certDecode = certBytes
		if err != nil {
			logrus.Errorln("error when decode cert", err, string(certBytes))
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("%s key doesn't exist in the secret %s", IngressSecretCertName, secretName)
	}

	return &certificates.ImportOpts{
		Name:             generateName,
		Type:             certificates.ImportOptsTypeOptTLS,
		Certificate:      string(certDecode),
		PrivateKey:       consts.PointerOf[string](string(keyDecode)),
		CertificateChain: consts.PointerOf[string](""),
		Passphrase:       consts.PointerOf[string](""),
	}, nil
}
