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

package fake

import (
	"context"

	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/distribution/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterResourceDistributions implements ClusterResourceDistributionInterface
type FakeClusterResourceDistributions struct {
	Fake *FakeDistributionV1
}

var clusterresourcedistributionsResource = v1.SchemeGroupVersion.WithResource("clusterresourcedistributions")

var clusterresourcedistributionsKind = v1.SchemeGroupVersion.WithKind("ClusterResourceDistribution")

// Get takes name of the clusterResourceDistribution, and returns the corresponding clusterResourceDistribution object, and an error if there is any.
func (c *FakeClusterResourceDistributions) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.ClusterResourceDistribution, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterresourcedistributionsResource, name), &v1.ClusterResourceDistribution{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceDistribution), err
}

// List takes label and field selectors, and returns the list of ClusterResourceDistributions that match those selectors.
func (c *FakeClusterResourceDistributions) List(ctx context.Context, opts metav1.ListOptions) (result *v1.ClusterResourceDistributionList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterresourcedistributionsResource, clusterresourcedistributionsKind, opts), &v1.ClusterResourceDistributionList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ClusterResourceDistributionList{ListMeta: obj.(*v1.ClusterResourceDistributionList).ListMeta}
	for _, item := range obj.(*v1.ClusterResourceDistributionList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterResourceDistributions.
func (c *FakeClusterResourceDistributions) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterresourcedistributionsResource, opts))
}

// Create takes the representation of a clusterResourceDistribution and creates it.  Returns the server's representation of the clusterResourceDistribution, and an error, if there is any.
func (c *FakeClusterResourceDistributions) Create(ctx context.Context, clusterResourceDistribution *v1.ClusterResourceDistribution, opts metav1.CreateOptions) (result *v1.ClusterResourceDistribution, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterresourcedistributionsResource, clusterResourceDistribution), &v1.ClusterResourceDistribution{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceDistribution), err
}

// Update takes the representation of a clusterResourceDistribution and updates it. Returns the server's representation of the clusterResourceDistribution, and an error, if there is any.
func (c *FakeClusterResourceDistributions) Update(ctx context.Context, clusterResourceDistribution *v1.ClusterResourceDistribution, opts metav1.UpdateOptions) (result *v1.ClusterResourceDistribution, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterresourcedistributionsResource, clusterResourceDistribution), &v1.ClusterResourceDistribution{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceDistribution), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeClusterResourceDistributions) UpdateStatus(ctx context.Context, clusterResourceDistribution *v1.ClusterResourceDistribution, opts metav1.UpdateOptions) (*v1.ClusterResourceDistribution, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(clusterresourcedistributionsResource, "status", clusterResourceDistribution), &v1.ClusterResourceDistribution{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceDistribution), err
}

// Delete takes name of the clusterResourceDistribution and deletes it. Returns an error if one occurs.
func (c *FakeClusterResourceDistributions) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(clusterresourcedistributionsResource, name, opts), &v1.ClusterResourceDistribution{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterResourceDistributions) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterresourcedistributionsResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1.ClusterResourceDistributionList{})
	return err
}

// Patch applies the patch and returns the patched clusterResourceDistribution.
func (c *FakeClusterResourceDistributions) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.ClusterResourceDistribution, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterresourcedistributionsResource, name, pt, data, subresources...), &v1.ClusterResourceDistribution{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterResourceDistribution), err
}