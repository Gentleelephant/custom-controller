package distribution

import (
	"context"
	"fmt"
	"github.com/Gentleelephant/custom-controller/pkg/apis/cluster/v1alpha1"
	distributionv1 "github.com/Gentleelephant/custom-controller/pkg/apis/distribution/v1"
	clientset "github.com/Gentleelephant/custom-controller/pkg/client/clientset/versioned"
	v1 "github.com/Gentleelephant/custom-controller/pkg/client/informers/externalversions/distribution/v1"
	listers "github.com/Gentleelephant/custom-controller/pkg/client/listers/distribution/v1"
	"github.com/Gentleelephant/custom-controller/pkg/utils"
	"github.com/duke-git/lancet/v2/maputil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
	"time"
)

const controllerAgentName = "workload-controller"

type WorkloadController struct {
	Client            client.Client
	kubeclientset     kubernetes.Interface
	workloadclientset clientset.Interface
	scheme            *runtime.Scheme
	workloadLister    listers.WorkloadLister
	workqueue         workqueue.RateLimitingInterface
	recorder          record.EventRecorder
	clients           map[string]client.Client
	workloadSynced    cache.InformerSynced
	stopCh            <-chan struct{}
}

// NewController returns a new sample controller
func NewController(
	ctx context.Context,
	c client.Client,
	kubeclientset kubernetes.Interface,
	schema *runtime.Scheme,
	workloadInformer v1.WorkloadInformer) *WorkloadController {
	logger := klog.FromContext(ctx)

	// Create event broadcaster
	logger.V(4).Info("Creating event broadcaster")

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(schema, corev1.EventSource{Component: controllerAgentName})

	controller := &WorkloadController{
		Client:         c,
		kubeclientset:  kubeclientset,
		workloadLister: workloadInformer.Lister(),
		workloadSynced: workloadInformer.Informer().HasSynced,
		workqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Foos"),
		recorder:       recorder,
		scheme:         schema,
		clients:        make(map[string]client.Client),
	}

	//logger.Info("Setting up Workload event handlers")
	// Set up an event handler for when Foo resources change
	workloadInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.OnAdd,
		UpdateFunc: func(old, new interface{}) {
			controller.OnUpdate(old, new)
		},
		DeleteFunc: controller.OnDelete,
	})

	return controller
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
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *WorkloadController) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()
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

	//c.checkSyncResource(45 * time.Second)

	//logger.Info("Started workers")
	<-ctx.Done()
	logger.Info("Shutting down workers")

	return nil
}

func (c *WorkloadController) checkSyncResource(period time.Duration) {
	//klog.Infof("===>Start to check resources")
	wait.Until(func() {
		// TODO: check resource
		var workloadList distributionv1.WorkloadList
		err := c.Client.List(context.Background(), &workloadList, &client.ListOptions{})
		if err != nil {
			return
		}
		for _, workload := range workloadList.Items {
			clusterClient := c.getClusterClient(context.Background(), &workload)
			for _, manifest := range workload.Spec.WorkloadTemplate.Manifests {
				syncObj := unstructured.Unstructured{}
				err := syncObj.UnmarshalJSON(manifest.Raw)
				if err != nil {
					return
				}
				for _, memberClient := range clusterClient {
					var existObj unstructured.Unstructured
					err := memberClient.Get(context.Background(), types.NamespacedName{
						Namespace: syncObj.GetNamespace(), Name: syncObj.GetName(),
					}, &existObj)
					if err != nil {
						if errors.IsNotFound(err) {
							c.OnAdd(syncObj.DeepCopyObject())
							break
						}
						return
					}
				}
			}
		}
	}, period, c.stopCh)
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
			go c.deleteExternalResources(ctx, workload)
			//if err = c.deleteExternalResources(ctx, workload); err != nil {
			//	// if fail to delete the external dependency here, return with error
			//	// so that it can be retried
			//	return err
			//}
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(workload, Finalizer)
			if err = c.Client.Update(ctx, workload); err != nil {
				return err
			}
			return nil
		}
	}

	c.syncWork(ctx, workload)

	// Finally, we update the status block of the Foo resource to reflect the
	// current state of the world
	//err = c.updateWorkloadStatus(foo, deployment)
	//if err != nil {
	//	return err
	//}

	c.recorder.Event(workload, corev1.EventTypeNormal, SuccessSynced, "Workload Synced successfully")
	return nil
}

//func (c *WorkloadController) updateWorkloadStatus(workload *v12.Workload, deployment *appsv1.Deployment) error {
//	// NEVER modify objects from the store. It's a read-only, local cache.
//	// You can use DeepCopy() to make a deep copy of original object and modify this copy
//	// Or create a copy manually for better performance
//	fooCopy := foo.DeepCopy()
//	fooCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
//	// If the CustomResourceSubresources feature gate is not enabled,
//	// we must use Update instead of UpdateStatus to update the Status block of the Foo resource.
//	// UpdateStatus will not allow changes to the Spec of the resource,
//	// which is ideal for ensuring nothing other than resource status has been updated.
//	_, err := c.sampleclientset.SamplecontrollerV1alpha1().Foos(foo.Namespace).UpdateStatus(context.TODO(), fooCopy, metav1.UpdateOptions{})
//	return err
//}

