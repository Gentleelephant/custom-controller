package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceDistributionSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS -- desired state of cluster
	// Define a field called "Name" of type string in the FooSpec struct
	// +required
	ResourceSelectors ResourceSelector `json:"resourceSelectors"`

	// +optional
	Placement Placement `json:"placement,omitempty"`

	// +optional
	OverrideRules []RuleWithCluster `json:"overrideRules,omitempty"`
}

// It should always be reconstructable from the state of the cluster and/or outside world.
type ResourceDistributionMessage map[string]ClusterSyncStatus

//type SyncStatus struct {
//	// INSERT ADDITIONAL STATUS FIELDS -- observed state of cluster
//	Status SyncMessage `json:"clusterSyncStatuses,omitempty"`
//}

type ClusterSyncStatus struct {
	// 同步时间
	Datetime *metav1.Time `json:"datetime,omitempty"`

	// 同步状态
	Status string `json:"status,omitempty"`

	// 同步信息
	Message string `json:"message,omitempty"`
}

// +genclient
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ResourceDistribution struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceDistributionSpec    `json:"spec,omitempty"`
	Status ResourceDistributionMessage `json:"status,omitempty"`
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

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterSyncPolicy represents the cluster-wide policy that propagates a group of resources to one or more clusters.
// Different with SyncPolicy that could only propagate resources in its own namespace, ClusterSyncPolicy
// is able to propagate cluster level resources and resources in any namespace other than system reserved ones.
// System reserved namespaces are: karmada-system, karmada-cluster, karmada-es-*.
type ClusterResourceDistribution struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of ClusterSyncPolicy.
	// +required
	Spec ResourceDistribution `json:"spec"`

	Status ResourceDistributionMessage `json:"status,omitempty"`
}

// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterSyncPolicyList contains a list of ClusterSyncPolicy.
type ClusterResourceDistributionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterResourceDistribution `json:"items"`
}
