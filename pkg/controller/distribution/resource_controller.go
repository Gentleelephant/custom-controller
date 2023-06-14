package distribution

import (
	"context"
	"fmt"
	"github.com/Gentleelephant/custom-controller/pkg/apis/cluster/v1alpha1"
	"github.com/Gentleelephant/custom-controller/pkg/apis/distribution"
	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/distribution/v1"
	clientset "github.com/Gentleelephant/custom-controller/pkg/client/clientset/versioned"
	clusterinformers "github.com/Gentleelephant/custom-controller/pkg/client/informers/externalversions/cluster/v1alpha1"
	distributioninformers "github.com/Gentleelephant/custom-controller/pkg/client/informers/externalversions/distribution/v1"
	clisters "github.com/Gentleelephant/custom-controller/pkg/client/listers/cluster/v1alpha1"
	listers "github.com/Gentleelephant/custom-controller/pkg/client/listers/distribution/v1"
	"github.com/Gentleelephant/custom-controller/pkg/constant"
	"github.com/Gentleelephant/custom-controller/pkg/utils"
	"github.com/Gentleelephant/custom-controller/pkg/utils/genericmanager"
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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

	Old *v1.ResourceDistribution

	New *v1.ResourceDistribution
}

//+kubebuilder:rbac:groups=distribution.kubesphere.io,resources=resourcedistributions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=distribution.kubesphere.io,resources=resourcedistributions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=distribution.kubesphere.io,resources=resourcedistributions/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch;update;

type DistributionController struct {
	Client           client.Client
	kubeclientset    kubernetes.Interface
	clientset        clientset.Interface
	rdLister         listers.ResourceDistributionLister
	clusterLister    clisters.ClusterLister
	scheme           *runtime.Scheme
	restMapper       meta.RESTMapper
	dynamicClient    dynamic.Interface
	discoverClient   discovery.DiscoveryClient
	Store            *KeyStore
	RuleStore        *ParsedOverrideRulesStore
	rdSynced         toolscache.InformerSynced
	clusterSynced    toolscache.InformerSynced
	workqueue        workqueue.RateLimitingInterface
	recorder         record.EventRecorder
	informersManager *genericmanager.InformerManager
	del              chan *DeleteObject
	cre              chan *BindObject
	EventHandler     toolscache.ResourceEventHandler
	stopCh           <-chan struct{}
	mu               sync.RWMutex
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
		kubeclientset:  kubeclientset,
		clientset:      clientset,
		scheme:         schema,
		restMapper:     restmapper,
		dynamicClient:  dynamicClient,
		discoverClient: discoverClient,
		rdLister:       rdinformer.Lister(),
		rdSynced:       rdinformer.Informer().HasSynced,
		clusterLister:  cinformer.Lister(),
		clusterSynced:  cinformer.Informer().HasSynced,
		workqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "distribution"),
		//recorder:       recorder,
	}

	controller.Store = NewKeyStore()
	controller.RuleStore = NewParsedOverrideRulesStore()
	controller.informersManager = genericmanager.NewInformerManager(dynamicClient, 0, ctx.Done())
	controller.del = make(chan *DeleteObject, ChannelSize)
	controller.cre = make(chan *BindObject, ChannelSize)
	controller.EventHandler = NewEventHandler(controller)

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

	for i := 0; i < 3; i++ {
		go func(num int) {
			klog.Infof("同步操作第%d个goroutine", num)
			for {
				select {
				case obj := <-c.cre:
					// TODO: 需要将对应的关联资源存储起来，后面如果rd有变化，这部分资源需要重新入队列
					c.createOrUpdateWorkload(obj)
				}
			}
		}(i)
	}

}

