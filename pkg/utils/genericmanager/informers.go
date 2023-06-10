package genericmanager

import (
	"context"
	"github.com/Gentleelephant/custom-controller/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

type InformerManager struct {
	ctx context.Context

	informers map[schema.GroupVersionResource]informers.GenericInformer

	cancles map[schema.GroupVersionResource]context.CancelFunc

	handlers map[schema.GroupVersionResource][]cache.ResourceEventHandler

	// 记录某个informer被使用的次数
	numbers map[schema.GroupVersionResource]int

	defaultResync time.Duration

	client dynamic.Interface

	lock sync.RWMutex
}

func NewInformerManager(client dynamic.Interface, defaultResync time.Duration, parentCh <-chan struct{}) *InformerManager {
	c, _ := utils.ContextForChannel(parentCh)
	return &InformerManager{
		ctx:           c,
		informers:     make(map[schema.GroupVersionResource]informers.GenericInformer),
		cancles:       make(map[schema.GroupVersionResource]context.CancelFunc),
		handlers:      make(map[schema.GroupVersionResource][]cache.ResourceEventHandler),
		numbers:       make(map[schema.GroupVersionResource]int),
		client:        client,
		defaultResync: defaultResync,
		lock:          sync.RWMutex{},
	}
}

func (i *InformerManager) ForResource(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) informers.GenericInformer {
	i.lock.Lock()
	defer i.lock.Unlock()
	// if handler already exist, just return, nothing changed.
	if i.isHandlerExist(resource, handler) {
		return i.informers[resource]
	}
	informer := dynamicinformer.NewFilteredDynamicInformer(i.client, resource, metav1.NamespaceAll, i.defaultResync, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, nil)
	informer.Informer().AddEventHandler(handler)
	ctx, cancel := utils.ContextForChannel(i.ctx.Done())
	i.informers[resource] = informer
	i.cancles[resource] = cancel
	i.handlers[resource] = append(i.handlers[resource], handler)
	i.numbers[resource]++
	i.appendHandler(resource, handler)
	go informer.Informer().Run(ctx.Done())
	return informer
}

func (i *InformerManager) Stop(resource schema.GroupVersionResource) {
	i.lock.Lock()
	defer i.lock.Unlock()

	klog.Info("stop informer for resource: ", resource)

}

func (i *InformerManager) Restart(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) {
	i.lock.Lock()
	defer i.lock.Unlock()

	informer, ok := i.informers[resource]
	if !ok {
		i.ForResource(resource, handler)
		return
	}

	cancel, exist := i.cancles[resource]
	if !exist {
		return
	}
	cancel()

	ctx, cancel := utils.ContextForChannel(i.ctx.Done())
	i.cancles[resource] = cancel

	informer.Informer().Run(ctx.Done())

}

func (i *InformerManager) isHandlerExist(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) bool {
	handlers, exist := i.handlers[resource]
	if !exist {
		return false
	}

	for _, h := range handlers {
		if h == handler {
			return true
		}
	}

	return false
}

func (i *InformerManager) appendHandler(resource schema.GroupVersionResource, handler cache.ResourceEventHandler) {
	// assume the handler list exist, caller should ensure for that.
	handlers := i.handlers[resource]

	// assume the handler not exist in it, caller should ensure for that.
	i.handlers[resource] = append(handlers, handler)
}

func (i *InformerManager) Context() context.Context {
	return i.ctx
}

func (i *InformerManager) GetClient() dynamic.Interface {
	return i.client
}

func (i *InformerManager) Lister(resource schema.GroupVersionResource) cache.GenericLister {
	return i.informers[resource].Lister()
}

func (i *InformerManager) Numbers(resource schema.GroupVersionResource) int {
	return i.numbers[resource]
}

func (i *InformerManager) Remove(resource schema.GroupVersionResource) {
	i.lock.Lock()
	defer i.lock.Unlock()

	cancel, exist := i.cancles[resource]
	if !exist {
		return
	}
	cancel()
	klog.Info("remove informer for resource: ", resource)

	delete(i.informers, resource)
	delete(i.cancles, resource)
	delete(i.handlers, resource)
	delete(i.numbers, resource)
}
