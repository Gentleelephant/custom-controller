package distribution

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Gentleelephant/custom-controller/pkg/apis/cluster/v1alpha1"
	"github.com/Gentleelephant/custom-controller/pkg/apis/distribution"
	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/distribution/v1"
	clientset "github.com/Gentleelephant/custom-controller/pkg/client/clientset/versioned"
	clusterinformers "github.com/Gentleelephant/custom-controller/pkg/client/informers/externalversions/cluster/v1alpha1"
	distributioninformers "github.com/Gentleelephant/custom-controller/pkg/client/informers/externalversions/distribution/v1"
	listers "github.com/Gentleelephant/custom-controller/pkg/client/listers/distribution/v1"
	"github.com/Gentleelephant/custom-controller/pkg/utils"
	"github.com/Gentleelephant/custom-controller/pkg/utils/genericmanager"
	"github.com/Gentleelephant/custom-controller/pkg/utils/keys"
	set "github.com/duke-git/lancet/v2/datastructure/set"
	"github.com/duke-git/lancet/v2/maputil"
	"github.com/duke-git/lancet/v2/slice"
	jsonpatch "github.com/evanphx/json-patch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"reflect"

	//"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
	"time"
)

const (
	ResourceDistributionPolicy = "distribution.kubesphere.io/policy"

	SyncCluster = "distribution.kubesphere.io/cluster"

	ResourceDistributionId = "distribution.kubesphere.io/id"

	ControllerName = "distribution-controller"

	KubernetesReservedNSPrefix = "kube-"

	Finalizer = "distribution.kubesphere.io/finalizer"

	ResourceDistributionAnnotation = "distribution.kubesphere.io/rd"

	WorkloadCLusterAnnotation = "distribution.kubesphere.io/workload-cluster"

	// SuccessSynced is used as part of the Event 'reason' when a Foo is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"
	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	// MessageResourceSynced is the message used for an Event fired when a Foo
	// is synced successfully
	MessageResourceSynced = "ResourceDistribution synced successfully"
)

type DistributionController struct {
	Client         client.Client
	kubeclientset  kubernetes.Interface
	clientset      clientset.Interface
	rdLister       listers.ResourceDistributionLister
	scheme         *runtime.Scheme
	restMapper     meta.RESTMapper
	dynamicClient  dynamic.Interface
	discoverClient discovery.DiscoveryClient
	//widekeyNamespace      map[string][]string
	//namespaceWidekey      map[string]string
	relationStore         *RelationStore
	rdSynced              toolscache.InformerSynced
	workqueue             workqueue.RateLimitingInterface
	recorder              record.EventRecorder
	EventHandler          toolscache.ResourceEventHandler
	InformerManager       genericmanager.SingleClusterInformerManager
	SkippedResourceConfig *utils.SkippedResourceConfig
	stopCh                <-chan struct{}
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
	informerManager genericmanager.SingleClusterInformerManager,
) *DistributionController {
	logger := klog.FromContext(ctx)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(schema, corev1.EventSource{Component: ControllerName})

	controller := &DistributionController{
		Client:          client,
		kubeclientset:   kubeclientset,
		clientset:       clientset,
		scheme:          schema,
		restMapper:      restmapper,
		dynamicClient:   dynamicClient,
		discoverClient:  discoverClient,
		rdLister:        rdinformer.Lister(),
		rdSynced:        rdinformer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "distribution"),
		recorder:        recorder,
		InformerManager: informerManager,
		//widekeyNamespace:      map[string][]string{},
		//namespaceWidekey:      map[string]string{},
		SkippedResourceConfig: utils.NewSkippedResourceConfig(),
	}

	controller.relationStore = NewRelationStore()

	logger.Info("Setting up ResourceDistribution event handlers")
	// Set up an event handler for when ResourceDistribution resources change
	rdinformer.Informer().AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controller.OnRDAdd(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			namespaceKey, err := NamespaceKeyFunc(new)
			if err != nil {
				return
			}
			controller.enqueue(namespaceKey)
		},
		DeleteFunc: func(obj interface{}) {
			controller.OnRDDelete(obj)
		},
	})

	// Set up an event handler for when Cluster resources change
	//TODO: 在这里需要找到该Cluster被哪些ResourceDistribution所引用，然后将找到的ResourceDistribution入队列
	cinformer.Informer().AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// cluster添加需要通知到ResourceDistribution
			// 这里需要找到该Cluster被哪些ResourceDistribution所引用，然后将找到的ResourceDistribution入队列
			//klog.Info("===>Cluster add event")
		},
		UpdateFunc: func(old, new interface{}) {
			// 暂时先不管
			// TODO:cluster更新label要通知到ResourceDistribution
			// 跟新了label，ResourceDistribution匹配的cluster就可能发生变化，所以需要通知到ResourceDistribution
		},
	})

	return controller
}

