package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// ResourceKindWork is kind name of Work.
	ResourceKindWork = "Work"
	// ResourceSingularWork is singular name of Work.
	ResourceSingularWork = "work"
	// ResourcePluralWork is plural name of Work.
	ResourcePluralWork = "works"
	// ResourceNamespaceScopedWork indicates if Work is NamespaceScoped.
	ResourceNamespaceScopedWork = true
)

// +genclient
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Workload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec represents the desired behavior of Work.
	Spec WorkloadSpec `json:"spec"`

	// Status represents the status of PropagationStatus.
	Status WorkloadStatus `json:"status,omitempty"`
}

// WorkSpec defines the desired state of Work.
type WorkloadSpec struct {
	Manifest Manifest `json:"manifests,omitempty"`
}

// Manifest represents a resource to be deployed on managed cluster.
type Manifest struct {
	// +kubebuilder:pruning:PreserveUnknownFields
	runtime.RawExtension `json:",inline"`
}

// WorkStatus defines the observed state of Work.
type WorkloadStatus struct {
	// ManifestStatuses contains running status of manifests in spec.
	// +optional
	ManifestStatuses ManifestStatus `json:"manifestStatuses,omitempty"`
}

// ManifestStatus contains running status of a specific manifest in spec.
type ManifestStatus struct {
	// Status reflects running status of current manifest.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Resource *runtime.RawExtension `json:"resource,omitempty"`

	Time metav1.Time `json:"time,omitempty"`

	Status string `json:"status,omitempty"`

	Message string `json:"message,omitempty"`
}

const (
	WorkFailed string = "Failed"

	WorkUnknown string = "Unknown"

	WorkSucceeded string = "Succeeded"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkList is a collection of Work.
type WorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items holds a list of Work.
	Items []Workload `json:"items"`
}
