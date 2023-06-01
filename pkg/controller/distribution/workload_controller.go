package distribution

import (
	"context"
	"fmt"
	"github.com/Gentleelephant/custom-controller/pkg/apis/cluster/v1alpha1"
	distributionv1 "github.com/Gentleelephant/custom-controller/pkg/apis/distribution/v1"
	clientset "github.com/Gentleelephant/custom-controller/pkg/client/clientset/versioned"
	clusterinformers "github.com/Gentleelephant/custom-controller/pkg/client/informers/externalversions/cluster/v1alpha1"
	v1 "github.com/Gentleelephant/custom-controller/pkg/client/informers/externalversions/distribution/v1"
	clusterlisters "github.com/Gentleelephant/custom-controller/pkg/client/listers/cluster/v1alpha1"
	listers "github.com/Gentleelephant/custom-controller/pkg/client/listers/distribution/v1"
	"github.com/Gentleelephant/custom-controller/pkg/utils"
	"github.com/Gentleelephant/custom-controller/pkg/utils/genericmanager"
	"github.com/Gentleelephant/custom-controller/pkg/utils/keys"
	kvcache "github.com/patrickmn/go-cache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"
)

const controllerAgentName = "workload-controller"

type WorkloadController struct {
	Client                client.Client
	kubeclientset         kubernetes.Interface
	workloadclientset     clientset.Interface
	scheme                *runtime.Scheme
	workloadLister        listers.WorkloadLister
	clusterLister         clusterlisters.ClusterLister
	Workqueue             workqueue.RateLimitingInterface
	restMapper            meta.RESTMapper
	recorder              record.EventRecorder
	clients               map[string]client.Client
	workloadSynced        cache.InformerSynced
	EventHandler          toolscache.ResourceEventHandler
	SkippedResourceConfig *utils.SkippedResourceConfig
	InformerManager       map[string]genericmanager.SingleClusterInformerManager
	Store                 *kvcache.Cache
	stopCh                <-chan struct{}
}

// NewController returns a new sample controller
func NewController(
	ctx context.Context,
	c client.Client,
	kubeclientset kubernetes.Interface,
	workloadclientset clientset.Interface,
	schema *runtime.Scheme,
	cinformer clusterinformers.ClusterInformer,
	restmapper meta.RESTMapper,
	workloadInformer v1.WorkloadInformer) *WorkloadController {
	logger := klog.FromContext(ctx)

	// Create event broadcaster
	logger.V(4).Info("Creating event broadcaster")

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(schema, corev1.EventSource{Component: controllerAgentName})

	controller := &WorkloadController{
		Client:                c,
		kubeclientset:         kubeclientset,
		workloadclientset:     workloadclientset,
		clusterLister:         cinformer.Lister(),
		restMapper:            restmapper,
		workloadLister:        workloadInformer.Lister(),
		workloadSynced:        workloadInformer.Informer().HasSynced,
		Workqueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Workload"),
		recorder:              recorder,
		scheme:                schema,
		clients:               make(map[string]client.Client),
		SkippedResourceConfig: utils.NewSkippedResourceConfig(),
	}

	controller.InformerManager = make(map[string]genericmanager.SingleClusterInformerManager)
	controller.Store = kvcache.New(kvcache.NoExpiration, kvcache.NoExpiration)
	//logger.Info("Setting up Workload event handlers")
	// Set up an event handler for when Foo resources change
	workloadInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.OnAdd,
		UpdateFunc: func(old, new interface{}) {
			controller.OnUpdate(old, new)
		},
		DeleteFunc: controller.OnDelete,
	})

	cinformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if controller.isHostCluster(obj) {
				return
			}
			// 监听member集群
			cluster, ok := obj.(*v1alpha1.Cluster)
			if !ok {
				return
			}
			go controller.discoverResources(ctx, 30*time.Second, cluster.Name)
		},
		UpdateFunc: func(old, new interface{}) {
			if controller.isHostCluster(new) {
				return
			}
		},
		DeleteFunc: func(obj interface{}) {
			if controller.isHostCluster(obj) {
				return
			}
		},
	})

	return controller
}

