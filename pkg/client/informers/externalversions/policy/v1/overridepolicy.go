/*
Copyright The Kubernetes Authors.

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

// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	time "time"

	policyv1 "github.com/Gentleelephant/custom-controller/pkg/apis/policy/v1"
	versioned "github.com/Gentleelephant/custom-controller/pkg/client/clientset/versioned"
	internalinterfaces "github.com/Gentleelephant/custom-controller/pkg/client/informers/externalversions/internalinterfaces"
	v1 "github.com/Gentleelephant/custom-controller/pkg/client/listers/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// OverridePolicyInformer provides access to a shared informer and lister for
// OverridePolicies.
type OverridePolicyInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.OverridePolicyLister
}

type overridePolicyInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewOverridePolicyInformer constructs a new informer for OverridePolicy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewOverridePolicyInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredOverridePolicyInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredOverridePolicyInformer constructs a new informer for OverridePolicy type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredOverridePolicyInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.PolicyV1().OverridePolicies(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.PolicyV1().OverridePolicies(namespace).Watch(context.TODO(), options)
			},
		},
		&policyv1.OverridePolicy{},
		resyncPeriod,
		indexers,
	)
}

func (f *overridePolicyInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredOverridePolicyInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *overridePolicyInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&policyv1.OverridePolicy{}, f.defaultInformer)
}

func (f *overridePolicyInformer) Lister() v1.OverridePolicyLister {
	return v1.NewOverridePolicyLister(f.Informer().GetIndexer())
}
