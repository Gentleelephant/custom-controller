package distribution

import (
	"github.com/Gentleelephant/custom-controller/pkg/utils"
	"github.com/Gentleelephant/custom-controller/pkg/utils/keys"
	"github.com/duke-git/lancet/v2/maputil"
	"k8s.io/klog/v2"
)

type BindObject struct {

	// 触发同步的ResourceDistribution
	RdNamespaceKey []string

	// 被同步对象
	Obj interface{}
}

type DeleteObject struct {
	RdNamespaceKey []string

	// 需要删除的Workload
	WorkloadName []string

	RuleId []string
}

type EventHandler struct {
	controller *DistributionController
}

func NewEventHandler(c *DistributionController) *EventHandler {
	return &EventHandler{
		controller: c,
	}
}

func (c *EventHandler) OnAdd(obj interface{}) {

	wideKeys, b := c.EventFilter(obj)
	if !b {
		return
	}
	object := &BindObject{
		RdNamespaceKey: wideKeys,
		Obj:            obj,
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

	klog.Info("关联资源更新...")

	object := &BindObject{
		RdNamespaceKey: wideKeys,
		Obj:            newObj,
	}

	// 将对象放入channel
	c.controller.cre <- object

}

func (c *EventHandler) OnDelete(obj interface{}) {

	namespaceKeys, b := c.EventFilter(obj)
	if !b {
		return
	}

	clusterWideKey, err := keys.ClusterWideKeyFunc(obj)
	if err != nil {
		klog.Errorf("Failed to get cluster wide key of object (kind=%s, %s/%s), error: %v", obj, err)
		return
	}

	var deleteWorkload []string

	for _, namespaceKey := range namespaceKeys {
		rules, exist := c.controller.RuleStore.Get(namespaceKey)
		if exist {
			ids := maputil.Keys(rules)
			for _, i := range ids {
				workloadName := getWorkloadName(&clusterWideKey, i, namespaceKey)
				deleteWorkload = append(deleteWorkload, workloadName)
			}
		}
	}

	object := &DeleteObject{
		RdNamespaceKey: nil,
		WorkloadName:   deleteWorkload,
		RuleId:         nil,
	}

	c.controller.del <- object

}

func (c *EventHandler) EventFilter(obj interface{}) ([]string, bool) {
	key, err := keys.ClusterWideKeyFunc(obj)
	if err != nil {
		return nil, false
	}
	if keys.IsReservedNamespace(key.Namespace) {
		return nil, false
	}

	templates := c.controller.Store.GetAllTemplates()

	var namespaceKeys []string
	var flag bool

	for _, template := range templates {
		// 如果模板中的Name不为空，则需要全等匹配
		if template.Name != "" {
			if template == key {
				distributions := c.controller.Store.GetDistributions(template)
				namespaceKeys = append(namespaceKeys, distributions...)
				flag = true
			}
			continue
		}
		// 如果模板中的Name为空，则需要其他字段匹配
		if template.Group == key.Group && template.Version == key.Version && template.Kind == key.Kind && template.Namespace == key.Namespace {
			// 匹配上
			klog.Info("其他字段匹配成功：", key, template)
			// TODO : 根据模板找到对应RD
			distributions := c.controller.Store.GetDistributions(template)
			namespaceKeys = append(namespaceKeys, distributions...)
			flag = true
		}
	}
	return namespaceKeys, flag
}