// 判断如果是host集群，则跳过
func (c *WorkloadController) isHostCluster(obj interface{}) bool {
	cluster, ok := obj.(*v1alpha1.Cluster)
	if !ok {
		return false
	}
	_, ok = cluster.Labels["cluster-role.kubesphere.io/host"]
	if ok {
		// 该集群为host集群
		return true
	}
	return false
}

func (c *WorkloadController) discoverResources(ctx context.Context, period time.Duration, clustername string) {
	handler := &ClusterEventHandler{
		ClusterName: clustername,
		Controller:  c,
	}
	handleFunc := handler.NewResourceEventHandler()
	// TODO：需要在每个member集群创建一个informer，用于监听该member集群的资源变化
	// 创建该member集群的dynamic client
	memberInformer, err := c.getMemberInformer(ctx, clustername)
	if err != nil {
		klog.Error(err)
		return
	}
	c.InformerManager[clustername] = memberInformer
	discoverClient, err := c.getMemberDiscoverClient(ctx, clustername)
	if err != nil {
		return
	}
	wait.Until(func() {
		newResources := GetDeletableResources(discoverClient)
		for r := range newResources {
			if memberInformer.IsHandlerExist(r, handleFunc) || c.gvrDisabled(r) {
				continue
			}
			klog.Infof("Setup informer for %s at cluster %s", r.String(), clustername)
			memberInformer.ForResource(r, handleFunc)
		}
		memberInformer.Start()
	}, period, c.stopCh)
}

// gvrDisabled returns whether GroupVersionResource is disabled.
func (c *WorkloadController) gvrDisabled(gvr schema.GroupVersionResource) bool {

	if c.SkippedResourceConfig == nil {
		return false
	}

	if c.SkippedResourceConfig.GroupVersionDisabled(gvr.GroupVersion()) {
		return true
	}
	if c.SkippedResourceConfig.GroupDisabled(gvr.Group) {
		return true
	}

	if c.allowGvr(gvr) {
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

func (c *WorkloadController) allowGvr(gvr schema.GroupVersionResource) bool {
	testgvr := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}
	if gvr != testgvr {
		return true
	}
	return false
}

func (c *WorkloadController) Start(ctx context.Context) error {
	err := c.Run(ctx, 4)
	if err != nil {
		return err
	}
	return nil
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the Workqueue and wait for
// workers to finish processing their current work items.
func (c *WorkloadController) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.Workqueue.ShutDown()
	logger := klog.FromContext(ctx)

	// Start the informer factories to begin populating the informer caches
	//logger.Info("Starting Workload controller")

	// Wait for the caches to be synced before starting workers
	//logger.Info("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(ctx.Done(), c.workloadSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	//logger.Info("Starting workers", "count", workers)
	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	//logger.Info("Started workers")
	<-ctx.Done()
	logger.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *WorkloadController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *WorkloadController) processNextWorkItem(ctx context.Context) bool {
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
		var key string
		var ok bool
		// We expect strings to come off the Workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// Workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// Workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the Workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.Workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in Workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(ctx, key); err != nil {
			// Put the item back on the Workqueue to handle any transient errors.
			c.Workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
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

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Foo resource
// with the current status of the resource.
func (c *WorkloadController) syncHandler(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	logger := klog.LoggerWithValues(klog.FromContext(ctx), "resourceName", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the workload resource with this namespace/name
	workload, err := c.workloadLister.Workloads(namespace).Get(name)
	if err != nil {
		// The Foo resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("workload '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	if workload.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(workload, Finalizer) {
			workload.ObjectMeta.Finalizers = append(workload.ObjectMeta.Finalizers, Finalizer)
			if err = c.updateExternalResources(context.Background(), workload); err != nil {
				logger.Error(err, "updateExternalResources error")
				return err
			}
			return nil
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(workload, Finalizer) {
			// our finalizer is present, so lets handle any external dependency
			// before deleting the policy
			//go c.deleteExternalResources(ctx, workload)
			if err = c.deleteExternalResources(ctx, workload); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return err
			}
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(workload, Finalizer)
			if err = c.Client.Update(ctx, workload); err != nil {
				return err
			}
			return nil
		}
	}

	err = c.syncWork(ctx, workload)
	if err != nil {
		return err
	}

	// Finally, we update the status block of the Foo resource to reflect the
	// current state of the world
	//err = c.updateWorkloadStatus(foo, deployment)
	//if err != nil {
	//	return err
	//}

	c.recorder.Event(workload, corev1.EventTypeNormal, SuccessSynced, "Workload Synced successfully")
	return nil
}

func (c *WorkloadController) enqueue(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.Workqueue.Add(key)
}

func (c *WorkloadController) updateExternalResources(ctx context.Context, workload *distributionv1.Workload) error {
	err := c.Client.Update(ctx, workload)
	if err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (c *WorkloadController) deleteExternalResources(ctx context.Context, w *distributionv1.Workload) error {
	klog.Info("delete member resources")
	memberClient, err := c.getClusterClient(ctx, w)
	if err != nil {
		klog.Error(err)
		return err
	}
	workload := unstructured.Unstructured{}
	manifests := w.Spec.WorkloadTemplate.Manifests
	for _, manifest := range manifests {
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Error("unmarshal manifest error:", err)
			return err
		}
		err = memberClient.Delete(ctx, &workload)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			klog.Error("delete resource error:", err)
		}
		klog.Info("资源删除成功")
	}
	return nil
}

func (c *WorkloadController) getClusterClient(ctx context.Context, work *distributionv1.Workload) (client.Client, error) {
	clustersName := work.Labels[SyncCluster]
	if clustersName == "" {
		return nil, fmt.Errorf("cluster name is empty")
	}
	cluster := v1alpha1.Cluster{}
	err := c.Client.Get(ctx, client.ObjectKey{Name: clustersName}, &cluster)
	if err != nil {
		klog.Error(err)
	}
	config := string(cluster.Spec.Connection.KubeConfig)
	restConfigFromKubeConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(config))
	if err != nil {
		klog.Error("failed to get rest config from kubeconfig: ", err)
	}
	ct, err := client.New(restConfigFromKubeConfig, client.Options{})
	if err != nil {
		klog.Error("failed to create ct: ", err)
	}
	return ct, nil
}

