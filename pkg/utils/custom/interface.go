package custom

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
)

type GroupVersionResourceNamespace struct {
	schema.GroupVersionResource

	Namespace string

	LabelSelector string
}

// CustomDynamicSharedInformerFactory provides access to a shared informer and lister for dynamic client
type CustomDynamicSharedInformerFactory interface {
	// Start initializes all requested informers. They are handled in goroutines
	// which run until the stop channel gets closed.
	Start(stopCh <-chan struct{})

	StopInformer(gvrn GroupVersionResourceNamespace)

	// ForResource gives generic access to a shared informer of the matching type.
	ForResource(gvrn GroupVersionResourceNamespace) informers.GenericInformer

	// WaitForCacheSync blocks until all started informers' caches were synced
	// or the stop channel gets closed.
	WaitForCacheSync(stopCh <-chan struct{}) map[GroupVersionResourceNamespace]bool

	// Shutdown marks a factory as shutting down. At that point no new
	// informers can be started anymore and Start will return without
	// doing anything.
	//
	// In addition, Shutdown blocks until all goroutines have terminated. For that
	// to happen, the close channel(s) that they were started with must be closed,
	// either before Shutdown gets called or while it is waiting.
	//
	// Shutdown may be called multiple times, even concurrently. All such calls will
	// block until all goroutines have terminated.
	Shutdown()
}

// TweakListOptionsFunc defines the signature of a helper function
// that wants to provide more listing options to API
type TweakListOptionsFunc func(*metav1.ListOptions)
