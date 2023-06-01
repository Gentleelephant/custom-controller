package distribution

import (
	"github.com/Gentleelephant/custom-controller/pkg/utils/keys"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type ClusterEventHandler struct {
	ClusterName string `json:"clusterName"`

	Controller *WorkloadController `json:"controller"`
}

func (c *ClusterEventHandler) GetClusterResourceKey(obj interface{}) (string, error) {
	key, err := ClusterWideKeyFunc(obj)
	if err != nil {
		return "", err
	}
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.Errorf("Invalid key")
		return "", err
	}
	wideKeyToString := WideKeyToString(clusterWideKey)
	clusterResourceKey := c.ClusterName + "/" + wideKeyToString
	return clusterResourceKey, nil
}

func (c *ClusterEventHandler) OnAdd(obj interface{}) {
	return
}

func (c *ClusterEventHandler) OnUpdate(oldObj, newObj interface{}) {
	//klog.Info("ClusterEventHandler OnUpdate")
	//// member集群上的资源更新，找到对应的workload，然后通知
	//clusterResourceKey,err := c.GetClusterResourceKey(newObj)
	//if err != nil {
	//	klog.Error("GetClusterResourceKey error:",err)
	//	return
	//}
	//wls, ok := c.Controller.Store.Get(clusterResourceKey)
	//if !ok {
	//	return
	//}
	//workloads, result := wls.([]string)
	//if !result {
	//	return
	//}
	//for _, workload := range workloads {
	//	c.Controller.Enqueue(workload)
	//}
	return
}

func (c *ClusterEventHandler) OnDelete(obj interface{}) {
	klog.Info("ClusterEventHandler OnDelete")
	// member集群上的资源删除，找到对应的workload，然后通知
	clusterResourceKey, err := c.GetClusterResourceKey(obj)
	if err != nil {
		klog.Error("GetClusterResourceKey error:", err)
		return
	}
	klog.Info("member集群上的资源删除:", clusterResourceKey)
	wl, ok := c.Controller.Store.Get(clusterResourceKey)
	if !ok {
		klog.Error("Get workload namespace/name from store error")
		return
	}
	workload, result := wl.(string)
	if !result {
		klog.Error("transform workload namespace/name key error")
		return
	}
	klog.Info("查找到对应的workload:", workload)
	c.Controller.Workqueue.Add(workload)
}

func (c *ClusterEventHandler) EventFilter(obj interface{}) bool {
	key, err := ClusterWideKeyFunc(obj)
	if err != nil {
		return false
	}
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.Errorf("Invalid key")
		return false
	}
	if IsReservedNamespace(clusterWideKey.Namespace) {
		return false
	}
	// if SkippedPropagatingNamespaces is set, skip object events in these namespaces.
	//if _, ok := c.Controller.SkippedPropagatingNamespaces[clusterWideKey.Namespace]; ok {
	//	return false
	//}

	if unstructObj, ok := obj.(*unstructured.Unstructured); ok {
		switch unstructObj.GroupVersionKind() {
		// The secret, with type 'kubernetes.io/service-account-token', is created along with `ServiceAccount` should be
		// prevented from propagating.
		// Refer to https://github.com/karmada-io/karmada/pull/1525#issuecomment-1091030659 for more details.
		case corev1.SchemeGroupVersion.WithKind("Secret"):
			secretType, found, _ := unstructured.NestedString(unstructObj.Object, "type")
			if found && secretType == string(corev1.SecretTypeServiceAccountToken) {
				return false
			}
		}
	}

	//判断这个资源和RD是否对应
	wideKeyToString := WideKeyToString(clusterWideKey)
	clusterResourceKey := c.ClusterName + "/" + wideKeyToString
	klog.Infof("filter clusterResourceKey:", clusterResourceKey)

	wls, ok := c.Controller.Store.Get(clusterResourceKey)
	if ok {
		workloads, result := wls.(string)
		if !result {
			klog.Errorf("Invalid workloads")
			return false
		}
		if len(workloads) > 0 {
			return true
		}
	}

	return true
}

func (c *ClusterEventHandler) NewResourceEventHandler() cache.ResourceEventHandler {
	return &cache.FilteringResourceEventHandler{
		FilterFunc: c.EventFilter,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.OnAdd,
			UpdateFunc: c.OnUpdate,
			DeleteFunc: c.OnDelete,
		},
	}
}
