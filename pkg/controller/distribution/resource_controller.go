package distribution

import (
	"context"
	"fmt"
	"github.com/Gentleelephant/custom-controller/pkg/apis/cluster/v1alpha1"
	"github.com/Gentleelephant/custom-controller/pkg/apis/distribution"
	distributionv1 "github.com/Gentleelephant/custom-controller/pkg/apis/distribution/v1alpha1"
	clientset "github.com/Gentleelephant/custom-controller/pkg/client/clientset/versioned"
	clusterinformers "github.com/Gentleelephant/custom-controller/pkg/client/informers/externalversions/cluster/v1alpha1"
	distributioninformers "github.com/Gentleelephant/custom-controller/pkg/client/informers/externalversions/distribution/v1alpha1"
	clisters "github.com/Gentleelephant/custom-controller/pkg/client/listers/cluster/v1alpha1"
	listers "github.com/Gentleelephant/custom-controller/pkg/client/listers/distribution/v1alpha1"
	"github.com/Gentleelephant/custom-controller/pkg/constant"
	"github.com/Gentleelephant/custom-controller/pkg/utils"
	"github.com/Gentleelephant/custom-controller/pkg/utils/custom"
	"github.com/Gentleelephant/custom-controller/pkg/utils/keys"
	"github.com/duke-git/lancet/v2/cryptor"
	"github.com/duke-git/lancet/v2/maputil"
	"github.com/duke-git/lancet/v2/slice"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"reflect"
	"strings"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	ChannelSize = 256

	ControllerName = "distribution-controller"

	EventCreate = "ADD"

	EventUpdate = "UPDATE"

	EventDelete = "DELETE"
)

type EventType string

type EventObject struct {
	EventType EventType

	Old *distributionv1.ResourceDistribution

	New *distributionv1.ResourceDistribution
}

//+kubebuilder:rbac:groups=distribution.kubesphere.io,resources=resourcedistributions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=distribution.kubesphere.io,resources=resourcedistributions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=distribution.kubesphere.io,resources=resourcedistributions/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch;update;

type DeleteObject struct {
	Selector *metav1.LabelSelector

	WorkloadName []string
}

type BindObject struct {
	Obj interface{}

	ResourceDistributions []string
}

type DistributionController struct {
	Client                client.Client
	Kubeclientset         kubernetes.Interface
	Clientset             clientset.Interface
	RdLister              listers.ResourceDistributionLister
	ClusterLister         clisters.ClusterLister
	Scheme                *runtime.Scheme
	RESTMapper            meta.RESTMapper
	DynamicClient         dynamic.Interface
	DiscoverClient        discovery.DiscoveryClient
	KeyStore              *KeyStore
	RuleStore             *ParsedOverrideRulesStore
	RdSynced              toolscache.InformerSynced
	ClusterSynced         toolscache.InformerSynced
	Workqueue             workqueue.RateLimitingInterface
	Recorder              record.EventRecorder
	InformerManager       custom.ClusterInformerManager
	SkippedResourceConfig *utils.SkippedConfig
	del                   chan *DeleteObject
	cre                   chan *BindObject
	EventHandler          toolscache.ResourceEventHandler
	stopCh                <-chan struct{}
	mu                    sync.RWMutex
}

