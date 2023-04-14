package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PropagationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS -- desired state of cluster
	// Define a field called "Name" of type string in the FooSpec struct
	// +required
	// +kubebuilder:validation:MinItems=1
	ResourceSelectors []ResourceSelector `json:"resourceSelectors"`

	// +optional
	Placement Placement `json:"placement,omitempty"`
}

// FooStatus defines the observed state of Foo.
// It should always be reconstructable from the state of the cluster and/or outside world.
type PropagationStatus struct {
	// INSERT ADDITIONAL STATUS FIELDS -- observed state of cluster
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Propagation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PropagationSpec   `json:"spec,omitempty"`
	Status PropagationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PropagationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Propagation `json:"items"`
}

// ResourceSelector the resources will be selected.
type ResourceSelector struct {
	// APIVersion represents the API version of the target resources.
	// +required
	APIVersion string `json:"apiVersion"`

	// Kind represents the Kind of the target resources.
	// +required
	Kind string `json:"kind"`

	// Namespace of the target resource.
	// Default is empty, which means inherit from the parent object scope.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name of the target resource.
	// Default is empty, which means selecting all resources.
	// +optional
	Name string `json:"name,omitempty"`

	// A label query over a set of resources.
	// If name is not empty, labelSelector will be ignored.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

type Placement struct {
	// +optional
	ClusterAffinity *ClusterAffinity `json:"clusterAffinity,omitempty"`
}

// ClusterAffinity represents the filter to select clusters.
type ClusterAffinity struct {
	// LabelSelector is a filter to select member clusters by labels.
	// If non-nil and non-empty, only the clusters match this filter will be selected.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// ClusterNames is the list of clusters to be selected.
	// +optional
	ClusterNames []string `json:"clusterNames,omitempty"`

	// ExcludedClusters is the list of clusters to be ignored.
	// +optional
	ExcludeClusters []string `json:"exclude,omitempty"`
}
