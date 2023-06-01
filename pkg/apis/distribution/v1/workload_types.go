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
	// Workload represents the manifest workload to be deployed on managed cluster.
	WorkloadTemplate WorkloadTemplate `json:"workloadTemplate,omitempty"`
}

// WorkloadTemplate represents the manifest workload to be deployed on managed cluster.
type WorkloadTemplate struct {
	// Manifests represents a list of Kubernetes resources to be deployed on the managed cluster.
	// +optional
	Manifests []Manifest `json:"manifests,omitempty"`
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
	ManifestStatuses []Manifest `json:"manifests,omitempty"`

	// Identifier represents the identity of a resource linking to manifests in spec.
	// +required
	//Identifier ResourceIdentifier `json:"identifier"`
}

// ManifestStatus contains running status of a specific manifest in spec.
type ManifestStatus struct {
	// Status reflects running status of current manifest.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Status *runtime.RawExtension `json:"status,omitempty"`
}

// ResourceIdentifier provides the identifiers needed to interact with any arbitrary object.
type ResourceIdentifier struct {
	//// Ordinal represents an index in manifests list, so the condition can still be linked
	//// to a manifest even though manifest cannot be parsed successfully.
	//Ordinal int `json:"ordinal"`

	// Group is the group of the resource.
	Group string `json:"group,omitempty"`

	// Version is the version of the resource.
	Version string `json:"version"`

	// Kind is the kind of the resource.
	Kind string `json:"kind"`

	// Resource is the resource type of the resource
	Resource string `json:"resource"`

	// Namespace is the namespace of the resource, the resource is cluster scoped if the value
	// is empty
	Namespace string `json:"namespace,omitempty"`

	// Name is the name of the resource
	Name string `json:"name"`
}

type ErrorMessage struct {
	Cluster string `json:"clusters,omitempty"`

	Message string `json:"message,omitempty"`
}

const (
	// WorkApplied represents that the resource defined in Work is
	// successfully applied on the managed cluster.
	WorkApplied string = "Applied"
	// WorkProgressing represents that the resource defined in Work is
	// in the progress to be applied on the managed cluster.
	WorkProgressing string = "Progressing"
	// WorkAvailable represents that all resources of the Work exists on
	// the managed cluster.
	WorkAvailable string = "Available"
	// WorkDegraded represents that the current state of Work does not match
	// the desired state for a certain period.
	WorkDegraded string = "Degraded"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkList is a collection of Work.
type WorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items holds a list of Work.
	Items []Workload `json:"items"`
}