func NewDistributionController(ctx context.Context,
	client client.Client,
	kubeclientset kubernetes.Interface,
	clientset clientset.Interface,
	schema *runtime.Scheme,
	restmapper meta.RESTMapper,
	dynamicClient dynamic.Interface,
	discoverClient discovery.DiscoveryClient,
	rdinformer distributioninformers.ResourceDistributionInformer,
	cinformer clusterinformers.ClusterInformer,
) *DistributionController {
	logger := klog.FromContext(ctx)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	_ = eventBroadcaster.NewRecorder(schema, corev1.EventSource{Component: ControllerName})

	controller := &DistributionController{
		Client:         client,
		Kubeclientset:  kubeclientset,
		Clientset:      clientset,
		Scheme:         schema,
		RESTMapper:     restmapper,
		DynamicClient:  dynamicClient,
		DiscoverClient: discoverClient,
		RdLister:       rdinformer.Lister(),
		RdSynced:       rdinformer.Informer().HasSynced,
		ClusterLister:  cinformer.Lister(),
		ClusterSynced:  cinformer.Informer().HasSynced,
		Workqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "distribution"),
		//Recorder:       Recorder,
	}

	controller.KeyStore = NewKeyStore()
	controller.RuleStore = NewParsedOverrideRulesStore()
	controller.SkippedResourceConfig = utils.NewSkippedResourceConfig()
	controller.InformerManager = custom.NewClusterInformerManager(dynamicClient, 0, ctx.Done())
	controller.del = make(chan *DeleteObject, ChannelSize)
	controller.cre = make(chan *BindObject, ChannelSize)

	logger.Info("Setting up ResourceDistribution event handlers")
	// Set up an event handler for when ResourceDistribution resources change
	rdinformer.Informer().AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// TODO: 创建对应资源的informer
			controller.OnRDAdd(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			controller.OnRDUpdate(old, new)
		},
		DeleteFunc: func(obj interface{}) {
			controller.OnRDDelete(obj)
		},
	})

	// Set up an event handler for when Cluster resources change
	//TODO: 在这里需要找到该Cluster被哪些ResourceDistribution所引用，然后将找到的ResourceDistribution入队列
	cinformer.Informer().AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// 如果是主集群，跳过
			clsuter, ok := obj.(*v1alpha1.Cluster)
			if !ok {
				klog.Error("Cluster add error, obj is not *v1alpha1.Cluster")
			}
			labels := clsuter.Labels
			_, ok = labels[constant.HostCluster]
			if ok {
				return
			}
			klog.Info("Cluster add")
			// TODO:cluster添加需要通知到ResourceDistribution
			// 直接暴力一点，将所有的ResourceDistribution都入队列！
		},
		UpdateFunc: func(old, new interface{}) {
			// 暂时先不管
			// TODO:cluster更新label要通知到ResourceDistribution
		},
	})
	return controller
}

func (c *DistributionController) applyRules() {

	go func() {
		for {
			select {
			case obj := <-c.cre:
				// TODO: 需要将对应的关联资源存储起来，后面如果rd有变化，这部分资源需要重新入队列
				c.createOrUpdateWorkload(obj)
			}
		}
	}()

}

func (c *DistributionController) deleteObject() {
	go func() {
		for {
			select {
			case obj := <-c.del:
				c.deleteWorkload(obj)
			}
		}
	}()
}

func (c *DistributionController) createOrUpdateWorkload(obj *BindObject) {

	key, err := keys.ClusterWideKeyFunc(obj.Obj)
	if err != nil {
		klog.Error("create error:", err)
	}
	unstruct, err := utils.ToUnstructured(obj.Obj)
	if err != nil {
		klog.Error("transform error:", err)
		return
	}
	labels := unstruct.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[constant.DistributionManaged] = "true"
	unstruct.SetLabels(labels)
	for _, s := range obj.ResourceDistributions {
		rules, b := c.RuleStore.Get(s)
		if !b {
			continue
		}
		for _, v := range rules {
			deepcopy := unstruct.DeepCopy()
			// 应用规则
			err := ApplyJSONPatchs(deepcopy, v.OverrideOptions)
			if err != nil {
				klog.Error("apply error:", err)
				return
			}
			wname := getWorkloadName(&key, v.Id, s)
			// 加上wokload的label
			deepcopy.GetLabels()[constant.WorkloadName] = wname
			rd, err := c.RdLister.Get(s)
			if err != nil {
				klog.Error("get rd error:", err)
				continue
			}
			work := c.creWork(wname, v.Id, v.Clusters, rd)
			json, err := deepcopy.MarshalJSON()
			if err != nil {
				klog.Error("marshal error:", err)
				continue
			}
			work.Spec.Manifest.Raw = json
			temp := &distributionv1.Workload{}
			err = c.Client.Get(context.Background(), client.ObjectKey{Name: work.Name}, temp)
			if err != nil {
				if errors.IsNotFound(err) {
					err = c.Client.Create(context.Background(), work)
					if err != nil {
						klog.Error("create error:", err)
					}
				}
			} else {
				// 更新
				klog.Info("update workload")
				work.SetResourceVersion(temp.GetResourceVersion())
				err = c.Client.Update(context.Background(), work)
				if err != nil {
					klog.Error("update error:", err)
				}
			}
		}
	}

}

func NamespaceKeyFunc(obj interface{}) (string, error) {
	var key string
	var err error
	if key, err = toolscache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return "", err
	}
	return key, nil
}

func (c *DistributionController) enqueue(obj interface{}) {
	c.Workqueue.Add(obj)
}

