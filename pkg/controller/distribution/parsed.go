package distribution

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Gentleelephant/custom-controller/pkg/apis/cluster/v1alpha1"
	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/distribution/v1"
	"github.com/Gentleelephant/custom-controller/pkg/constant"
	"github.com/Gentleelephant/custom-controller/pkg/utils"
	"github.com/duke-git/lancet/v2/cryptor"
	set "github.com/duke-git/lancet/v2/datastructure/set"
	"github.com/duke-git/lancet/v2/slice"
	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type overrideOption struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type ParsedOverrideRules struct {
	Id string `json:"id"`

	Clusters []string `json:"clusters"`

	OverrideOptions []overrideOption `json:"overrideOptions"`
}

type ParsedOverrideRulesStore struct {
	// 存储rd->ParsedOverrideRules
	store map[string]map[string]ParsedOverrideRules
}

func (d *ParsedOverrideRulesStore) Store(namespaceKey string, object ParsedOverrideRules) {
	m, ok := d.store[namespaceKey]
	if !ok {
		value := make(map[string]ParsedOverrideRules)
		value[object.Id] = object
		d.store[namespaceKey] = value
		return
	}
	m[object.Id] = object
	d.store[namespaceKey] = m
}

func (d *ParsedOverrideRulesStore) StoreMap(namespaceKey string, m map[string]ParsedOverrideRules) {
	d.store[namespaceKey] = m
}

func (d *ParsedOverrideRulesStore) Get(namespaceKey string) (map[string]ParsedOverrideRules, bool) {
	m, ok := d.store[namespaceKey]
	return m, ok
}

func (d *ParsedOverrideRulesStore) Delete(namespaceKey, name string) {
	delete(d.store[namespaceKey], name)
}

func (d *ParsedOverrideRulesStore) DeleteAll(namespaceKey string) {
	// 是否需要提供回调，删除DistributionObject的同时对应的workload也要删除
	delete(d.store, namespaceKey)
}

func NewParsedOverrideRulesStore() *ParsedOverrideRulesStore {
	return &ParsedOverrideRulesStore{
		store: make(map[string]map[string]ParsedOverrideRules),
	}
}

// 解析ResourceDistribution
func ParseResourceDistribution(ctx context.Context, client ctrlclient.Client, rd *v1.ResourceDistribution) (map[string]ParsedOverrideRules, error) {
	m := make(map[string]ParsedOverrideRules)
	overrideRules := rd.Spec.OverrideRules
	clusters, err := parseRdClusterName(ctx, client, rd)
	if err != nil {
		return nil, err
	}
	if overrideRules == nil || len(overrideRules) == 0 {
		temp := ParsedOverrideRules{
			Clusters:        clusters,
			OverrideOptions: nil,
			Id:              rd.Name,
		}
		m[rd.Name] = temp
	}
	var usedCluster []string
	for _, item := range overrideRules {
		var clusterSlice []string
		var overrideOptions []overrideOption
		if item.TargetCluster == nil {
			clusterSlice = clusters
		} else {
			selector, err := parseClustersByLabelSelector(ctx, client, item.TargetCluster.LabelSelector)
			if err != nil {
				klog.Error(err)
				return nil, err
			}
			clusterSlice = append(clusterSlice, selector...)
			clusterSlice = append(clusterSlice, item.TargetCluster.ClusterNames...)
			if err != nil {
				klog.Error(err)
				return nil, err
			}
		}
		// 如果clusterSlices中存在不属于placement定义的clustername，需要将这部分排除
		intersection := slice.Intersection(clusters, clusterSlice)
		overriders := item.Overriders
		if overriders.Plaintext != nil || len(overriders.Plaintext) > 0 {
			for _, plaintext := range overriders.Plaintext {
				overrideOptions = append(overrideOptions, overrideOption{
					Op:    string(plaintext.Operator),
					Path:  plaintext.Path,
					Value: plaintext.Value,
				})
			}
		}
		parseRule := ParsedOverrideRules{
			Clusters:        intersection,
			OverrideOptions: overrideOptions,
			Id:              item.Id,
		}
		m[item.Id] = parseRule
		usedCluster = append(usedCluster, intersection...)
	}
	// 将useCluster去重，再和clusterPlacement比较，如果存在不同的，说明还有需要同步的不用override的集群
	useClusterSet := set.NewSetFromSlice(usedCluster)
	clusterPlacementSet := set.NewSetFromSlice(clusters)
	minusSet := clusterPlacementSet.Minus(useClusterSet)
	residue := minusSet.Values()
	if len(residue) > 0 {
		temp := ParsedOverrideRules{
			Clusters:        residue,
			OverrideOptions: nil,
			Id:              rd.Name,
		}
		m[rd.Name] = temp
	}
	return m, nil
}