func (c *WorkloadController) syncWork(ctx context.Context, work *distributionv1.Workload) error {

	// TODO:  1、对比status和spec中的manifests，如果有存在status中但是不存在于spec中的对象，删除该资源

	// TODO：2、向member集群同步资源

	// TODO：3、更新workload的status
	klog.Info("syncWork..")

	unstruct := &unstructured.Unstructured{}
	manifests := work.Spec.WorkloadTemplate.Manifests
	// 获取集群客户端
	memberClient, err := c.getClusterClient(ctx, work)
	if err != nil {
		return err
	}
	for _, manifest := range manifests {
		//workloadStatus.ManifestStatuses = append(workloadStatus.ManifestStatuses, distributionv1.ManifestStatus{Status: &manifest.RawExtension})
		err := unstruct.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("Failed to unmarshal unstruct, error is: %v", err)
		}
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			temp := unstruct.DeepCopy()
			err = memberClient.Get(ctx, client.ObjectKeyFromObject(temp), temp)
			if err != nil {
				if errors.IsNotFound(err) {
					unstruct.SetResourceVersion("")
					err = memberClient.Create(ctx, unstruct)
					if err != nil {
						klog.Errorf("failed to create member resource", err)
					}
					return nil
				}
				klog.Error("get unstruct failed:", err)
				return err
			}

			if !reflect.DeepEqual(temp.Object["spec"], unstruct.Object["spec"]) {
				temp.Object["spec"] = unstruct.Object["spec"]
				unstruct.SetResourceVersion(temp.GetResourceVersion())
				err = memberClient.Update(ctx, unstruct)
				if err != nil {
					klog.Errorf("failed to update member resource", err)
					return err
				}
			}
			return nil
		})
		if err != nil {
			klog.Error("update member resource failed:", err)
		}
	}
	return nil
}

func (c *WorkloadController) OnAdd(obj interface{}) {
	c.storeResourceKey(obj)
	c.enqueue(obj)
}

func (c *WorkloadController) OnUpdate(oldObj, newObj interface{}) {
	// TODO 如果workload的同步集群发生变化，需要将原来的集群中的资源删除，然后再向新的集群中同步资源
	klog.Info("==>Workload OnUpdate")
	c.storeResourceKey(newObj)
	c.enqueue(newObj)
}