func (c *DistributionController) Start(ctx context.Context) error {
	err := c.Run(ctx, 4)
	if err != nil {
		return err
	}
	return nil
}

func (c *DistributionController) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.Workqueue.ShutDown()
	c.stopCh = ctx.Done()
	logger := klog.FromContext(ctx)
	// Start the informer factories to begin populating the informer caches
	logger.Info("Starting ResourceDistribution controller")

	// Wait for the caches to be synced before starting workers
	logger.Info("Waiting for informer caches to sync")
	if ok := toolscache.WaitForCacheSync(ctx.Done(), c.RdSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	if ok := toolscache.WaitForCacheSync(ctx.Done(), c.ClusterSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logger.Info("Starting workers", "count", workers)
	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}
	logger.Info("Started workers")

	c.EventHandler = NewFilteringHandlerOnAllEvents(c.EventFilter, c.OnAdd, c.OnUpdate, c.OnDelete)
	//go c.discoverResources(30 * time.Second)

	// 启动applyRules
	go c.applyRules()
	go c.deleteObject()

	<-ctx.Done()
	logger.Info("Shutting down workers")

	return nil
}

//func (c *DistributionController) discoverResources(period time.Duration) {
//	wait.Until(func() {
//		klog.Infof("===>Start to discover resources")
//		newResources := GetDeletableResources(&c.DiscoverClient)
//		for r := range newResources {
//			if c.InformerManager.IsHandlerExist(r, c.EventHandler) || c.gvrDisabled(r) {
//				continue
//			}
//			klog.Info("discover resource", "gvr", r.String())
//			c.InformerManager.ForResource(r, c.EventHandler)
//		}
//		c.InformerManager.Start()
//	}, period, c.stopCh)
//}

// gvrDisabled returns whether GroupVersionResource is disabled.
func (c *DistributionController) gvrDisabled(gvr schema.GroupVersionResource) bool {
	if c.SkippedResourceConfig == nil {
		return false
	}

	if c.SkippedResourceConfig.GroupVersionDisabled(gvr.GroupVersion()) {
		return true
	}
	if c.SkippedResourceConfig.GroupDisabled(gvr.Group) {
		return true
	}

	if !c.allowGvr(gvr) {
		return true
	}

	gvks, err := c.RESTMapper.KindsFor(gvr)
	if err != nil {
		klog.Errorf("gvr(%s) transform failed: %v", gvr.String(), err)
		return false
	}

	for _, gvk := range gvks {
		if c.SkippedResourceConfig.GroupVersionKindDisabled(gvk) {
			return true
		}
	}

	return false
}

func (c *DistributionController) allowGvr(gvr schema.GroupVersionResource) bool {
	testgvr := []schema.GroupVersionResource{
		{Group: "apps", Version: "v1alpha1", Resource: "deployments"},
	}
	for _, g := range testgvr {
		if g == gvr {
			return true
		}
	}
	return false
}

