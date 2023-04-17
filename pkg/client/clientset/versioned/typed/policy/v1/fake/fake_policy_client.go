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
	v1 "github.com/Gentleelephant/custom-controller/pkg/client/clientset/versioned/typed/policy/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakePolicyV1 struct {
	*testing.Fake
}

func (c *FakePolicyV1) ClusterOverridePolicies() v1.ClusterOverridePolicyInterface {
	return &FakeClusterOverridePolicies{c}
}

func (c *FakePolicyV1) OverridePolicies(namespace string) v1.OverridePolicyInterface {
	return &FakeOverridePolicies{c, namespace}
}

func (c *FakePolicyV1) Propagations(namespace string) v1.PropagationInterface {
	return &FakePropagations{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakePolicyV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
