package custom

import (
	"context"
	"github.com/Gentleelephant/custom-controller/pkg/utils"
	"sync"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

// ClusterInformerManager manages dynamic shared informer for all resources, include Kubernetes resource and
// custom resources defined by CustomResourceDefinition.
type ClusterInformerManager interface {
	// ForResource builds a dynamic shared informer for 'resource' then set event handler.
	// If the informer already exist, the event handler will be appended to the informer.
	// The handler should not be nil.
	ForResource(resource GroupVersionResourceNamespace, handler cache.ResourceEventHandler)

	// IsInformerSynced checks if the resource's informer is synced.
	// An informer is synced means:
	// - The informer has been created(by method 'ForResource' or 'Lister').
	// - The informer has started(by method 'Start').
	// - The informer's cache has been synced.
	IsInformerSynced(resource GroupVersionResourceNamespace) bool

	// IsHandlerExist checks if handler already added to the informer that watches the 'resource'.
	IsHandlerExist(resource GroupVersionResourceNamespace, handler cache.ResourceEventHandler) bool

	// Lister returns a generic lister used to get 'resource' from informer's store.
	// The informer for 'resource' will be created if not exist, but without any event handler.
	Lister(resource GroupVersionResourceNamespace) cache.GenericLister

	// Start will run all informers, the informers will keep running until the channel closed.
	// It is intended to be called after create new informer(s), and it's safe to call multi times.
	Start()

	// Stop stops all single cluster informers of a cluster. Once it is stopped, it will be not able
	// to Start again.
	Stop()

	StopInformer(resource GroupVersionResourceNamespace)

	// WaitForCacheSync waits for all caches to populate.
	WaitForCacheSync() map[GroupVersionResourceNamespace]bool

	// WaitForCacheSyncWithTimeout waits for all caches to populate with a definitive timeout.
	WaitForCacheSyncWithTimeout(cacheSyncTimeout time.Duration) map[GroupVersionResourceNamespace]bool

	// Context returns the single cluster context.
	Context() context.Context

	// GetClient returns the dynamic client.
	GetClient() dynamic.Interface
}

// NewClusterInformerManager constructs a new instance of clusterInformerManagerImpl.
// defaultResync with value '0' means no re-sync.
func NewClusterInformerManager(client dynamic.Interface, defaultResync time.Duration, parentCh <-chan struct{}) ClusterInformerManager {
	ctx, cancel := utils.ContextForChannel(parentCh)

	return &clusterInformerManagerImpl{
		informerFactory: NewDynamicSharedInformerFactory(client, defaultResync),
		handlers:        make(map[GroupVersionResourceNamespace][]cache.ResourceEventHandler),
		syncedInformers: make(map[GroupVersionResourceNamespace]struct{}),
		ctx:             ctx,
		cancel:          cancel,
		client:          client,
	}
}

type clusterInformerManagerImpl struct {
	ctx    context.Context
	cancel context.CancelFunc

	informerFactory CustomDynamicSharedInformerFactory

	syncedInformers map[GroupVersionResourceNamespace]struct{}

	handlers map[GroupVersionResourceNamespace][]cache.ResourceEventHandler

	referenceNumber map[GroupVersionResourceNamespace]int

	client dynamic.Interface

	lock sync.RWMutex
}

func (s *clusterInformerManagerImpl) ForResource(resource GroupVersionResourceNamespace, handler cache.ResourceEventHandler) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// if handler already exist, just return, nothing changed.
	//if s.isHandlerExist(resource, handler) {
	//	return
	//}

	s.informerFactory.ForResource(resource).Informer().AddEventHandler(handler)
	//if err != nil {
	//	klog.Errorf("Failed to add handler for resource(%s): %v", resource.String(), err)
	//	return
	//}

	s.appendHandler(resource, handler)
}

func (s *clusterInformerManagerImpl) IsInformerSynced(resource GroupVersionResourceNamespace) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	_, exist := s.syncedInformers[resource]
	return exist
}

func (s *clusterInformerManagerImpl) IsHandlerExist(resource GroupVersionResourceNamespace, handler cache.ResourceEventHandler) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.isHandlerExist(resource, handler)
}

func (s *clusterInformerManagerImpl) isHandlerExist(resource GroupVersionResourceNamespace, handler cache.ResourceEventHandler) bool {
	handlers, exist := s.handlers[resource]
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

func (s *clusterInformerManagerImpl) Lister(resource GroupVersionResourceNamespace) cache.GenericLister {
	return s.informerFactory.ForResource(resource).Lister()
}

func (s *clusterInformerManagerImpl) appendHandler(resource GroupVersionResourceNamespace, handler cache.ResourceEventHandler) {
	// assume the handler list exist, caller should ensure for that.
	handlers := s.handlers[resource]

	// assume the handler not exist in it, caller should ensure for that.
	s.handlers[resource] = append(handlers, handler)
}

func (s *clusterInformerManagerImpl) Start() {
	s.informerFactory.Start(s.ctx.Done())
}

func (s *clusterInformerManagerImpl) StopInformer(resource GroupVersionResourceNamespace) {
	s.informerFactory.StopInformer(resource)
}

func (s *clusterInformerManagerImpl) Stop() {
	s.cancel()
}

func (s *clusterInformerManagerImpl) WaitForCacheSync() map[GroupVersionResourceNamespace]bool {
	return s.waitForCacheSync(s.ctx)
}

func (s *clusterInformerManagerImpl) WaitForCacheSyncWithTimeout(cacheSyncTimeout time.Duration) map[GroupVersionResourceNamespace]bool {
	ctx, cancel := context.WithTimeout(s.ctx, cacheSyncTimeout)
	defer cancel()

	return s.waitForCacheSync(ctx)
}

func (s *clusterInformerManagerImpl) waitForCacheSync(ctx context.Context) map[GroupVersionResourceNamespace]bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	res := s.informerFactory.WaitForCacheSync(ctx.Done())
	for resource, synced := range res {
		if _, exist := s.syncedInformers[resource]; !exist && synced {
			s.syncedInformers[resource] = struct{}{}
		}
	}
	return res
}

func (s *clusterInformerManagerImpl) Context() context.Context {
	return s.ctx
}

func (s *clusterInformerManagerImpl) GetClient() dynamic.Interface {
	return s.client
}