func (c *DistributionController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *DistributionController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.Workqueue.Get()
	//logger := klog.FromContext(ctx)

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.Workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the Workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the Workqueue and attempted again after a back-off
		// period.
		defer c.Workqueue.Done(obj)
		var key *EventObject
		var ok bool
		// We expect strings to come off the Workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// Workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// Workqueue.
		if key, ok = obj.(*EventObject); !ok {
			// As the item in the Workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.Workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected EventObj in Workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(ctx, key); err != nil {
			// Put the item back on the Workqueue to handle any transient errors.
			c.Workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%v': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.Workqueue.Forget(obj)
		//logger.Info("Successfully synced", "resourceName", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *DistributionController) syncHandler(ctx context.Context, eventObj *EventObject) error {
	// Convert the namespace/name string into a distinct namespace and name
	//logger := klog.LoggerWithValues(klog.FromContext(ctx), "resourceName")
	//rd := eventObj.New

	//if rd.ObjectMeta.DeletionTimestamp.IsZero() {
	//	if !controllerutil.ContainsFinalizer(rd, constant.Finalizer) {
	//		if err := c.updateExternalResources(context.Background(), rd); err != nil {
	//			logger.Error(err, "updateExternalResources error")
	//			return err
	//		}
	//	}
	//} else {
	//	// The object is being deleted
	//	if controllerutil.ContainsFinalizer(rd, constant.Finalizer) {
	//		// our finalizer is present, so lets handle any external dependency
	//		// before deleting the policy
	//		if err := c.deleteExternalResources(ctx, rd); err != nil {
	//			// if fail to delete the external dependency here, return with error
	//			// so that it can be retried
	//			return err
	//		}
	//		// remove our finalizer from the list and update it.
	//		//controllerutil.RemoveFinalizer(rd, constant.Finalizer)
	//		//if err := c.Client.Update(ctx, rd, &client.UpdateOptions{}); err != nil {
	//		//	klog.Error("update ResourceDistribution error:", err)
	//		//	return err
	//		//}
	//	}
	//}

	switch eventObj.EventType {
	case EventCreate:
		klog.Info("EventCreate")
		err := c.eventAdd(ctx, eventObj)
		if err != nil {
			klog.Error("eventAdd error", "error", err)
			return err
		}
	case EventUpdate:
		klog.Info("EventUpdate")
		err := c.eventUpdate(ctx, eventObj)
		if err != nil {
			klog.Error("eventUpdate error", "error", err)
			return err
		}
	}

	//c.Recorder.Event(rd, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *DistributionController) eventAdd(ctx context.Context, object *EventObject) error {

	c.mu.Lock()
	defer c.mu.Unlock()

	// TODO: 添加操作，需要创建对应的informer

	resource, err := findReferenceResource(&object.Old.Spec.ResourceSelector)
	if err != nil {
		klog.Error("get reference resource error:", err)
		return err
	}

	c.KeyStore.StoreRelationship(resource, object.Old.Name)
	// 先创建rules
	err = c.storeRules(ctx, object.Old)
	if err != nil {
		klog.Error("store rules error:", err)
		return err
	}

	// 创建informer
	gvr, err := utils.GetGroupVersionResource(c.RESTMapper, resource.GroupVersionKind())
	if err != nil {
		return err
	}
	gvrn := custom.GroupVersionResourceNamespace{
		GroupVersionResource: gvr,
		Namespace:            object.Old.Namespace,
		LabelSelector:        "",
	}
	c.InformerManager.ForResource(gvrn, c.EventHandler)
	go c.InformerManager.Start()

	return nil

}

func (c *DistributionController) eventUpdate(ctx context.Context, object *EventObject) error {
	// 如果是更新操作
	// TODO: 是否应该把同步的资源给记录下来，如果对应的RD更新了，对应的资源需要被重新入队列

	c.mu.Lock()
	defer c.mu.Unlock()

	// 判断一下resourceSelector的gvk是否改了
	b, b1, b2 := changes(object.Old, object.New)
	if b && b1 && b2 {
		return nil
	}

	oldResource, err := findReferenceResource(&object.Old.Spec.ResourceSelector)
	if err != nil {
		return err
	}
	newResource, err := findReferenceResource(&object.New.Spec.ResourceSelector)
	if err != nil {
		return err
	}

	if b {
		d := &DeleteObject{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constant.ResourceDistribution: object.Old.Name,
				},
			},
		}
		// 如果resourceSelector变化了，需要更新rules
		c.KeyStore.RemoveRelationship(oldResource, object.New.Name)
		c.KeyStore.StoreRelationship(newResource, object.New.Name)

		// 删除对应的workload
		c.del <- d

		return nil
	}
	// placement修改或者overrideRules修改需要重新生成workload
	// 需要将新增一个map存储rd->资源
	// 这些资源需要重新入队列
	newOverrideRules, err := ParseResourceDistribution(ctx, c.Client, object.New)
	if err != nil {
		return err
	}
	oldOverrideRules, ok := c.RuleStore.Get(object.New.Name)
	if ok {
		// 对比overrideRules
		newRuleId := maputil.Keys(newOverrideRules)
		oldRuleId := maputil.Keys(oldOverrideRules)
		// 比较oldRuleId和newRuleId的差集
		// 如果oldRuleId有，newRuleId没有，需要删除对应的workload
		willDelete := slice.Difference(oldRuleId, newRuleId)
		c.RuleStore.DeleteAll(object.New.Name)
		c.RuleStore.StoreMap(object.New.Name, newOverrideRules)
		for _, ruleId := range willDelete {
			d := &DeleteObject{
				WorkloadName: nil,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constant.ResourceDistribution:       object.New.Name,
						constant.ResourceDistributionRuleId: ruleId,
					},
				},
			}
			c.del <- d
		}
	}

	//TODO: 重新入队列

	return nil
}