// 根据传入的Cluster对象，提取出Cluster的name、和label去找到ResourceDistribution，并将其入队列
func (c *DistributionController) findRDbyCluster(ctx context.Context, obj interface{}) {
	cluster, ok := obj.(v1alpha1.Cluster)
	if ok {
		name := cluster.Name
		labels := cluster.Labels
		// TODO: 查询所有ResourceDistribution
		resourceDistributionList := v1.ResourceDistributionList{}
		err := c.Client.List(ctx, &resourceDistributionList, nil)
		if err != nil {
			klog.Error("list all ResourceDistribution error:", err)
			return
		}
		for _, item := range resourceDistributionList.Items {
			if item.Spec.Placement.ClusterAffinity != nil {
				clusterNames := item.Spec.Placement.ClusterAffinity.ClusterNames
				labelSelector := item.Spec.Placement.ClusterAffinity.LabelSelector
				if labelSelector != nil {
					rdLabel := labelSelector.MatchLabels
					intersect := maputil.Intersect(labels, rdLabel)
					length := len(intersect)
					if length > 0 {
						// length > 0 说明resourceDistribution引用了该cluster
						// 需要将该resourceDistribution入队列
						namespaceKey, err := NamespaceKeyFunc(item)
						if err != nil {
							return
						}
						c.enqueue(namespaceKey)
						continue
					}
				}
				// 判断clustername是否在resourceDistribution引用的clustername中
				contain := slice.Contain(clusterNames, name)
				if contain {
					// 如果contain为true说明resourceDistribution引用了cluster
					namespaceKey, err := NamespaceKeyFunc(item)
					if err != nil {
						return
					}
					c.enqueue(namespaceKey)
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

func (c *DistributionController) enqueue(namespaceKey string) {
	c.workqueue.Add(namespaceKey)
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

	logger.Info("Starting workers", "count", workers)
	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}
	logger.Info("Started workers")

	// TODO: 处理其他事件监听
	c.EventHandler = NewFilteringHandlerOnAllEvents(c.EventFilter, c.OnAdd, c.OnUpdate, c.OnDelete)
	go c.discoverResources(30 * time.Second)

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
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(ctx, key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
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

func (c *DistributionController) syncHandler(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	logger := klog.LoggerWithValues(klog.FromContext(ctx), "resourceName", key)

	namespace, name, err := toolscache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the resource with this namespace/name
	rd, err := c.rdLister.ResourceDistributions(namespace).Get(name)
	if err != nil {
		// The RD resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("ResourceDistributions '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	if rd.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(rd, Finalizer) {
			rd.ObjectMeta.Finalizers = append(rd.ObjectMeta.Finalizers, Finalizer)
			if err = c.updateExternalResources(context.Background(), rd); err != nil {
				logger.Error(err, "updateExternalResources error")
				return nil
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(rd, Finalizer) {
			// our finalizer is present, so lets handle any external dependency
			// before deleting the policy
			if err = c.deleteExternalResources(ctx, rd); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return err
			}
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(rd, Finalizer)
			if err = c.Client.Update(ctx, rd); err != nil {
				return err
			}
		}
	}

	err = c.applyWorkloads(ctx, rd)
	if err != nil {
		return err
	}

	// Finally, we update the status block of the Foo resource to reflect the
	// current state of the world
	//err = c.updateFooStatus(foo, deployment)
	//if err != nil {
	//	return err
	//}

	c.recorder.Event(rd, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *DistributionController) updateExternalResources(ctx context.Context, rd *v1.ResourceDistribution) error {
	if rd.Labels == nil {
		rd.Labels = make(map[string]string)
	}
	rd.Labels[ResourceDistributionPolicy] = rd.Name
	//rd.Labels[ResourceDistributionId] = random.RandLower(8)
	_, err := c.clientset.DistributionV1().ResourceDistributions(rd.Namespace).Update(ctx, rd, metav1.UpdateOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (c *DistributionController) deleteExternalResources(ctx context.Context, rd *v1.ResourceDistribution) error {
	workList := v1.WorkloadList{}
	err := c.Client.List(ctx, &workList, client.MatchingLabels{ResourceDistributionPolicy: rd.Name})
	if err != nil {
		klog.Error(err)
		return err
	}
	for _, work := range workList.Items {
		err = c.Client.Delete(ctx, &work)
		if err != nil {
			klog.Error(err)
			return err
		}
	}
	return nil
}

func (c *DistributionController) applyWorkloads(ctx context.Context, policy *v1.ResourceDistribution) error {
	klog.Info("创建Workloads")
	works, err := c.generateWorks(ctx, policy)
	if err != nil {
		klog.Error(err)
		return nil
	}
	for _, work := range works {
		var workObj v1.Workload
		retry.RetryOnConflict(retry.DefaultRetry, func() error {
			err = c.Client.Get(ctx, types.NamespacedName{Name: work.Name, Namespace: work.Namespace}, &workObj)
			if err != nil {
				if errors.IsNotFound(err) {
					work.SetResourceVersion("")
					err = c.Client.Create(ctx, &work)
					if err != nil {
						klog.Error(err)
						return err
					}
				} else {
					klog.Error(err)
					return err
				}
			}
			if !reflect.DeepEqual(workObj.Spec, work.Spec) {
				workObj.Spec = work.Spec
				err = c.Client.Update(ctx, &workObj)
				if err != nil {
					klog.Error(err)
				}
				return err
			}
			return nil
		})
	}
	return err
}

func (c *DistributionController) generateWorks(ctx context.Context, policy *v1.ResourceDistribution) ([]v1.Workload, error) {
	clusterPlacement, err := c.getClusterName(ctx, policy)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	workV2map := c.createWorkV2(clusterPlacement, policy)
	overrideRules := policy.Spec.OverrideRules
	unstructObjArr, err := fetchResourceTemplateByRD(c.dynamicClient, c.restMapper, policy)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	var works []v1.Workload
	var usedCluster []string
	for _, item := range overrideRules {
		var arrObj []unstructured.Unstructured
		var clusterSlice []string
		for _, unstructObj := range unstructObjArr {
			deepCopyObj := unstructObj.DeepCopy()
			options := getOverrideOption(&item)
			err = applyJSONPatchs(deepCopyObj, options)
			if err != nil {
				klog.Error("apply json patch error:", err)
				return nil, err
			}
			arrObj = append(arrObj, *deepCopyObj)
		}
		if item.TargetCluster == nil {
			// 如果没有指定targetCluster，则使用placement中定义的cluster
			clusterSlice = clusterPlacement
		} else {
			var byLabel []string
			byLabel, err = c.getClusterNameByLabelSelector(item.TargetCluster.LabelSelector)
			if err != nil {
				return nil, err
			}
			clusterSlice = append(clusterSlice, byLabel...)
			clusterSlice = append(clusterSlice, item.TargetCluster.ClusterNames...)
			if err != nil {
				klog.Error(err)
				return nil, err
			}
		}
		// 如果clusterSlices中存在不属于placement定义的clustername，需要将这部分排除
		intersection := slice.Intersection(clusterPlacement, clusterSlice)
		for _, s := range intersection {
			wl := workV2map[s]
			err = c.fillWorkload(&wl, arrObj)
			if err != nil {
				return nil, err
			}
			works = append(works, wl)
		}
		usedCluster = append(usedCluster, intersection...)
	}
	// 将useCluster去重，再和clusterPlacement比较，如果存在不同的，说明还有需要同步的不用override的集群
	useClusterSet := set.NewSetFromSlice(usedCluster)
	clusterPlacementSet := set.NewSetFromSlice(clusterPlacement)
	minusSet := clusterPlacementSet.Minus(useClusterSet)
	residue := minusSet.Values()
	if len(residue) > 0 {
		for _, cname := range residue {
			work := workV2map[cname]
			err = c.fillWorkload(&work, unstructObjArr)
			if err != nil {
				return nil, err
			}
			works = append(works, work)
		}
	}
	return works, err
}

func (c *DistributionController) fillWorkload(workload *v1.Workload, uns []unstructured.Unstructured) error {
	for _, un := range uns {
		marshalJSON, err := un.MarshalJSON()
		if err != nil {
			return err
		}
		workload.Spec.WorkloadTemplate.Manifests = append(workload.Spec.WorkloadTemplate.Manifests, v1.Manifest{RawExtension: runtime.RawExtension{Raw: marshalJSON}})
	}
	return nil
}

//func (c *DistributionController) getWorkloadByCluster(ctx context.Context, clusterNames []string, policy *v1.ResourceDistribution) (map[string]v1.Workload, error) {
//	clusterWorkload := make(map[string]v1.Workload)
//	var temp v1.WorkloadList
//	for _, name := range clusterNames {
//		err := c.Client.List(ctx, &temp, &client.ListOptions{
//			Namespace: policy.GetNamespace(),
//			LabelSelector: labels.SelectorFromSet(map[string]string{
//				WorkloadCLusterAnnotation: name,
//			}),
//		})
//		if err != nil {
//			klog.Error(err)
//			return nil, err
//		}
//		if len(temp.Items) > 0 {
//			// 说明查询到了workload
//			clusterWorkload[name] = temp.Items[0]
//		} else {
//			// TODO：如果没有找到对应的workload，说明需要新建一个workload
//			w := v1.Workload{
//				TypeMeta: metav1.TypeMeta{
//					Kind:       "Workload",
//					APIVersion: "distribution.kubesphere.io/v1",
//				},
//				ObjectMeta: metav1.ObjectMeta{
//					Name:              "workload-" + name,
//					Namespace:         policy.GetNamespace(),
//					CreationTimestamp: metav1.Time{},
//					Labels:            map[string]string{},
//				},
//				Spec: v1.WorkloadSpec{
//					WorkloadTemplate: v1.WorkloadTemplate{
//						Manifests: []v1.Manifest{},
//					},
//				},
//				Status: v1.WorkloadStatus{},
//			}
//			w.Labels[WorkloadCLusterAnnotation] = name
//			w.Labels[ResourceDistributionAnnotation] = policy.GetName()
//			clusterWorkload[name] = w
//		}
//	}
//	return clusterWorkload, nil
//}

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
			err := c.Client.List(ctx, &clusterList, client.MatchingLabels(pr.Spec.Placement.ClusterAffinity.LabelSelector.MatchLabels))
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

func applyJSONPatchs(obj *unstructured.Unstructured, overrides []overrideOption) error {
	jsonPatchBytes, err := json.Marshal(overrides)
	if err != nil {
		return err
	}
	patch, err := jsonpatch.DecodePatch(jsonPatchBytes)
	if err != nil {
		return err
	}
	objectJSONBytes, err := obj.MarshalJSON()
	if err != nil {
		return err
	}
	patchedObjectJSONBytes, err := patch.Apply(objectJSONBytes)
	if err != nil {
		return err
	}
	err = obj.UnmarshalJSON(patchedObjectJSONBytes)
	if err != nil {
		return err
	}
	return nil
}

func fetchResourceTemplateByRD(dynamicClient dynamic.Interface, restMapper meta.RESTMapper, rd *v1.ResourceDistribution) ([]unstructured.Unstructured, error) {

	var unobjArr []unstructured.Unstructured
	gvr, err := getGroupVersionResource(restMapper, schema.FromAPIVersionAndKind(rd.Spec.ResourceSelectors.APIVersion, rd.Spec.ResourceSelectors.Kind))
	if err != nil {
		return nil, err
	}
	ns := rd.Spec.ResourceSelectors.Namespace
	name := rd.Spec.ResourceSelectors.Name
	kind := rd.Spec.ResourceSelectors.Kind
	if kind == "" {
		err = fmt.Errorf("kind is empty")
		return nil, err
	}

	if name != "" {
		obj, err := dynamicClient.Resource(gvr).Namespace(ns).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		unobjArr = append(unobjArr, *obj)
	} else {
		lists, err := dynamicClient.Resource(gvr).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, item := range lists.Items {
			unobjArr = append(unobjArr, item)
		}
	}
	if err != nil {
		klog.Error("Failed to transform object(%s/%s), Error: %v", ns, name, err)
		return nil, err
	}
	return unobjArr, nil

}

func getGroupVersionResource(restMapper meta.RESTMapper, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	restMapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return restMapping.Resource, nil
}

// create work
//func (c *DistributionController) createWork(policy *v1.ResourceDistribution, unstructured []unstructured.Unstructured, clusters []string, id string) (*v1.Workload, error) {
//
//	sort.Strings(clusters)
//	work := v1.Workload{
//		TypeMeta: metav1.TypeMeta{
//			Kind:       "Work",
//			APIVersion: distribution.GroupName + "/v1",
//		},
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      policy.GetName() + "-" + id,
//			Namespace: policy.GetNamespace(),
//			Labels: map[string]string{
//				SyncCluster: strings.Join(clusters, ","),
//				ResourceDistributionPolicy:  policy.GetName(),
//			},
//			OwnerReferences: []metav1.OwnerReference{
//				*metav1.NewControllerRef(policy, schema.GroupVersionKind{
//					Group:   "distribution.kubesphere.io",
//					Version: "v1",
//					Kind:    "ResourceDistribution",
//				}),
//			},
//		},
//	}
//
//	for _, u := range unstructured {
//		deepCopy := u.DeepCopy()
//		marshalJSON, err := deepCopy.MarshalJSON()
//		if err != nil {
//			klog.Error(err)
//			return nil, err
//		}
//		work.Spec.WorkloadTemplate.Manifests = append(work.Spec.WorkloadTemplate.Manifests, v1.Manifest{
//			RawExtension: runtime.RawExtension{
//				Raw: marshalJSON,
//			},
//		})
//	}
//
//	return &work, nil
//}

func getOverrideOption(overrideRules *v1.RuleWithCluster) []overrideOption {
	plaintext := overrideRules.Overriders.Plaintext
	var overrideOptions []overrideOption
	for i := range plaintext {
		var temp overrideOption
		temp.Path = plaintext[i].Path
		temp.Value = plaintext[i].Value
		temp.Op = string(plaintext[i].Operator)
		overrideOptions = append(overrideOptions, temp)
	}
	return overrideOptions
}

func (c *DistributionController) createWorkV2(clusterNames []string, rd *v1.ResourceDistribution) map[string]v1.Workload {
	result := make(map[string]v1.Workload)
	for _, name := range clusterNames {
		workload := v1.Workload{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Work",
				APIVersion: distribution.GroupName + "/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "workload-" + name + "-" + rd.Name,
				Namespace: rd.Namespace,
				Labels: map[string]string{
					SyncCluster:                name,
					ResourceDistributionPolicy: rd.Name,
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
		result[name] = workload
	}
	return result
}

//func (c *DistributionController) createWorkWithCluster(ctx context.Context, work *v1.Workload, clusters []string) {
//
//	for _, cluster := range clusters {
//		var clusterObj = v1alpha1.Cluster{}
//		err := c.Client.Get(ctx, types.NamespacedName{Name: cluster}, &clusterObj)
//		if err != nil {
//			klog.Error(err)
//		}
//		// 创建client
//		kubeconfig := clusterObj.Spec.Connection.KubeConfig
//		config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
//		if err != nil {
//			klog.Error("Failed to create rest config for cluster %s, Error: %v", cluster, err)
//		}
//		tempClient, err := client.New(config, client.Options{})
//		if err != nil {
//			klog.Error("Failed to create client for cluster %s, Error: %v", cluster, err)
//		}
//		err = tempClient.Create(ctx, work)
//		if err != nil {
//			klog.Error("Failed to create work for cluster %s, Error: %v", cluster, err)
//		}
//	}
//}

func (c *DistributionController) discoverResources(period time.Duration) {
	//klog.Infof("===>Start to discover resources")
	wait.Until(func() {
		newResources := GetDeletableResources(&c.discoverClient)
		for r := range newResources {
			if c.InformerManager.IsHandlerExist(r, c.EventHandler) || c.gvrDisabled(r) {
				continue
			}
			//klog.Infof("Setup informer for %s", r.String())
			c.InformerManager.ForResource(r, c.EventHandler)
		}
		c.InformerManager.Start()
	}, period, c.stopCh)
}

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

	gvks, err := c.restMapper.KindsFor(gvr)
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

// All discovery errors are considered temporary. Upon encountering any error,
// GetDeletableResources will log and return any discovered resources it was
// able to process (which may be none).
func GetDeletableResources(discoveryClient discovery.ServerResourcesInterface) map[schema.GroupVersionResource]struct{} {
	preferredResources, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		if discovery.IsGroupDiscoveryFailedError(err) {
			klog.Warningf("Failed to discover some groups: %v", err.(*discovery.ErrGroupDiscoveryFailed).Groups)
		} else {
			klog.Warningf("Failed to discover preferred resources: %v", err)
		}
	}
	if preferredResources == nil {
		return map[schema.GroupVersionResource]struct{}{}
	}

	// This is extracted from discovery.GroupVersionResources to allow tolerating
	// failures on a per-resource basis.
	deletableResources := discovery.FilteredBy(discovery.SupportsAllVerbs{Verbs: []string{"delete", "list", "watch"}}, preferredResources)
	deletableGroupVersionResources := map[schema.GroupVersionResource]struct{}{}
	for _, rl := range deletableResources {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			klog.Warningf("Ignore invalid discovered resource %q: %v", rl.GroupVersion, err)
			continue
		}
		for i := range rl.APIResources {
			deletableGroupVersionResources[schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: rl.APIResources[i].Name}] = struct{}{}
		}
	}

	return deletableGroupVersionResources
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

// EventFilter tells if an object should be take care of.
//
// All objects under Kubernetes reserved namespace should be ignored:
// - kube-*
// If '--skipped-propagating-namespaces' is specified, all APIs in the skipped-propagating-namespaces will be ignored.
func (c *DistributionController) EventFilter(obj interface{}) bool {
	key, err := ClusterWideKeyFunc(obj)
	if err != nil {
		return false
	}
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.Errorf("Invalid key")
		return false
	}
	if IsReservedNamespace(clusterWideKey.Namespace) {
		return false
	}
	// if SkippedPropagatingNamespaces is set, skip object events in these namespaces.
	//if _, ok := c.SkippedPropagatingNamespaces[clusterWideKey.Namespace]; ok {
	//	return false
	//}

	if unstructObj, ok := obj.(*unstructured.Unstructured); ok {
		switch unstructObj.GroupVersionKind() {
		// The secret, with type 'kubernetes.io/service-account-token', is created along with `ServiceAccount` should be
		// prevented from propagating.
		// Refer to https://github.com/karmada-io/karmada/pull/1525#issuecomment-1091030659 for more details.
		case corev1.SchemeGroupVersion.WithKind("Secret"):
			secretType, found, _ := unstructured.NestedString(unstructObj.Object, "type")
			if found && secretType == string(corev1.SecretTypeServiceAccountToken) {
				return false
			}
		}
	}

	// 判断这个资源和RD是否对应
	wideKeyToString := WideKeyToString(clusterWideKey)

	resourceKeys := c.relationStore.GetAllResourceKey()

	for _, rkey := range resourceKeys {
		// 进行前缀匹配
		if strings.HasPrefix(wideKeyToString, rkey) {
			klog.Info("前缀匹配成功")
			return true
		}
	}
	return false
}

func (c *DistributionController) getRdByKeyPrefix(key string) []string {

	var result []string
	resourceKeys := c.relationStore.GetAllResourceKey()
	for _, resourceKey := range resourceKeys {
		if strings.HasPrefix(key, resourceKey) {
			rdKey := c.relationStore.GetRdsByRe(resourceKey)
			result = append(result, rdKey...)
		}
	}
	return result
}

func (c *DistributionController) notifyRD(obj interface{}) {
	toUnstructured, err := ToUnstructured(obj)
	if err != nil {
		klog.Errorf("Failed to transform object, error: %v", err)
		return
	}
	wideKey := keys.ClusterWideKey{
		Group:     toUnstructured.GroupVersionKind().Group,
		Version:   toUnstructured.GroupVersionKind().Version,
		Kind:      toUnstructured.GroupVersionKind().Kind,
		Namespace: toUnstructured.GetNamespace(),
		Name:      toUnstructured.GetName(),
	}
	wideKeyToString := WideKeyToString(wideKey)
	rds := c.getRdByKeyPrefix(wideKeyToString)
	for _, rd := range rds {
		klog.Info("匹配成功，通知RD", rd)
		c.enqueue(rd)
	}
}

// OnAdd handles object add event and push the object to queue.
func (c *DistributionController) OnAdd(obj interface{}) {
	klog.Info("=======>OnAdd")
	c.notifyRD(obj)
}

// OnUpdate handles object update event and push the object to queue.
func (c *DistributionController) OnUpdate(oldObj, newObj interface{}) {
	klog.Info("=======>OnUpdate")
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
	c.notifyRD(newObj)
}

// OnDelete handles object delete event and push the object to queue.
func (c *DistributionController) OnDelete(obj interface{}) {
	klog.Info("=======>OnDelete")
	c.notifyRD(obj)
}

func (c *DistributionController) OnRDAdd(obj interface{}) {
	klog.Info("=======>OnRDAdd")
	rd, ok := obj.(*v1.ResourceDistribution)
	if !ok {
		return
	}
	key, err := findReferenceResource(&rd.Spec.ResourceSelectors)
	if err != nil {
		klog.Error("findReferenceResource error:", err)
		return
	}
	// group/version/kind/namespace/name
	resourceKey := WideKeyToString(key)
	namespaceKey := types.NamespacedName{
		Namespace: rd.Namespace,
		Name:      rd.Name,
	}
	rdKey := namespaceKey.String()
	c.relationStore.StoreReToRd(resourceKey, rdKey)
	c.relationStore.StoreRdToRe(rdKey, resourceKey)

	klog.Info("OnRDAdd resourceKey:", resourceKey)
	klog.Info("OnRDAdd rds:", c.relationStore.GetRdsByRe(resourceKey))
	klog.Info("OnRDAdd re:", c.relationStore.GetReByRd(rdKey))

	c.enqueue(rdKey)
}

func (c *DistributionController) OnRDUpdate(oldObj, newObj interface{}) {

	klog.Info("=======>OnRDUpdate")
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

	key := types.NamespacedName{
		Namespace: unstructuredNewObj.GetNamespace(),
		Name:      unstructuredNewObj.GetName(),
	}.String()
	// TODO: 这个rd如果修改了resourceSelector，那么也需要修改widekeyNamespace
	resourceDistributionOld, ok := oldObj.(*v1.ResourceDistribution)
	if !ok {
		return
	}
	keyOld, err := findReferenceResource(&resourceDistributionOld.Spec.ResourceSelectors)
	if err != nil {
		klog.Error(err)
		return
	}
	keyoldToString := WideKeyToString(keyOld)
	resourceDistributionNew, ok := newObj.(*v1.ResourceDistribution)
	if ok {
		return
	}
	keyNew, err := findReferenceResource(&resourceDistributionNew.Spec.ResourceSelectors)
	if err != nil {
		klog.Error(err)
		return
	}
	keynewToString := WideKeyToString(keyNew)
	if keyoldToString != keynewToString {
		c.relationStore.RemoveRdByRe(keyoldToString, key)
		c.relationStore.RemoveReByRd(key)
		c.relationStore.StoreReToRd(keynewToString, key)
		c.relationStore.StoreRdToRe(key, keynewToString)
	}
	c.OnAdd(key)
}

func (c *DistributionController) OnRDDelete(obj interface{}) {
	rd, ok := obj.(*v1.ResourceDistribution)
	if !ok {
		return
	}
	key, err := findReferenceResource(&rd.Spec.ResourceSelectors)
	if err != nil {
		klog.Error(err)
		return
	}
	rdNkey := types.NamespacedName{
		Namespace: rd.Namespace,
		Name:      rd.Name,
	}.String()
	wideKeyToString := WideKeyToString(key)

	c.relationStore.RemoveRdByRe(wideKeyToString, rdNkey)
	c.relationStore.RemoveReByRd(rdNkey)

	c.enqueue(rdNkey)
}

func findReferenceResource(resourceSelector *v1.ResourceSelector) (keys.ClusterWideKey, error) {

	if resourceSelector.APIVersion == "" {
		return keys.ClusterWideKey{}, fmt.Errorf("resourceSelector APIVersion is empty")
	}

	if resourceSelector.Kind == "" {
		return keys.ClusterWideKey{}, fmt.Errorf("resourceSelector Kind is empty")
	}

	apiversion := strings.Split(resourceSelector.APIVersion, "/")
	// TODO: 删除rd时，清除widekeyNamespace
	key := keys.ClusterWideKey{
		Kind:      resourceSelector.Kind,
		Name:      resourceSelector.Name,
		Namespace: resourceSelector.Namespace,
	}
	if len(apiversion) == 2 {
		key.Group = apiversion[0]
		key.Version = apiversion[1]
	} else {
		key.Version = apiversion[0]
	}

	return key, nil
}

func (c *DistributionController) findRDbyResourceGVK(resourceKey string) ([]string, error) {
	rds := c.relationStore.GetRdsByRe(resourceKey)
	return rds, nil
}

func ClusterWideKeyFunc(obj interface{}) (interface{}, error) {
	return keys.ClusterWideKeyFunc(obj)
}

func IsReservedNamespace(namespace string) bool {
	return strings.HasPrefix(namespace, KubernetesReservedNSPrefix) || strings.HasPrefix(namespace, "kubesphere-")
}

func WideKeyToString(key keys.ClusterWideKey) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s", key.Group, key.Version, key.Kind, key.Namespace, key.Name)
}