// enqueueFoo takes a Foo resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Foo.
func (c *WorkloadController) enqueue(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *WorkloadController) updateExternalResources(ctx context.Context, workload *distributionv1.Workload) error {
	err := c.Client.Update(ctx, workload)
	if err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (c *WorkloadController) deleteExternalResources(ctx context.Context, w *distributionv1.Workload) {
	klog.Error("===>删除workload:", "work", w.Name)
	clients := c.getClusterClient(context.Background(), w)
	workload := unstructured.Unstructured{}
	manifests := w.Spec.WorkloadTemplate.Manifests
	for _, manifest := range manifests {
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Error(err)
		}
		for _, memberClient := range clients {
			err = memberClient.Delete(ctx, &workload)
			if err != nil {
				klog.Error(err)
				if errors.IsNotFound(err) {
					continue
				}
			}
			klog.Info("资源删除成功")
		}
	}
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Foo resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Foo resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
//func (c *WorkloadController) handleObject(obj interface{}) {
//	var object metav1.Object
//	var ok bool
//	logger := klog.FromContext(context.Background())
//	if object, ok = obj.(metav1.Object); !ok {
//		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
//		if !ok {
//			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
//			return
//		}
//		object, ok = tombstone.Obj.(metav1.Object)
//		if !ok {
//			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
//			return
//		}
//		logger.V(4).Info("Recovered deleted object", "resourceName", object.GetName())
//	}
//	logger.V(4).Info("Processing object", "object", klog.KObj(object))
//	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
//		// If this object is not owned by a Foo, we should not do anything more
//		// with it.
//		if ownerRef.Kind != "Foo" {
//			return
//		}
//
//		workload, err := c.workloadLister.Workloads(object.GetNamespace()).Get(ownerRef.Name)
//		if err != nil {
//			logger.V(4).Info("Ignore orphaned object", "object", klog.KObj(object), "foo", ownerRef.Name)
//			return
//		}
//
//		c.enqueueFoo(workload)
//		return
//	}
//}

func (c *WorkloadController) getClusterClient(ctx context.Context, work *distributionv1.Workload) map[string]client.Client {
	var clientsByName = make(map[string]client.Client)
	clustersName := work.Spec.WorkloadTemplate.Clusters
	for _, cname := range clustersName {
		cluster := v1alpha1.Cluster{}
		err := c.Client.Get(ctx, client.ObjectKey{Name: cname}, &cluster)
		if err != nil {
			klog.Error(err)
			continue
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
		clientsByName[cname] = ct
	}
	return clientsByName
}

func (c *WorkloadController) syncWork(ctx context.Context, work *distributionv1.Workload) {

	generateStatus(work)
	workloadStatus := work.Status
	errorMessages := workloadStatus.ErrorMessage
	workload := &unstructured.Unstructured{}
	manifests := work.Spec.WorkloadTemplate.Manifests
	// 获取集群客户端
	clients := c.getClusterClient(ctx, work)
	for _, manifest := range manifests {
		workloadStatus.ManifestStatuses = append(workloadStatus.ManifestStatuses, distributionv1.ManifestStatus{Status: &manifest.RawExtension})
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("Failed to unmarshal workload, error is: %v", err)
		}
		for clusterName, memberClient := range clients {
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				dpwl := workload.DeepCopy()
				err = memberClient.Get(ctx, client.ObjectKeyFromObject(dpwl), dpwl)
				if err != nil {
					if errors.IsNotFound(err) {
						workload.SetResourceVersion("")
						err = memberClient.Create(ctx, workload)
						if err != nil {
							klog.Errorf("failed to create workload", err)
							errorMessages = append(errorMessages, distributionv1.ErrorMessage{Cluster: clusterName, Message: err.Error()})
						}
						return nil
					}
					klog.Error("get workload failed:", err)
					errorMessages = append(errorMessages, distributionv1.ErrorMessage{Cluster: clusterName, Message: err.Error()})
					return err
				}

				if !reflect.DeepEqual(dpwl.Object["spec"], workload.Object["spec"]) {
					dpwl.Object["spec"] = workload.Object["spec"]
					workload.SetResourceVersion(dpwl.GetResourceVersion())
					err = memberClient.Update(ctx, workload)
					if err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				klog.Error("update workload failed:", err)
				errorMessages = append(errorMessages, distributionv1.ErrorMessage{Cluster: clusterName, Message: err.Error()})
			}
		}
	}
	work.Status = workloadStatus
}

func generateStatus(work *distributionv1.Workload) {
	var identifier distributionv1.ResourceIdentifier
	workload := &unstructured.Unstructured{}
	manifests := work.Spec.WorkloadTemplate.Manifests
	if len(manifests) != 0 {
		err := workload.UnmarshalJSON(manifests[0].Raw)
		if err != nil {
			klog.Error(err)
		}
		identifier.Kind = workload.GetKind()
		identifier.Name = workload.GetName()
		identifier.Namespace = workload.GetNamespace()
		apiVersion := workload.GetAPIVersion()
		split := strings.Split(apiVersion, "/")
		if len(split) == 2 {
			identifier.Group = split[0]
			identifier.Version = split[1]
		}
		identifier.Group = ""
		identifier.Version = split[0]
	}
	var clusters []string
	copy(clusters, work.Spec.WorkloadTemplate.Clusters)
	status := distributionv1.WorkloadStatus{
		Clusters:         clusters,
		ManifestStatuses: []distributionv1.ManifestStatus{},
		Identifier:       identifier,
		ErrorMessage:     []distributionv1.ErrorMessage{},
	}
	work.Status = status
}

func (c *WorkloadController) OnAdd(obj interface{}) {

	workload, ok := obj.(*distributionv1.Workload)
	if !ok {
		klog.Error("workload convert failed")
		return
	}
	clusterClient := c.getClusterClient(context.Background(), workload)
	maputil.Merge(c.clients, clusterClient)
	c.enqueue(workload)

}

func (c *WorkloadController) OnUpdate(oldObj, newObj interface{}) {
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
	c.enqueue(newObj)
}

func (c WorkloadController) OnDelete(obj interface{}) {
	c.enqueue(obj)
}