func parseRdClusterName(ctx context.Context, client ctrlclient.Client, rd *v1.ResourceDistribution) ([]string, error) {
	var target []string
	// 如果没有指定cluster，那么就是所有的cluster
	if rd.Spec.Placement == nil || rd.Spec.Placement.ClusterAffinity == nil {
		var clusterList v1alpha1.ClusterList
		err := client.List(ctx, &clusterList)
		if err != nil {
			return nil, err
		}
		for _, cluster := range clusterList.Items {
			// 如果是主集群，不需要同步
			labels := cluster.Labels
			if labels != nil {
				_, ok := labels[constant.HostCluster]
				if ok {
					continue
				}
			}
			target = append(target, cluster.Name)
		}
		return target, nil
	}
	c := rd.Spec.Placement.ClusterAffinity.ClusterNames
	if c != nil {
		target = append(target, c...)
	}
	if rd.Spec.Placement.ClusterAffinity.LabelSelector != nil {
		var clusterList v1alpha1.ClusterList
		s, err := metav1.LabelSelectorAsSelector(rd.Spec.Placement.ClusterAffinity.LabelSelector)
		if err != nil {
			return nil, err
		}
		err = client.List(ctx, &clusterList, &ctrlclient.ListOptions{
			LabelSelector: s,
		})
		if err != nil {
			klog.Error(err)
			return nil, err
		}
		for _, cluster := range clusterList.Items {
			target = append(target, cluster.Name)
		}
	}
	return target, nil
}

func parseClustersByLabelSelector(ctx context.Context, client ctrlclient.Client, labelSelector *metav1.LabelSelector) ([]string, error) {
	if labelSelector == nil {
		return []string{}, nil
	}
	var target []string
	var clusterList v1alpha1.ClusterList
	s, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil, err
	}
	err = client.List(ctx, &clusterList, &ctrlclient.ListOptions{
		LabelSelector: s,
	})
	if err != nil {
		return nil, err
	}
	for _, cluster := range clusterList.Items {
		target = append(target, cluster.Name)
	}
	return target, nil
}

// 应用override规则
func ApplyOverrideRules(object interface{}, rule ParsedOverrideRules) (*unstructured.Unstructured, error) {
	unObj, err := utils.ToUnstructured(object)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	err = ApplyJSONPatchs(unObj, rule.OverrideOptions)
	if err != nil {
		return nil, err
	}

	unObj.SetAnnotations(map[string]string{
		constant.SyncCluster: strings.Join(rule.Clusters, ","),
	})

	return unObj, nil
}

func ApplyJSONPatchs(obj *unstructured.Unstructured, overrides []overrideOption) error {
	if obj == nil {
		return fmt.Errorf("object is nil")
	}
	if overrides == nil || len(overrides) == 0 {
		return nil
	}
	jsonPatchBytes, err := json.Marshal(overrides)
	if err != nil {
		return err
	}
	patch, err := jsonpatch.DecodePatch(jsonPatchBytes)
	if err != nil {
		return err
	}
	objectJSONBytes, err := obj.MarshalJSON()
	if err != nil {
		return err
	}
	patchedObjectJSONBytes, err := patch.Apply(objectJSONBytes)
	if err != nil {
		return err
	}
	err = obj.UnmarshalJSON(patchedObjectJSONBytes)
	if err != nil {
		return err
	}
	return nil
}

func fetchResourceTemplateByRD(dynamicClient dynamic.Interface, restMapper meta.RESTMapper, rd *v1.ResourceDistribution) ([]unstructured.Unstructured, error) {

	var unobjArr []unstructured.Unstructured
	gvr, err := getGroupVersionResource(restMapper, schema.FromAPIVersionAndKind(rd.Spec.ResourceSelector.APIVersion, rd.Spec.ResourceSelector.Kind))
	if err != nil {
		return nil, err
	}
	ns := rd.Spec.ResourceSelector.Namespace
	name := rd.Spec.ResourceSelector.Name
	kind := rd.Spec.ResourceSelector.Kind
	if kind == "" {
		err = fmt.Errorf("kind is empty")
		return nil, err
	}

	if name != "" {
		obj, err := dynamicClient.Resource(gvr).Namespace(ns).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		unobjArr = append(unobjArr, *obj)
	} else {
		lists, err := dynamicClient.Resource(gvr).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, item := range lists.Items {
			unobjArr = append(unobjArr, item)
		}
	}
	if err != nil {
		klog.Error("Failed to transform object(%s/%s), Error: %v", ns, name, err)
		return nil, err
	}
	return unobjArr, nil
}

func generateName(namespaceKey string, ruleName string) string {
	hmacSha256 := cryptor.HmacSha256(ruleName, namespaceKey)
	key := hmacSha256[0:8]
	return key
}