func (c *DistributionController) eventDelete(object *EventObject) error {

	// TODO:判断是否需要删除informer
	//key, err := findReferenceResource(&object.Old.Spec.ResourceSelector)
	//if err != nil {
	//	return err
	//}
	//gvr, err := getGroupVersionResource(c.RESTMapper, key.GroupVersionKind())
	//if err != nil {
	//	return err
	//}
	//numbers := c.informersManager.Numbers(gvr)
	//if numbers == 1 {
	//	c.informersManager.Remove(gvr)
	//}

	return nil
}

func changes(old, new *distributionv1.ResourceDistribution) (bool, bool, bool) {
	var resourceChanged, placementChanged, overrideRulesChanged bool

	equal := reflect.DeepEqual(old.Spec.ResourceSelector, new.Spec.ResourceSelector)
	if equal {
		resourceChanged = true
	}

	equal = reflect.DeepEqual(old.Spec.Placement, new.Spec.Placement)
	if equal {
		placementChanged = true
	}

	equal = reflect.DeepEqual(old.Spec.OverrideRules, new.Spec.OverrideRules)
	if equal {
		overrideRulesChanged = true
	}

	return resourceChanged, placementChanged, overrideRulesChanged
}

func (c *DistributionController) storeRules(ctx context.Context, rd *distributionv1.ResourceDistribution) error {

	rules, err := ParseResourceDistribution(ctx, c.Client, rd)
	if err != nil {
		return err
	}
	c.RuleStore.StoreMap(rd.Name, rules)
	return nil

}

func (c *DistributionController) comparedRules(ctx context.Context, rd *distributionv1.ResourceDistribution) ([]string, error) {

	newRules, err := ParseResourceDistribution(ctx, c.Client, rd)
	if err != nil {
		return nil, err
	}
	newRulesNames := maputil.Keys(newRules)
	oldRules, exist := c.RuleStore.Get(rd.Name)
	if !exist {
		for _, rule := range newRules {
			c.RuleStore.Store(rd.Name, rule)
		}
		return nil, nil
	}
	oldRulesNames := maputil.Keys(oldRules)
	// 找出存在oldRulesNames，但是不存在newRulesNames的规则，这些要删除
	difference := slice.Difference(oldRulesNames, newRulesNames)

	//for _, v := range difference {
	//	object := &DeleteObject{
	//		Selector: &metav1.LabelSelector{
	//			MatchLabels: map[string]string{
	//				constant.ResourceDistribution:       rd.Name,
	//				constant.ResourceDistributionRuleId: v,
	//			},
	//		},
	//	}
	//	c.del <- object
	//}

	return difference, nil

}

func (c *DistributionController) removeRules(rd *distributionv1.ResourceDistribution) error {

	rules, exist := c.RuleStore.Get(rd.Name)
	if !exist {
		return nil
	}
	r := maputil.Keys(rules)

	for _, v := range r {
		deleteObject := &DeleteObject{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constant.ResourceDistribution:       rd.Name,
					constant.ResourceDistributionRuleId: v,
				},
			},
		}
		c.del <- deleteObject
	}
	c.RuleStore.DeleteAll(rd.Name)

	return nil

}

func (c *DistributionController) updateExternalResources(ctx context.Context, rd *distributionv1.ResourceDistribution) error {
	if rd.Labels == nil {
		rd.Labels = make(map[string]string)
	}
	//rd.ObjectMeta.Finalizers = append(rd.ObjectMeta.Finalizers, constant.Finalizer)
	//_, err := c.Clientset.DistributionV1alpha1().ResourceDistributions().Update(ctx, rd, metav1.UpdateOptions{})
	err := c.Client.Update(ctx, rd, &client.UpdateOptions{})
	if err != nil {
		klog.Error("update rd error:", err)
		return err
	}
	return nil
}

func (c *DistributionController) deleteExternalResources(ctx context.Context, rd *distributionv1.ResourceDistribution) error {

	klog.Info("func:", "deleteExternalResources")

	// 如果是删除操作
	wideKey, err := findReferenceResource(&rd.Spec.ResourceSelector)
	if err != nil {
		return err
	}
	c.KeyStore.RemoveRelationship(wideKey, rd.Name)
	c.RuleStore.DeleteAll(rd.Name)

	d := &DeleteObject{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				constant.ResourceDistribution: rd.Name,
			},
		},
	}
	c.del <- d

	return nil
}