func (c *WorkloadController) OnDelete(obj interface{}) {
	klog.Info("==>Workload OnDelete")
	workoad, ok := obj.(*distributionv1.Workload)
	if !ok {
		klog.Error("workload convert failed")
		return
	}
	clusterName := workoad.Labels[SyncCluster]
	if clusterName == "" {
		return
	}
	namespaceKey, err := toolscache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error("failed to get namespace key: ", err)
		return
	}
	manifests := workoad.Spec.WorkloadTemplate.Manifests
	for _, manifest := range manifests {
		unstruct := &unstructured.Unstructured{}
		err := unstruct.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Error("failed to unmarshal manifest: ", err)
			return
		}
		wideKey := keys.ClusterWideKey{
			Group:     unstruct.GroupVersionKind().Group,
			Version:   unstruct.GroupVersionKind().Version,
			Kind:      unstruct.GroupVersionKind().Kind,
			Namespace: unstruct.GetNamespace(),
			Name:      unstruct.GetName(),
		}
		str := clusterName + "/" + WideKeyToString(wideKey)
		c.Store.Delete(namespaceKey)
		c.Store.Delete(str)
	}
	c.enqueue(obj)
}

func (c *WorkloadController) storeResourceKey(obj interface{}) {

	// TODO 当workload的spec发生了变化，那么Store中的键值对也需要发生变化
	klog.Info("==> storeResourceKey")
	workoad, ok := obj.(*distributionv1.Workload)
	if !ok {
		klog.Error("workload convert failed")
		return
	}
	clusterName := workoad.Labels[SyncCluster]
	if clusterName == "" {
		return
	}
	namespaceKey, err := toolscache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error("failed to get namespace key: ", err)
		return
	}
	rekey, ok := c.Store.Get(namespaceKey)
	var resourceKeys map[string]struct{}
	if !ok {
		resourceKeys = make(map[string]struct{})
	} else {
		resourceKeys = rekey.(map[string]struct{})
	}
	manifests := workoad.Spec.WorkloadTemplate.Manifests
	for _, manifest := range manifests {
		unstruct := &unstructured.Unstructured{}
		err := unstruct.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Error("failed to unmarshal manifest: ", err)
			return
		}
		wideKey := keys.ClusterWideKey{
			Group:     unstruct.GroupVersionKind().Group,
			Version:   unstruct.GroupVersionKind().Version,
			Kind:      unstruct.GroupVersionKind().Kind,
			Namespace: unstruct.GetNamespace(),
			Name:      unstruct.GetName(),
		}
		str := clusterName + "/" + WideKeyToString(wideKey)
		klog.Infof("存储资源%s--->%s", str, namespaceKey)
		resourceKeys[str] = struct{}{}
		c.Store.Set(namespaceKey, resourceKeys, kvcache.NoExpiration)
		c.Store.Set(str, namespaceKey, kvcache.NoExpiration)
	}
}

func (c *WorkloadController) getClusterKubeconfig(ctx context.Context, clusterName string) (string, error) {
	cluster := v1alpha1.Cluster{}
	err := c.Client.Get(ctx, client.ObjectKey{Name: clusterName}, &cluster)
	if err != nil {
		return "", err
	}
	return string(cluster.Spec.Connection.KubeConfig), nil
}

func (c *WorkloadController) getMemberInformer(ctx context.Context, clustername string) (genericmanager.SingleClusterInformerManager, error) {

	kubeconfig, err := c.getClusterKubeconfig(ctx, clustername)
	if err != nil {
		klog.Error("Failed to get kubeconfig:", err)
		return nil, err
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		klog.Error("Failed to create rest config:", err)
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		klog.Error("Failed to create dynamic client:", err)
		return nil, err
	}
	informerManager := genericmanager.NewSingleClusterInformerManager(dynamicClient, 0, ctx.Done())
	return informerManager, nil
}

func (c *WorkloadController) getMemberDiscoverClient(ctx context.Context, clustername string) (*discovery.DiscoveryClient, error) {
	kubeconfig, err := c.getClusterKubeconfig(ctx, clustername)
	if err != nil {
		klog.Error("Failed to get kubeconfig:", err)
		return nil, err
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		klog.Error("Failed to create rest config:", err)
		return nil, err
	}
	discoverClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return discoverClient, nil
}
