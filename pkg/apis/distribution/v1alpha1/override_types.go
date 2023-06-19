package v1alpha1

import (
	"github.com/duke-git/lancet/v2/random"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const (
	// ResourceKindOverridePolicy is kind name of OverridePolicy.
	ResourceKindOverridePolicy = "OverridePolicy"
	// ResourceSingularOverridePolicy is singular name of OverridePolicy.
	ResourceSingularOverridePolicy = "overridepolicy"
	// ResourcePluralOverridePolicy is plural name of OverridePolicy.
	ResourcePluralOverridePolicy = "overridepolicies"
	// ResourceNamespaceScopedOverridePolicy indicates if OverridePolicy is NamespaceScoped.
	ResourceNamespaceScopedOverridePolicy = true

	// ResourceKindClusterOverridePolicy is kind name of ClusterOverridePolicy.
	ResourceKindClusterOverridePolicy = "ClusterOverridePolicy"
	// ResourceSingularClusterOverridePolicy is singular name of ClusterOverridePolicy.
	ResourceSingularClusterOverridePolicy = "clusteroverridepolicy"
	// ResourcePluralClusterOverridePolicy is kind plural of ClusterOverridePolicy.
	ResourcePluralClusterOverridePolicy = "clusteroverridepolicies"
	// ResourceNamespaceScopedClusterOverridePolicy indicates if ClusterOverridePolicy is NamespaceScoped.
	ResourceNamespaceScopedClusterOverridePolicy = false
)

// OverridePolicy represents the policy that overrides a group of resources to one or more clusters.
//type OverridePolicy struct {
//	metav1.TypeMeta   `json:",inline"`
//	metav1.ObjectMeta `json:"metadata,omitempty"`
//
//	// Spec represents the desired behavior of OverridePolicy.
//	Spec OverrideSpec `json:"spec"`
//}
//
//// OverrideSpec defines the desired behavior of OverridePolicy.
//type OverrideSpec struct {
//	// OverrideRules defines a collection of override rules on target clusters.
//	OverrideRules RuleWithCluster `json:"overrideRules,omitempty"`
//}

// RuleWithCluster defines the override rules on clusters.
type RuleWithCluster struct {
	Id string `json:"id,omitempty"`
	// TargetCluster defines restrictions on this override policy
	// that only applies to resources propagated to the matching clusters.
	// nil means matching all clusters.
	TargetCluster *ClusterAffinity `json:"targetCluster,omitempty"`

	// Overriders represents the override rules that would apply on resources
	Overriders Overriders `json:"overriders"`
}

func ApplyRuleName(rules []RuleWithCluster) []RuleWithCluster {
	for i := range rules {
		if rules[i].Id == "" {
			rules[i].Id = random.RandLower(8)
		}
	}
	return rules
}

// Overriders offers various alternatives to represent the override rules.
//
// If more than one alternative exists, they will be applied with following order:
// - ImageOverrider
// - CommandOverrider
// - ArgsOverrider
// - LabelsOverrider
// - AnnotationsOverrider
// - Plaintext
type Overriders struct {
	// Plaintext represents override rules defined with plaintext overriders.
	// +optional
	Plaintext []PlaintextOverrider `json:"plaintext,omitempty"`
}

// PlaintextOverrider is a simple overrider that overrides target fields
// according to path, operator and value.
type PlaintextOverrider struct {
	// Path indicates the path of target field
	Path string `json:"path"`
	// Operator indicates the operation on target field.
	// Available operators are: add, replace and remove.
	// +kubebuilder:validation:Enum=add;remove;replace
	Operator OverriderOperator `json:"operator"`
	// Value to be applied to target field.
	// Must be empty when operator is Remove.
	// +optional
	Value apiextensionsv1.JSON `json:"value,omitempty"`
}

// OverriderOperator is the set of operators that can be used in an overrider.
type OverriderOperator string

// These are valid overrider operators.
const (
	OverriderOpAdd     OverriderOperator = "add"
	OverriderOpRemove  OverriderOperator = "remove"
	OverriderOpReplace OverriderOperator = "replace"
)