func (c *DistributionController) getClusterNameByLabelSelector(selector *metav1.LabelSelector) ([]string, error) {
	if selector == nil {
		return nil, nil
	}
	clusterList := v1alpha1.ClusterList{}
	err := c.Client.List(context.Background(), &clusterList, client.MatchingLabels(selector.MatchLabels))
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	var target []string
	for _, cluster := range clusterList.Items {
		target = append(target, cluster.Name)
	}
	return target, nil
}

func (c *DistributionController) getClusterName(ctx context.Context, pr *distributionv1.ResourceDistribution) ([]string, error) {
	var target []string
	if pr.Spec.Placement.ClusterAffinity != nil {
		for _, cluster := range pr.Spec.Placement.ClusterAffinity.ClusterNames {
			target = append(target, cluster)
		}
		if pr.Spec.Placement.ClusterAffinity.LabelSelector != nil {
			var clusterList v1alpha1.ClusterList
			s, err := metav1.LabelSelectorAsSelector(pr.Spec.Placement.ClusterAffinity.LabelSelector)
			if err != nil {
				return nil, err
			}
			err = c.Client.List(ctx, &clusterList, &client.ListOptions{
				LabelSelector: s,
			})
			if err != nil {
				klog.Error(err)
				return nil, err
			}
			for _, cluster := range clusterList.Items {
				target = append(target, cluster.Name)
			}
		}
	}
	return target, nil
}

func getGroupVersionResource(restMapper meta.RESTMapper, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	restMapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return restMapping.Resource, nil
}

func (c *DistributionController) creWork(name, ruleid string, clusters []string, rd *distributionv1.ResourceDistribution) *distributionv1.Workload {
	workload := &distributionv1.Workload{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Work",
			APIVersion: distribution.GroupName + "/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rd.Namespace,
			Labels: map[string]string{
				constant.SyncCluster:                strings.Join(clusters, ","), // 用于标记同步到哪些集群
				constant.ResourceDistribution:       rd.Name,
				constant.ResourceDistributionRuleId: ruleid,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(rd, schema.GroupVersionKind{
					Group:   "distribution.kubesphere.io",
					Version: "v1alpha1",
					Kind:    "ResourceDistribution",
				}),
			},
		},
		Spec:   distributionv1.WorkloadSpec{},
		Status: distributionv1.WorkloadStatus{},
	}
	return workload
}

func (c *DistributionController) GetStoredResources() (map[schema.GroupVersionResource]struct{}, error) {
	templates := c.KeyStore.GetAllTemplates()
	m := make(map[schema.GroupVersionResource]struct{})
	for _, template := range templates {
		gvk := schema.GroupVersionKind{
			Group:   template.Group,
			Version: template.Version,
			Kind:    template.Kind,
		}
		resource, err := getGroupVersionResource(c.RESTMapper, gvk)
		if err != nil {
			return nil, err
		}
		m[resource] = struct{}{}
	}
	return m, nil
}

// store resource and rd relation
func (c *DistributionController) storeRelationships(obj interface{}) error {
	resourceDistribution, ok := obj.(*distributionv1.ResourceDistribution)
	if !ok {
		return fmt.Errorf("obj is not ResourceDistribution")
	}
	customeResourceKey, err := getResourceKey(resourceDistribution)
	if err != nil {
		return err
	}
	namespaceKey, err := NamespaceKeyFunc(resourceDistribution)
	if err != nil {
		klog.Error("NamespaceKeyFunc error:", err)
		return err
	}
	c.KeyStore.StoreRelationship(customeResourceKey, namespaceKey)
	return nil
}

func getResourceKey(resourceDistribution *distributionv1.ResourceDistribution) (keys.ClusterWideKey, error) {
	wideKey, err := findReferenceResource(&resourceDistribution.Spec.ResourceSelector)
	if err != nil {
		klog.Error("findReferenceResource error:", err)
		return keys.ClusterWideKey{}, err
	}
	return wideKey, nil
}

// remove resource and rd relation
func (c *DistributionController) removeRelationships(obj interface{}) error {
	resourceDistribution, ok := obj.(*distributionv1.ResourceDistribution)
	if !ok {
		return fmt.Errorf("obj is not ResourceDistribution")
	}
	customrResourceKey, err := getResourceKey(resourceDistribution)
	if err != nil {
		return err
	}
	namespaceKey, err := NamespaceKeyFunc(resourceDistribution)
	if err != nil {
		klog.Error("NamespaceKeyFunc error:", err)
		return err
	}
	c.KeyStore.RemoveRelationship(customrResourceKey, namespaceKey)
	return nil
}

