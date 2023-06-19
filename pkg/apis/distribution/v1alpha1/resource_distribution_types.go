package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceDistributionSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS -- desired state of cluster
	// Define a field called "Name" of type string in the FooSpec struct
	// +required
	ResourceSelector ResourceSelector `json:"resourceSelector"`

	// +optional
	Placement *Placement `json:"placement,omitempty"`

	// +optional
	OverrideRules []RuleWithCluster `json:"overrideRules,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=resourcedistributions,singular=resourcedistribution,scope=Cluster,shortName=rd
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ResourceDistribution struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ResourceDistributionSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ResourceDistributionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceDistribution `json:"items"`
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

	// LabelSelector is a filter to select resources by labels.
	// If non-nil and non-empty, only the resources match this filter will be selected.
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
}
