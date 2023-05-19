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

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	"context"
	"time"

	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/distribution/v1"
	scheme "github.com/Gentleelephant/custom-controller/pkg/client/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ClusterResourceDistributionsGetter has a method to return a ClusterResourceDistributionInterface.
// A group's client should implement this interface.
type ClusterResourceDistributionsGetter interface {
	ClusterResourceDistributions() ClusterResourceDistributionInterface
}

// ClusterResourceDistributionInterface has methods to work with ClusterResourceDistribution resources.
type ClusterResourceDistributionInterface interface {
	Create(ctx context.Context, clusterResourceDistribution *v1.ClusterResourceDistribution, opts metav1.CreateOptions) (*v1.ClusterResourceDistribution, error)
	Update(ctx context.Context, clusterResourceDistribution *v1.ClusterResourceDistribution, opts metav1.UpdateOptions) (*v1.ClusterResourceDistribution, error)
	UpdateStatus(ctx context.Context, clusterResourceDistribution *v1.ClusterResourceDistribution, opts metav1.UpdateOptions) (*v1.ClusterResourceDistribution, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.ClusterResourceDistribution, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.ClusterResourceDistributionList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.ClusterResourceDistribution, err error)
	ClusterResourceDistributionExpansion
}

// clusterResourceDistributions implements ClusterResourceDistributionInterface
type clusterResourceDistributions struct {
	client rest.Interface
}

// newClusterResourceDistributions returns a ClusterResourceDistributions
func newClusterResourceDistributions(c *DistributionV1Client) *clusterResourceDistributions {
	return &clusterResourceDistributions{
		client: c.RESTClient(),
	}
}

// Get takes name of the clusterResourceDistribution, and returns the corresponding clusterResourceDistribution object, and an error if there is any.
func (c *clusterResourceDistributions) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.ClusterResourceDistribution, err error) {
	result = &v1.ClusterResourceDistribution{}
	err = c.client.Get().
		Resource("clusterresourcedistributions").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ClusterResourceDistributions that match those selectors.
func (c *clusterResourceDistributions) List(ctx context.Context, opts metav1.ListOptions) (result *v1.ClusterResourceDistributionList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.ClusterResourceDistributionList{}
	err = c.client.Get().
		Resource("clusterresourcedistributions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested clusterResourceDistributions.
func (c *clusterResourceDistributions) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("clusterresourcedistributions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a clusterResourceDistribution and creates it.  Returns the server's representation of the clusterResourceDistribution, and an error, if there is any.
func (c *clusterResourceDistributions) Create(ctx context.Context, clusterResourceDistribution *v1.ClusterResourceDistribution, opts metav1.CreateOptions) (result *v1.ClusterResourceDistribution, err error) {
	result = &v1.ClusterResourceDistribution{}
	err = c.client.Post().
		Resource("clusterresourcedistributions").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(clusterResourceDistribution).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a clusterResourceDistribution and updates it. Returns the server's representation of the clusterResourceDistribution, and an error, if there is any.
func (c *clusterResourceDistributions) Update(ctx context.Context, clusterResourceDistribution *v1.ClusterResourceDistribution, opts metav1.UpdateOptions) (result *v1.ClusterResourceDistribution, err error) {
	result = &v1.ClusterResourceDistribution{}
	err = c.client.Put().
		Resource("clusterresourcedistributions").
		Name(clusterResourceDistribution.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(clusterResourceDistribution).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *clusterResourceDistributions) UpdateStatus(ctx context.Context, clusterResourceDistribution *v1.ClusterResourceDistribution, opts metav1.UpdateOptions) (result *v1.ClusterResourceDistribution, err error) {
	result = &v1.ClusterResourceDistribution{}
	err = c.client.Put().
		Resource("clusterresourcedistributions").
		Name(clusterResourceDistribution.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(clusterResourceDistribution).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the clusterResourceDistribution and deletes it. Returns an error if one occurs.
func (c *clusterResourceDistributions) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Resource("clusterresourcedistributions").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *clusterResourceDistributions) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("clusterresourcedistributions").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched clusterResourceDistribution.
func (c *clusterResourceDistributions) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.ClusterResourceDistribution, err error) {
	result = &v1.ClusterResourceDistribution{}
	err = c.client.Patch(pt).
		Resource("clusterresourcedistributions").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
