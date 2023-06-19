package v1alpha1

import (
	"github.com/Gentleelephant/custom-controller/pkg/apis/distribution"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: distribution.GroupName, Version: "v1alpha1"}

var RDGVK = schema.GroupVersionKind{
	Group:   distribution.GroupName,
	Version: "v1alpha1",
	Kind:    "ResourceDistribution",
}

var Clusterv1alpha1GVK = schema.GroupVersionKind{
	Group:   "cluster.kubesphere.io",
	Version: "v1alpha1",
	Kind:    "Cluster",
}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	// SchemeBuilder initializes a scheme builder
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme is a global function that registers this API group & version to a scheme
	// 这个函数是全局的，它将这个资源的API组和版本注册到一个scheme中
	AddToScheme = SchemeBuilder.AddToScheme
)

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&ResourceDistribution{},
		&ResourceDistributionList{},
		&Workload{},
		&WorkloadList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
