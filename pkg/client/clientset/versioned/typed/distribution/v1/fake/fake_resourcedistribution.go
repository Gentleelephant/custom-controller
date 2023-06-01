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

// FakeResourceDistributions implements ResourceDistributionInterface
type FakeResourceDistributions struct {
	Fake *FakeDistributionV1
	ns   string
}

var resourcedistributionsResource = v1.SchemeGroupVersion.WithResource("resourcedistributions")

var resourcedistributionsKind = v1.SchemeGroupVersion.WithKind("ResourceDistribution")

// Get takes name of the resourceDistribution, and returns the corresponding resourceDistribution object, and an error if there is any.
func (c *FakeResourceDistributions) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.ResourceDistribution, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(resourcedistributionsResource, c.ns, name), &v1.ResourceDistribution{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ResourceDistribution), err
}

// List takes label and field selectors, and returns the list of ResourceDistributions that match those selectors.
func (c *FakeResourceDistributions) List(ctx context.Context, opts metav1.ListOptions) (result *v1.ResourceDistributionList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(resourcedistributionsResource, resourcedistributionsKind, c.ns, opts), &v1.ResourceDistributionList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ResourceDistributionList{ListMeta: obj.(*v1.ResourceDistributionList).ListMeta}
	for _, item := range obj.(*v1.ResourceDistributionList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested resourceDistributions.
func (c *FakeResourceDistributions) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(resourcedistributionsResource, c.ns, opts))

}

// Create takes the representation of a resourceDistribution and creates it.  Returns the server's representation of the resourceDistribution, and an error, if there is any.
func (c *FakeResourceDistributions) Create(ctx context.Context, resourceDistribution *v1.ResourceDistribution, opts metav1.CreateOptions) (result *v1.ResourceDistribution, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(resourcedistributionsResource, c.ns, resourceDistribution), &v1.ResourceDistribution{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ResourceDistribution), err
}

// Update takes the representation of a resourceDistribution and updates it. Returns the server's representation of the resourceDistribution, and an error, if there is any.
func (c *FakeResourceDistributions) Update(ctx context.Context, resourceDistribution *v1.ResourceDistribution, opts metav1.UpdateOptions) (result *v1.ResourceDistribution, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(resourcedistributionsResource, c.ns, resourceDistribution), &v1.ResourceDistribution{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ResourceDistribution), err
}

// Delete takes name of the resourceDistribution and deletes it. Returns an error if one occurs.
func (c *FakeResourceDistributions) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(resourcedistributionsResource, c.ns, name, opts), &v1.ResourceDistribution{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeResourceDistributions) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(resourcedistributionsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1.ResourceDistributionList{})
	return err
}

// Patch applies the patch and returns the patched resourceDistribution.
func (c *FakeResourceDistributions) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.ResourceDistribution, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(resourcedistributionsResource, c.ns, name, pt, data, subresources...), &v1.ResourceDistribution{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.ResourceDistribution), err
}