package distribution

//
//import (
//	"github.com/Gentleelephant/custom-controller/pkg/utils"
//	"github.com/Gentleelephant/custom-controller/pkg/utils/keys"
//	corev1 "k8s.io/api/core/v1"
//	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
//	"k8s.io/client-go/tools/cache"
//	"k8s.io/klog/v2"
//)
//
//type ClusterEventHandler struct {
//	ClusterName string `json:"clusterName"`
//
//	Controller *WorkloadController `json:"controller"`
//}
//
//func (c *ClusterEventHandler) GetClusterResourceKey(obj interface{}) (string, error) {
//	key, err := keys.ClusterWideKeyFunc(obj)
//	if err != nil {
//		return "", err
//	}
//	wideKeyToString := keys.WideKeyToString(key)
//	clusterResourceKey := c.ClusterName + "/" + wideKeyToString
//	return clusterResourceKey, nil
//}
//
//func (c *ClusterEventHandler) OnAdd(obj interface{}) {
//	return
//}
//
//func (c *ClusterEventHandler) OnUpdate(oldObj, newObj interface{}) {
//
//	clusterResourceKey, err := c.GetClusterResourceKey(newObj)
//	if err != nil {
//		klog.Error("GetClusterResourceKey error:", err)
//		return
//	}
//
//	unstructuredOldObj, err := utils.ToUnstructured(oldObj)
//	if err != nil {
//		klog.Errorf("Failed to transform oldObj, error: %v", err)
//		return
//	}
//	unstructuredNewObj, err := utils.ToUnstructured(newObj)
//	if err != nil {
//		klog.Errorf("Failed to transform newObj, error: %v", err)
//		return
//	}
//
//	if !utils.SpecificationChanged(unstructuredOldObj, unstructuredNewObj) {
//		klog.V(4).Infof("Ignore update event of object (kind=%s, %s/%s) as specification no change", unstructuredOldObj.GetKind(), unstructuredOldObj.GetNamespace(), unstructuredOldObj.GetName())
//		return
//	}
//
//	workloadKey, ok := c.Controller.Store.GetWorkloadByResource(clusterResourceKey)
//	if !ok {
//		klog.Error("Get workload namespace/name from store error")
//		return
//	}
//
//	klog.Info("member cluster resource on update")
//
//	c.Controller.Workqueue.Add(workloadKey)
//}
//
//func (c *ClusterEventHandler) OnDelete(obj interface{}) {
//	klog.Info("member cluster resource on delete:")
//	// member集群上的资源删除，找到对应的workload，然后通知
//	clusterResourceKey, err := c.GetClusterResourceKey(obj)
//	if err != nil {
//		klog.Error("GetClusterResourceKey error:", err)
//		return
//	}
//	workloadKey, ok := c.Controller.Store.GetWorkloadByResource(clusterResourceKey)
//	if !ok {
//		klog.Error("Get workload namespace/name from store error")
//		return
//	}
//	c.Controller.Workqueue.Add(workloadKey)
//}
//
//func (c *ClusterEventHandler) EventFilter(obj interface{}) bool {
//	key, err := keys.ClusterWideKeyFunc(obj)
//	if err != nil {
//		return false
//	}
//	if keys.IsReservedNamespace(key.Namespace) {
//		return false
//	}
//	// if SkippedPropagatingNamespaces is set, skip object events in these namespaces.
//	//if _, ok := c.Controller.SkippedPropagatingNamespaces[clusterWideKey.Namespace]; ok {
//	//	return false
//	//}
//	if unstructObj, ok := obj.(*unstructured.Unstructured); ok {
//		switch unstructObj.GroupVersionKind() {
//		case corev1.SchemeGroupVersion.WithKind("Secret"):
//			secretType, found, _ := unstructured.NestedString(unstructObj.Object, "type")
//			if found && secretType == string(corev1.SecretTypeServiceAccountToken) {
//				return false
//			}
//		}
//	}
//
//	//判断这个资源和Workload是否对应
//	wideKeyToString := keys.WideKeyToString(key)
//	clusterResourceKey := c.ClusterName + "/" + wideKeyToString
//	_, ok := c.Controller.Store.GetWorkloadByResource(clusterResourceKey)
//	if ok {
//		//klog.Info("查找到匹配的workload:", wl)
//		return true
//	}
//	return false
//}
//
//func (c *ClusterEventHandler) NewResourceEventHandler() cache.ResourceEventHandler {
//	return &cache.FilteringResourceEventHandler{
//		FilterFunc: c.EventFilter,
//		Handler: cache.ResourceEventHandlerFuncs{
//			AddFunc:    c.OnAdd,
//			UpdateFunc: c.OnUpdate,
//			DeleteFunc: c.OnDelete,
//		},
//	}
//}