func (c *DistributionController) OnRDAdd(obj interface{}) {

	err := c.storeRelationships(obj)
	if err != nil {
		klog.Error("storeRelationships error:", err)
	}

	resourceDistribution := obj.(*distributionv1.ResourceDistribution)

	eventObject := &EventObject{
		EventType: EventCreate,
		Old:       resourceDistribution,
		New:       resourceDistribution,
	}

	c.enqueue(eventObject)

}

func (c *DistributionController) OnRDUpdate(oldObj, newObj interface{}) {

	unstructuredOldObj, err := utils.ToUnstructured(oldObj)
	if err != nil {
		klog.Errorf("Failed to transform oldObj, error: %v", err)
		return
	}
	unstructuredNewObj, err := utils.ToUnstructured(newObj)
	if err != nil {
		klog.Errorf("Failed to transform newObj, error: %v", err)
		return
	}
	if !utils.SpecificationChanged(unstructuredOldObj, unstructuredNewObj) {
		klog.V(4).Infof("Ignore update event of object (kind=%s, %s/%s) as specification no change", unstructuredOldObj.GetKind(), unstructuredOldObj.GetNamespace(), unstructuredOldObj.GetName())
		return
	}

	oldRd, ok := oldObj.(*distributionv1.ResourceDistribution)
	if !ok {
		return
	}
	newRd, ok := newObj.(*distributionv1.ResourceDistribution)
	if !ok {
		return
	}

	equal := reflect.DeepEqual(oldRd.Spec.Placement, newRd.Spec.Placement)
	if !equal {
		err = c.removeRelationships(oldObj)
		if err != nil {
			return
		}
		err = c.storeRelationships(newObj)
		if err != nil {
			return
		}
	}

	eventObj := &EventObject{
		EventType: EventUpdate,
		Old:       oldRd,
		New:       newRd,
	}

	c.enqueue(eventObj)

}

func (c *DistributionController) OnRDDelete(obj interface{}) {

	resourceDistribution, ok := obj.(*distributionv1.ResourceDistribution)
	if !ok {
		klog.Error("obj is not ResourceDistribution")
		return
	}
	eventObj := &EventObject{
		EventType: EventDelete,
		Old:       resourceDistribution,
		New:       resourceDistribution,
	}

	c.enqueue(eventObj)

}

func (c *DistributionController) findResourceDistribution(key string) (*distributionv1.ResourceDistribution, error) {
	_, name, err := toolscache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}
	rd, err := c.RdLister.Get(name)
	if err != nil {
		return nil, err
	}
	return rd, nil
}

func (c *DistributionController) deleteWorkload(obj *DeleteObject) {
	if obj == nil {
		if obj.WorkloadName != nil && len(obj.WorkloadName) > 0 {
			for _, workloadName := range obj.WorkloadName {
				temp := &distributionv1.Workload{}
				err := c.Client.Get(context.Background(), types.NamespacedName{
					Name: workloadName,
				}, temp)
				if err != nil {
					klog.Error("get workload error:", err)
					return
				}
				err = c.Client.Delete(context.Background(), temp)
				if err != nil {
					klog.Error("delete workload error:", err)
					return
				}
			}
		}
		if obj.Selector != nil {
			klog.Info("delete all of selector:", obj.Selector.MatchLabels)
			err := c.Client.DeleteAllOf(context.Background(), &distributionv1.Workload{}, client.MatchingLabels(obj.Selector.MatchLabels))
			if err != nil {
				klog.Error("delete workload error:", err)
				return
			}
		}
	}
}

func findReferenceResource(resourceSelector *distributionv1.ResourceSelector) (keys.ClusterWideKey, error) {
	if resourceSelector.APIVersion == "" {
		return keys.ClusterWideKey{}, fmt.Errorf("resourceSelector APIVersion is empty")
	}
	if resourceSelector.Kind == "" {
		return keys.ClusterWideKey{}, fmt.Errorf("resourceSelector Kind is empty")
	}
	gv, err := schema.ParseGroupVersion(resourceSelector.APIVersion)
	if err != nil {
		return keys.ClusterWideKey{}, err
	}
	key := keys.ClusterWideKey{
		Group:     gv.Group,
		Version:   gv.Version,
		Kind:      resourceSelector.Kind,
		Name:      resourceSelector.Name,
		Namespace: resourceSelector.Namespace,
	}
	return key, nil
}

