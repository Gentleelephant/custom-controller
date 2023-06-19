/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package custom

import (
	"context"
	"fmt"
	"github.com/Gentleelephant/custom-controller/pkg/utils"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamiclister"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// NewDynamicSharedInformerFactory constructs a new instance of dynamicSharedInformerFactory for all namespaces.
func NewDynamicSharedInformerFactory(client dynamic.Interface, defaultResync time.Duration) CustomDynamicSharedInformerFactory {
	return NewFilteredDynamicSharedInformerFactory(client, defaultResync, nil)
}

// NewFilteredDynamicSharedInformerFactory constructs a new instance of dynamicSharedInformerFactory.
// Listers obtained via this factory will be subject to the same filters as specified here.
func NewFilteredDynamicSharedInformerFactory(client dynamic.Interface, defaultResync time.Duration, tweakListOptions TweakListOptionsFunc) CustomDynamicSharedInformerFactory {
	return &dynamicSharedInformerFactory{
		client:           client,
		defaultResync:    defaultResync,
		informers:        map[GroupVersionResourceNamespace]informers.GenericInformer{},
		canles:           map[GroupVersionResourceNamespace]context.CancelFunc{},
		startedInformers: make(map[GroupVersionResourceNamespace]bool),
		tweakListOptions: tweakListOptions,
	}
}

type dynamicSharedInformerFactory struct {
	client        dynamic.Interface
	defaultResync time.Duration

	lock      sync.Mutex
	informers map[GroupVersionResourceNamespace]informers.GenericInformer
	// startedInformers is used for tracking which informers have been started.
	canles map[GroupVersionResourceNamespace]context.CancelFunc

	// This allows Start() to be called multiple times safely.
	startedInformers map[GroupVersionResourceNamespace]bool
	tweakListOptions TweakListOptionsFunc

	// wg tracks how many goroutines were started.
	wg sync.WaitGroup
	// shuttingDown is true when Shutdown has been called. It may still be running
	// because it needs to wait for goroutines.
	shuttingDown bool
}

func (f *dynamicSharedInformerFactory) StopInformer(gvrn GroupVersionResourceNamespace) {
	//TODO implement me
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.startedInformers[gvrn] {
		fmt.Println("stop informer")
		f.canles[gvrn]()
		delete(f.informers, gvrn)
		delete(f.startedInformers, gvrn)
		delete(f.canles, gvrn)
	}
}

var _ CustomDynamicSharedInformerFactory = &dynamicSharedInformerFactory{}

func (f *dynamicSharedInformerFactory) ForResource(gvrn GroupVersionResourceNamespace) informers.GenericInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	key := gvrn
	informer, exists := f.informers[key]
	if exists {
		return informer
	}
	var opt TweakListOptionsFunc
	if gvrn.LabelSelector != "" {
		opt = func(options *metav1.ListOptions) {
			options.LabelSelector = gvrn.LabelSelector
		}
	} else {
		opt = nil
	}

	informer = NewFilteredDynamicInformer(f.client, gvrn, gvrn.Namespace, f.defaultResync, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, opt)
	f.informers[key] = informer

	return informer
}

// Start initializes all requested informers.
func (f *dynamicSharedInformerFactory) Start(stopCh <-chan struct{}) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.shuttingDown {
		return
	}

	for informerType, informer := range f.informers {

		if !f.startedInformers[informerType] {
			f.wg.Add(1)

			ctx, cancle := utils.ContextForChannel(stopCh)
			f.canles[informerType] = cancle
			// We need a new variable in each loop iteration,
			// otherwise the goroutine would use the loop variable
			// and that keeps changing.
			informer := informer.Informer()
			go func() {
				defer f.wg.Done()
				informer.Run(ctx.Done())
			}()
			f.startedInformers[informerType] = true
		}
	}
}

// WaitForCacheSync waits for all started informers' cache were synced.
func (f *dynamicSharedInformerFactory) WaitForCacheSync(stopCh <-chan struct{}) map[GroupVersionResourceNamespace]bool {
	informers := func() map[GroupVersionResourceNamespace]cache.SharedIndexInformer {
		f.lock.Lock()
		defer f.lock.Unlock()

		informers := map[GroupVersionResourceNamespace]cache.SharedIndexInformer{}
		for informerType, informer := range f.informers {
			if f.startedInformers[informerType] {
				informers[informerType] = informer.Informer()
			}
		}
		return informers
	}()

	res := map[GroupVersionResourceNamespace]bool{}
	for informType, informer := range informers {
		res[informType] = cache.WaitForCacheSync(stopCh, informer.HasSynced)
	}
	return res
}

func (f *dynamicSharedInformerFactory) Shutdown() {
	// Will return immediately if there is nothing to wait for.
	defer f.wg.Wait()

	f.lock.Lock()
	defer f.lock.Unlock()
	f.shuttingDown = true
}

// NewFilteredDynamicInformer constructs a new informer for a dynamic type.
func NewFilteredDynamicInformer(client dynamic.Interface, gvrn GroupVersionResourceNamespace, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions TweakListOptionsFunc) informers.GenericInformer {
	gvr := gvrn.GroupVersionResource
	return &dynamicInformer{
		gvr: gvr,
		informer: cache.NewSharedIndexInformerWithOptions(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					if tweakListOptions != nil {
						tweakListOptions(&options)
					}
					return client.Resource(gvr).Namespace(namespace).List(context.TODO(), options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					if tweakListOptions != nil {
						tweakListOptions(&options)
					}
					return client.Resource(gvr).Namespace(namespace).Watch(context.TODO(), options)
				},
			},
			&unstructured.Unstructured{},
			cache.SharedIndexInformerOptions{
				ResyncPeriod:      resyncPeriod,
				Indexers:          indexers,
				ObjectDescription: gvr.String(),
			},
		),
	}
}

type dynamicInformer struct {
	informer cache.SharedIndexInformer
	gvr      schema.GroupVersionResource
}

var _ informers.GenericInformer = &dynamicInformer{}

func (d *dynamicInformer) Informer() cache.SharedIndexInformer {
	return d.informer
}

func (d *dynamicInformer) Lister() cache.GenericLister {
	return dynamiclister.NewRuntimeObjectShim(dynamiclister.New(d.informer.GetIndexer(), d.gvr))
}