func (c *DistributionController) deleteObject() {
	for i := 0; i < 3; i++ {
		go func(num int) {
			klog.Infof("删除操作第%d个goroutine", num)
			for {
				select {
				case obj := <-c.del:
					c.deleteWorkload(obj)
				}
			}
		}(i)
	}
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
	for _, s := range obj.RdNamespaceKey {
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
			namespace, name, err := toolscache.SplitMetaNamespaceKey(s)
			if err != nil {
				klog.Error("create error:", err)
				continue
			}
			// 加上wokload的label
			deepcopy.GetLabels()[constant.WorkloadName] = namespace + "=" + wname
			rd, err := c.rdLister.ResourceDistributions(namespace).Get(name)
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
			temp := &v1.Workload{}
			err = c.Client.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: work.Name}, temp)
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
	c.workqueue.Add(obj)
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
	defer c.workqueue.ShutDown()
	c.stopCh = ctx.Done()
	logger := klog.FromContext(ctx)
	// Start the informer factories to begin populating the informer caches
	logger.Info("Starting ResourceDistribution controller")

	// Wait for the caches to be synced before starting workers
	logger.Info("Waiting for informer caches to sync")
	if ok := toolscache.WaitForCacheSync(ctx.Done(), c.rdSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	if ok := toolscache.WaitForCacheSync(ctx.Done(), c.clusterSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logger.Info("Starting workers", "count", workers)
	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}
	logger.Info("Started workers")

	// 启动applyRules
	go c.applyRules()
	go c.deleteObject()

	<-ctx.Done()
	logger.Info("Shutting down workers")

	return nil
}

func (c *DistributionController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *DistributionController) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.workqueue.Get()
	//logger := klog.FromContext(ctx)

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key *EventObject
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(*EventObject); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected EventObj in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(ctx, key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%v': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
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
	logger := klog.LoggerWithValues(klog.FromContext(ctx), "resourceName")

	rd := eventObj.Old

	if rd.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(rd, constant.Finalizer) {
			if err := c.updateExternalResources(context.Background(), rd); err != nil {
				logger.Error(err, "updateExternalResources error")
				return err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(rd, constant.Finalizer) {
			// our finalizer is present, so lets handle any external dependency
			// before deleting the policy
			if err := c.deleteExternalResources(ctx, rd); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return err
			}
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(rd, constant.Finalizer)
			if err := c.Client.Update(ctx, rd); err != nil {
				return err
			}
		}
	}

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
	case EventDelete:
		klog.Info("EventDelete")
		err := c.eventDelete(eventObj)
		if err != nil {
			klog.Error("eventDelete error", "error", err)
			return err
		}
	}

	//c.recorder.Event(rd, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *DistributionController) eventAdd(ctx context.Context, object *EventObject) error {

	// 如果是添加操作
	resource, err := findReferenceResource(&object.Old.Spec.ResourceSelector)
	if err != nil {
		klog.Error("get reference resource error:", err)
		return err
	}
	namespaceKey, err := toolscache.MetaNamespaceKeyFunc(object.Old)
	if err != nil {
		klog.Error("get namespacekey error:", err)
		return err
	}
	c.Store.StoreRelationship(resource, namespaceKey)

	// 先创建rules
	err = c.storeRules(ctx, object.Old)
	if err != nil {
		klog.Error("store rules error:", err)
		return err
	}
	// 再创建informer
	err = c.installInformer(object.Old)
	if err != nil {
		klog.Error("install informer error:", err)
		return err
	}
	return nil

}