// 创建名字
func getWorkloadName(re *keys.ClusterWideKey, ruleID, namespaceKey string) string {
	toString := re.ToString()
	str := fmt.Sprintf("%s-%s-%s", toString, ruleID, namespaceKey)
	sha1 := cryptor.Sha1(str)
	return fmt.Sprintf("workload-%s", sha1[0:10])
}

func NewFilteringHandlerOnAllEvents(filterFunc func(obj interface{}) bool, addFunc func(obj interface{}),
	updateFunc func(oldObj, newObj interface{}), deleteFunc func(obj interface{})) toolscache.ResourceEventHandler {
	return &toolscache.FilteringResourceEventHandler{
		FilterFunc: filterFunc,
		Handler: toolscache.ResourceEventHandlerFuncs{
			AddFunc:    addFunc,
			UpdateFunc: updateFunc,
			DeleteFunc: deleteFunc,
		},
	}
}

func (c *DistributionController) OnAdd(obj interface{}) {
	klog.Info("关联资源创建...")

	key, err := keys.ClusterWideKeyFunc(obj)
	if err != nil {
		return
	}
	if keys.IsReservedNamespace(key.Namespace) {
		return
	}
	excludeName := key.ExcludeName()

	find := c.KeyStore.GetDistributions(key)
	if find == nil || len(find) == 0 {
		find = c.KeyStore.GetDistributions(excludeName)
		if find == nil || len(find) == 0 {
			return
		}
	}

	bindObject := &BindObject{
		Obj:                   obj,
		ResourceDistributions: find,
	}

	c.cre <- bindObject

	return
}

func (c *DistributionController) OnUpdate(oldObj, newObj interface{}) {

	unstructuredOldObj, err := utils.ToUnstructured(oldObj)
	if err != nil {
		klog.Errorf("Failed to transform oldObj, error: %v", err)
		return
	}
	unstructuredNewObj, err := utils.ToUnstructured(newObj)
	if err != nil {
		klog.Errorf("Failed to transform newObj, error: %v", err)
		return
	}
	if !utils.SpecificationChanged(unstructuredOldObj, unstructuredNewObj) {
		klog.V(4).Infof("Ignore update event of object (kind=%s, %s/%s) as specification no change", unstructuredOldObj.GetKind(), unstructuredOldObj.GetNamespace(), unstructuredOldObj.GetName())
		return
	}

	klog.Info("关联资源更新...")

	key, err := keys.ClusterWideKeyFunc(newObj)
	if err != nil {
		return
	}
	if keys.IsReservedNamespace(key.Namespace) {
		return
	}
	excludeName := key.ExcludeName()

	find := c.KeyStore.GetDistributions(key)
	if find == nil || len(find) == 0 {
		find = c.KeyStore.GetDistributions(excludeName)
		if find == nil || len(find) == 0 {
			return
		}
	}

	bindObject := &BindObject{
		Obj:                   newObj,
		ResourceDistributions: find,
	}

	c.cre <- bindObject

	return

}

func (c *DistributionController) OnDelete(obj interface{}) {

	//clusterWideKey, err := keys.ClusterWideKeyFunc(obj)
	//if err != nil {
	//	klog.Errorf("Failed to get cluster wide key of object (kind=%s, %s/%s), error: %v", obj, err)
	//	return
	//}

	klog.Info("关联资源删除...")

	key, err := keys.ClusterWideKeyFunc(obj)
	if err != nil {
		return
	}
	if keys.IsReservedNamespace(key.Namespace) {
		return
	}
	excludeName := key.ExcludeName()

	find := c.KeyStore.GetDistributions(key)
	if find == nil || len(find) == 0 {
		find = c.KeyStore.GetDistributions(excludeName)
		if find == nil || len(find) == 0 {
			return
		}
	}

	for _, s := range find {
		deleteObj := &DeleteObject{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constant.ResourceDistribution: s,
				},
			},
		}
		c.del <- deleteObj
	}

	return
}

func (c *DistributionController) EventFilter(obj interface{}) bool {

	key, err := keys.ClusterWideKeyFunc(obj)
	if err != nil {
		return false
	}
	if keys.IsReservedNamespace(key.Namespace) {
		return false
	}
	excludeName := key.ExcludeName()

	find := c.KeyStore.GetDistributions(key)
	if find == nil || len(find) == 0 {
		exist := c.KeyStore.GetDistributions(excludeName)
		if exist == nil || len(exist) == 0 {
			return false
		}
		return true
	}

	return true

}
