package distribution

import (
	"github.com/Gentleelephant/custom-controller/pkg/utils"
	"github.com/Gentleelephant/custom-controller/pkg/utils/keys"
	"github.com/duke-git/lancet/v2/maputil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

type BindObject struct {

	// 触发同步的ResourceDistribution
	RdWidekey []keys.ClusterWideKey

	// 被同步对象
	Obj interface{}
}

type EventHandler struct {
	controller *DistributionController
}

func (c *EventHandler) OnAdd(obj interface{}) {

	wideKeys, b := c.EventFilter(obj)
	if !b {
		return
	}
	object := BindObject{
		RdWidekey: wideKeys,
		Obj:       obj,
	}

	// 将对象放入channel
	c.controller.cre <- object

	return
}

func (c *EventHandler) OnUpdate(oldObj, newObj interface{}) {

	wideKeys, b := c.EventFilter(oldObj)
	if !b {
		return
	}

	unstructuredOldObj, err := utils.ToUnstructured(oldObj)
	if err != nil {
		klog.Errorf("Failed to transform oldObj, error: %v", err)
		return
	}
	unstructuredNewObj, err := utils.ToUnstructured(newObj)
	if err != nil {
		klog.Errorf("Failed to transform newObj, error: %v", err)
		return
	}
	if !utils.SpecificationChanged(unstructuredOldObj, unstructuredNewObj) {
		klog.V(4).Infof("Ignore update event of object (kind=%s, %s/%s) as specification no change", unstructuredOldObj.GetKind(), unstructuredOldObj.GetNamespace(), unstructuredOldObj.GetName())
		return
	}

	object := BindObject{
		RdWidekey: wideKeys,
		Obj:       newObj,
	}

	// 将对象放入channel
	c.controller.cre <- object

}

func (c *EventHandler) OnDelete(obj interface{}) {

	wideKeys, b := c.EventFilter(obj)
	if !b {
		return
	}

	for _, key := range wideKeys {
		toString := keys.WideKeyToString(key)
		klog.Info("删除资源对应的Rules:", toString)
		rules, exist := c.controller.RuleStore.Get(toString)
		if exist {
			ruleNames := maputil.Keys(rules)
			for _, ruleName := range ruleNames {
				c.controller.del <- ruleName
			}
		}
	}

}

func (c *EventHandler) EventFilter(obj interface{}) ([]keys.ClusterWideKey, bool) {
	key, err := keys.ClusterWideKeyFunc(obj)
	if err != nil {
		return nil, false
	}
	if keys.IsReservedNamespace(key.Namespace) {
		return nil, false
	}
	if unstructObj, ok := obj.(*unstructured.Unstructured); ok {
		switch unstructObj.GroupVersionKind() {
		case corev1.SchemeGroupVersion.WithKind("Secret"):
			secretType, found, _ := unstructured.NestedString(unstructObj.Object, "type")
			if found && secretType == string(corev1.SecretTypeServiceAccountToken) {
				return nil, false
			}
		}
	}

	templates := c.controller.Store.GetAllTemplates()

	var namespaceKeys []keys.ClusterWideKey
	var flag bool

	for _, template := range templates {
		if keys.PrefixMatch(key, template) {
			klog.Info("前缀匹配成功：", key, template)
			namespaceKeys = append(namespaceKeys, template)
			flag = true
		}
	}

	return namespaceKeys, flag
}
