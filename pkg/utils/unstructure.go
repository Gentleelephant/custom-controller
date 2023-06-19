package utils

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
)

// ToUnstructured converts a typed object to an unstructured object.
func ToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	uncastObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: uncastObj}, nil
}

// SpecificationChanged check if the specification of the given object change or not
func SpecificationChanged(oldObj, newObj *unstructured.Unstructured) bool {
	oldBackup := oldObj.DeepCopy()
	newBackup := newObj.DeepCopy()

	// Remove the status and some system defined mutable fields in metadata, including managedFields and resourceVersion.
	// Refer to https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/object-meta/#ObjectMeta for more details.
	removedFields := [][]string{{"status"}, {"metadata", "resourceVersion"}}
	for _, r := range removedFields {
		unstructured.RemoveNestedField(oldBackup.Object, r...)
		unstructured.RemoveNestedField(newBackup.Object, r...)
	}

	return !reflect.DeepEqual(oldBackup, newBackup)
}

// GetGroupVersionResource is a helper to map GVK(schema.GroupVersionKind) to GVR(schema.GroupVersionResource).
func GetGroupVersionResource(restMapper meta.RESTMapper, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	restMapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return restMapping.Resource, nil
}
