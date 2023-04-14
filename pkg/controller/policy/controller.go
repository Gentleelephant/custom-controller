package policy

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
)

const (
	controllerName = "custom-controller"
)

const (
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
	MessageResourceSynced = "Foo synced successfully"
)

type Controller struct {
	ctrlcache ctrlcache.Cache

	// fooSynced returns true if the foo store has been synced at least once.
	foosSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface

	recorder record.EventRecorder
}

func NewController(ctrlcache ctrlcache.Cache) *Controller {
	eventBroadcaster := record.NewBroadcaster()
	//eventBroadcaster.StartLogging(klog.Infof)
	//eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: k8sClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerName})
	controller := &Controller{
		ctrlcache: ctrlcache,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Foos"),
		recorder:  recorder,
	}

	return controller

}

func (c *Controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) Start(ctx context.Context) error {
	go c.Run(3, ctx.Done())
	return nil
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer c.workqueue.ShutDown()

	if ok := cache.WaitForCacheSync(stopCh, c.foosSynced); !ok {
		return nil
	}

	for i := 0; i < threadiness; i++ {
		go c.runWorker()
	}

	<-stopCh
	return nil
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	key, quit := c.workqueue.Get()
	if quit {
		return false
	}
	defer c.workqueue.Done(key)

	err := c.syncHandler(key.(string))
	if err == nil {
		//q: why forget?
		//a: forget means that the item has been successfully processed and can be removed from the queue
		c.workqueue.Forget(key)
		return true
	}

	c.workqueue.AddRateLimited(key)
	return true
}

func (c *Controller) syncHandler(s string) error {
	fmt.Println("syncHandler:", s)
	return nil
}