func (c *DistributionController) eventUpdate(ctx context.Context, object *EventObject) error {
	// 如果是更新操作
	// TODO: 是否应该把同步的资源给记录下来，如果对应的RD更新了，对应的资源需要被重新入队列

	// 判断一下resourceSelector的gvk是否改了
	b, b1, b2 := changes(object.Old, object.New)
	if b && b1 && b2 {
		return nil
	}

	oldResource, err := findReferenceResource(&object.Old.Spec.ResourceSelector)
	if err != nil {
		return err
	}
	oldNamespaceKey, err := toolscache.MetaNamespaceKeyFunc(object.Old)
	if err != nil {
		return err
	}
	newResource, err := findReferenceResource(&object.New.Spec.ResourceSelector)
	if err != nil {
		return err
	}

	if b {
		// 如果resourceSelector变化了，需要更新rules
		c.Store.RemoveRelationship(oldResource, oldNamespaceKey)
		c.Store.StoreRelationship(newResource, oldNamespaceKey)
		// 删除这个rd对应的workload

		newOverrideRules, err := ParseResourceDistribution(ctx, c.Client, object.New)
		if err != nil {
			return err
		}
		c.RuleStore.DeleteAll(oldNamespaceKey)
		c.RuleStore.StoreMap(oldNamespaceKey, newOverrideRules)

		// 等待30ms，等待workqueue中的删除操作完成
		time.Sleep(30 * time.Millisecond)

		d := &DeleteObject{
			RdNamespaceKey: []string{oldNamespaceKey},
			WorkloadName:   nil,
			RuleId:         nil,
		}
		c.del <- d

		// 判断是否需要删除informer
		oldGvr, err := getGroupVersionResource(c.restMapper, oldResource.GroupVersionKind())
		if err != nil {
			return err
		}
		numbers := c.informersManager.Numbers(oldGvr)
		if numbers == 1 {
			c.informersManager.Remove(oldGvr)
		}

		// 是否需要创建informer
		gvr, err := getGroupVersionResource(c.restMapper, newResource.GroupVersionKind())
		if err != nil {
			return err
		}
		// 重启informer
		c.informersManager.Restart(gvr, c.EventHandler)
		return nil
	}

	if b1 || b2 {

		// placement修改或者overrideRules修改需要重新生成workload
		// 需要将新增一个map存储rd->资源
		// 这些资源需要重新入队列

		newOverrideRules, err := ParseResourceDistribution(ctx, c.Client, object.New)
		if err != nil {
			return err
		}
		oldOverrideRules, ok := c.RuleStore.Get(oldNamespaceKey)
		if ok {
			// 对比overrideRules
			newRuleId := maputil.Keys(newOverrideRules)
			oldRuleId := maputil.Keys(oldOverrideRules)
			// 比较oldRuleId和newRuleId的差集
			// 如果oldRuleId有，newRuleId没有，需要删除对应的workload
			willDelete := slice.Difference(oldRuleId, newRuleId)
			d := &DeleteObject{
				RdNamespaceKey: nil,
				WorkloadName:   nil,
				RuleId:         willDelete,
			}
			c.del <- d
		}
		c.RuleStore.DeleteAll(oldNamespaceKey)
		c.RuleStore.StoreMap(oldNamespaceKey, newOverrideRules)

	}

	// TODO: 如果placement改变了？
	// 暴力一点重新生成rules
	// placement改变不会新增workload
	// 单独修改placement没什么作用，除非将placement置为空

	// TODO: 如果overrideRules改变了？
	// overrideRules的改变有可能会减少或者新增workload
	// 因此需要对比rules, 如果有新增的rules，需要创建新的workload
	// 如果有减少的rules，需要删除对应的workload

	return nil
}

func (c *DistributionController) eventDelete(object *EventObject) error {
	// 如果是删除操作

	// TODO：删除操作，需要删除带有该RD label的所有Workload

	wideKey, err := findReferenceResource(&object.Old.Spec.ResourceSelector)
	if err != nil {
		return err
	}
	//
	namespaceKey, err := toolscache.MetaNamespaceKeyFunc(object.Old)
	if err != nil {
		return err
	}
	c.Store.RemoveRelationship(wideKey, namespaceKey)
	c.RuleStore.DeleteAll(namespaceKey)

	// 判断是否需要删除informer
	//key, err := findReferenceResource(&object.Old.Spec.ResourceSelector)
	//if err != nil {
	//	return err
	//}
	//gvr, err := getGroupVersionResource(c.restMapper, key.GroupVersionKind())
	//if err != nil {
	//	return err
	//}
	//numbers := c.informersManager.Numbers(gvr)
	//if numbers == 1 {
	//	c.informersManager.Remove(gvr)
	//}
	return nil
}

