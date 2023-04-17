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

	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusterPropagationPolicies implements ClusterPropagationPolicyInterface
type FakeClusterPropagationPolicies struct {
	Fake *FakePolicyV1
}

var clusterpropagationpoliciesResource = v1.SchemeGroupVersion.WithResource("clusterpropagationpolicies")

var clusterpropagationpoliciesKind = v1.SchemeGroupVersion.WithKind("ClusterPropagationPolicy")

// Get takes name of the clusterPropagationPolicy, and returns the corresponding clusterPropagationPolicy object, and an error if there is any.
func (c *FakeClusterPropagationPolicies) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.ClusterPropagationPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(clusterpropagationpoliciesResource, name), &v1.ClusterPropagationPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterPropagationPolicy), err
}

// List takes label and field selectors, and returns the list of ClusterPropagationPolicies that match those selectors.
func (c *FakeClusterPropagationPolicies) List(ctx context.Context, opts metav1.ListOptions) (result *v1.ClusterPropagationPolicyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(clusterpropagationpoliciesResource, clusterpropagationpoliciesKind, opts), &v1.ClusterPropagationPolicyList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ClusterPropagationPolicyList{ListMeta: obj.(*v1.ClusterPropagationPolicyList).ListMeta}
	for _, item := range obj.(*v1.ClusterPropagationPolicyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusterPropagationPolicies.
func (c *FakeClusterPropagationPolicies) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(clusterpropagationpoliciesResource, opts))
}

// Create takes the representation of a clusterPropagationPolicy and creates it.  Returns the server's representation of the clusterPropagationPolicy, and an error, if there is any.
func (c *FakeClusterPropagationPolicies) Create(ctx context.Context, clusterPropagationPolicy *v1.ClusterPropagationPolicy, opts metav1.CreateOptions) (result *v1.ClusterPropagationPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(clusterpropagationpoliciesResource, clusterPropagationPolicy), &v1.ClusterPropagationPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterPropagationPolicy), err
}

// Update takes the representation of a clusterPropagationPolicy and updates it. Returns the server's representation of the clusterPropagationPolicy, and an error, if there is any.
func (c *FakeClusterPropagationPolicies) Update(ctx context.Context, clusterPropagationPolicy *v1.ClusterPropagationPolicy, opts metav1.UpdateOptions) (result *v1.ClusterPropagationPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(clusterpropagationpoliciesResource, clusterPropagationPolicy), &v1.ClusterPropagationPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterPropagationPolicy), err
}

// Delete takes name of the clusterPropagationPolicy and deletes it. Returns an error if one occurs.
func (c *FakeClusterPropagationPolicies) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(clusterpropagationpoliciesResource, name, opts), &v1.ClusterPropagationPolicy{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusterPropagationPolicies) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(clusterpropagationpoliciesResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1.ClusterPropagationPolicyList{})
	return err
}

// Patch applies the patch and returns the patched clusterPropagationPolicy.
func (c *FakeClusterPropagationPolicies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.ClusterPropagationPolicy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(clusterpropagationpoliciesResource, name, pt, data, subresources...), &v1.ClusterPropagationPolicy{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ClusterPropagationPolicy), err
}