func changes(old, new *v1.ResourceDistribution) (bool, bool, bool) {
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

func (c *DistributionController) installInformer(rd *v1.ResourceDistribution) error {

	handler := &EventHandler{
		controller: c,
	}
	wideKey, err := findReferenceResource(&rd.Spec.ResourceSelector)
	if err != nil {
		return err
	}
	groupVersionKind := wideKey.GroupVersionKind()
	gvr, err := getGroupVersionResource(c.restMapper, groupVersionKind)
	if err != nil {
		return err
	}
	c.informersManager.ForResource(gvr, handler)
	return nil

}

func (c *DistributionController) storeRules(ctx context.Context, rd *v1.ResourceDistribution) error {

	namespaceKey, err := toolscache.MetaNamespaceKeyFunc(rd)
	if err != nil {
		return err
	}
	rules, err := ParseResourceDistribution(ctx, c.Client, rd)
	if err != nil {
		return err
	}
	c.RuleStore.StoreMap(namespaceKey, rules)
	return nil

}

func (c *DistributionController) comparedRules(ctx context.Context, rd *v1.ResourceDistribution) error {

	namespaceKey, err := toolscache.MetaNamespaceKeyFunc(rd)
	if err != nil {
		return err
	}
	newRules, err := ParseResourceDistribution(ctx, c.Client, rd)
	if err != nil {
		return err
	}
	newRulesNames := maputil.Keys(newRules)
	oldRules, exist := c.RuleStore.Get(namespaceKey)
	if !exist {
		for _, rule := range newRules {
			c.RuleStore.Store(namespaceKey, rule)
		}
		return nil
	}
	oldRulesNames := maputil.Keys(oldRules)
	// 找出存在oldRulesNames，但是不存在newRulesNames的规则，这些要删除
	difference := slice.Difference(oldRulesNames, newRulesNames)

	object := &DeleteObject{
		RuleId: difference,
	}

	c.del <- object

	return nil

}

func (c *DistributionController) removeRules(rd *v1.ResourceDistribution) error {

	namespaceKey, err := toolscache.MetaNamespaceKeyFunc(rd)
	if err != nil {
		return err
	}
	rules, exist := c.RuleStore.Get(namespaceKey)
	if !exist {
		return nil
	}
	//for _, rule := range rules {
	//	c.del <- rule.Id
	//}
	r := maputil.Keys(rules)

	deleteObject := &DeleteObject{
		RuleId: r,
	}

	c.del <- deleteObject

	c.RuleStore.DeleteAll(namespaceKey)

	return nil

}

func (c *DistributionController) updateExternalResources(ctx context.Context, rd *v1.ResourceDistribution) error {
	if rd.Labels == nil {
		rd.Labels = make(map[string]string)
	}
	rd.ObjectMeta.Finalizers = append(rd.ObjectMeta.Finalizers, constant.Finalizer)
	_, err := c.clientset.DistributionV1().ResourceDistributions(rd.Namespace).Update(ctx, rd, metav1.UpdateOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (c *DistributionController) deleteExternalResources(ctx context.Context, rd *v1.ResourceDistribution) error {
	err := c.Client.DeleteAllOf(ctx, &v1.Workload{}, client.InNamespace(rd.Namespace), client.MatchingLabels{constant.ResourceDistribution: rd.Name})
	if err != nil {
		klog.Error("delete workload error:", err)
		return err
	}
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

func (c *DistributionController) getClusterName(ctx context.Context, pr *v1.ResourceDistribution) ([]string, error) {
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

func (c *DistributionController) creWork(name, ruleid string, clusters []string, rd *v1.ResourceDistribution) *v1.Workload {
	workload := &v1.Workload{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Work",
			APIVersion: distribution.GroupName + "/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rd.Namespace,
			Labels: map[string]string{
				constant.SyncCluster:                strings.Join(clusters, ","), // 用于标记同步到哪些集群
				constant.ResourceDistribution:       rd.Namespace + "." + rd.Name,
				constant.ResourceDistributionRuleId: ruleid,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(rd, schema.GroupVersionKind{
					Group:   "distribution.kubesphere.io",
					Version: "v1",
					Kind:    "ResourceDistribution",
				}),
			},
		},
		Spec:   v1.WorkloadSpec{},
		Status: v1.WorkloadStatus{},
	}
	return workload
}

func (c *DistributionController) GetStoredResources() (map[schema.GroupVersionResource]struct{}, error) {
	templates := c.Store.GetAllTemplates()
	m := make(map[schema.GroupVersionResource]struct{})
	for _, template := range templates {
		gvk := schema.GroupVersionKind{
			Group:   template.Group,
			Version: template.Version,
			Kind:    template.Kind,
		}
		resource, err := getGroupVersionResource(c.restMapper, gvk)
		if err != nil {
			return nil, err
		}
		m[resource] = struct{}{}
	}
	return m, nil
}

// store resource and rd relation
func (c *DistributionController) storeRelationships(obj interface{}) error {
	resourceDistribution, ok := obj.(*v1.ResourceDistribution)
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
	c.Store.StoreRelationship(customeResourceKey, namespaceKey)
	return nil
}

func getResourceKey(resourceDistribution *v1.ResourceDistribution) (keys.ClusterWideKey, error) {
	wideKey, err := findReferenceResource(&resourceDistribution.Spec.ResourceSelector)
	if err != nil {
		klog.Error("findReferenceResource error:", err)
		return keys.ClusterWideKey{}, err
	}
	return wideKey, nil
}

// remove resource and rd relation
func (c *DistributionController) removeRelationships(obj interface{}) error {
	resourceDistribution, ok := obj.(*v1.ResourceDistribution)
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
	c.Store.RemoveRelationship(customrResourceKey, namespaceKey)
	return nil
}

func (c *DistributionController) OnRDAdd(obj interface{}) {

	err := c.storeRelationships(obj)
	if err != nil {
		klog.Error("storeRelationships error:", err)
	}

	resourceDistribution := obj.(*v1.ResourceDistribution)

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

	oldRd, ok := oldObj.(*v1.ResourceDistribution)
	if !ok {
		return
	}
	newRd, ok := newObj.(*v1.ResourceDistribution)
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

	resourceDistribution, ok := obj.(*v1.ResourceDistribution)
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

func (c *DistributionController) findResourceDistribution(key string) (*v1.ResourceDistribution, error) {
	namespace, name, err := toolscache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}
	rd, err := c.rdLister.ResourceDistributions(namespace).Get(name)
	if err != nil {
		return nil, err
	}
	return rd, nil
}

func (c *DistributionController) deleteWorkload(obj *DeleteObject) {

	if obj == nil {

		if obj.WorkloadName != nil && len(obj.WorkloadName) > 0 {
			for _, workloadName := range obj.WorkloadName {
				temp := &v1.Workload{}
				split := strings.Split(workloadName, ".")
				if len(split) != 2 {
					klog.Error("workloadName format error:", workloadName)
					return
				}
				err := c.Client.Get(context.Background(), types.NamespacedName{
					Namespace: split[0],
					Name:      split[1],
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

		if obj.RuleId != nil && len(obj.RuleId) > 0 {
			for _, ruleName := range obj.RuleId {
				err := c.Client.DeleteAllOf(context.Background(), &v1.Workload{}, client.MatchingLabels{
					constant.ResourceDistributionRuleId: ruleName,
				})
				if err != nil {
					klog.Error("delete workload error:", err)
					return
				}
			}
		}

		if obj.RdNamespaceKey != nil && len(obj.RdNamespaceKey) > 0 {
			for _, rd := range obj.RdNamespaceKey {
				err := c.Client.DeleteAllOf(context.Background(), &v1.Workload{}, client.MatchingLabels{
					constant.ResourceDistribution: rd,
				})
				if err != nil {
					klog.Error("delete workload error:", err)
					return
				}

			}

		}

	}

}

func findReferenceResource(resourceSelector *v1.ResourceSelector) (keys.ClusterWideKey, error) {
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
