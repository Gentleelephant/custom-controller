package distribution

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Gentleelephant/custom-controller/pkg/apis/cluster/v1alpha1"
	"github.com/Gentleelephant/custom-controller/pkg/apis/distribution"
	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/distribution/v1"
	"github.com/duke-git/lancet/v2/random"
	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sort"
	"strings"
)

type Reconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	RestMapper    meta.RESTMapper
	DynamicClient dynamic.Interface
	Cache         cache.Cache
}

type overrideOption struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	policy := v1.ResourceDistribution{}
	err := r.Client.Get(ctx, req.NamespacedName, &policy)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		klog.Error(err)
		return ctrl.Result{}, err
	}
	// Check if the policy instance is marked to be deleted
	if policy.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&policy, Finalizer) {
			policy.ObjectMeta.Finalizers = append(policy.ObjectMeta.Finalizers, Finalizer)
			if err = r.updateExternalResources(context.Background(), &policy); err != nil {
				klog.Error(err)
				return ctrl.Result{Requeue: true}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(&policy, Finalizer) {
			// our finalizer is present, so lets handle any external dependency
			// before deleting the policy
			if err = r.deleteExternalResources(ctx, &policy); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return ctrl.Result{}, err
			}
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&policy, Finalizer)
			if err = r.Update(ctx, &policy); err != nil {
				klog.Error(err)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	go r.watchObject(ctx, &policy)

	//err = r.applyWorks(ctx, &policy)
	//if err != nil {
	//	klog.Error(err)
	//	return ctrl.Result{}, err
	//}
	return reconcile.Result{}, nil
}

func (r *Reconciler) watchObject(ctx context.Context, p *v1.ResourceDistribution) {
	klog.Info("===========>watch object")
	groupVersionKind := schema.FromAPIVersionAndKind(p.Spec.ResourceSelectors.APIVersion, p.Spec.ResourceSelectors.Kind)
	informer, err := r.Cache.GetInformerForKind(ctx, groupVersionKind)
	klog.Error("informer is nil:", informer == nil)
	if err != nil {
		klog.Error("get informer error:", err)
		return
	}

	var handler = toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Info("==============add================")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			klog.Info("==============update================")
		},
		DeleteFunc: func(obj interface{}) {
			klog.Info("==============delete================")
		},
	}
	informer.AddEventHandler(handler)
}

func (r *Reconciler) deleteExternalResources(ctx context.Context, p *v1.ResourceDistribution) error {
	workList := v1.WorkloadList{}
	err := r.List(ctx, &workList, client.MatchingLabels{ResourceDistributionPolicy: p.Name})
	if err != nil {
		klog.Error(err)
		return err
	}
	for _, work := range workList.Items {
		err = r.Delete(ctx, &work)
		if err != nil {
			klog.Error(err)
			return err
		}
	}
	return nil
}

func (r *Reconciler) updateExternalResources(ctx context.Context, p *v1.ResourceDistribution) error {

	p.Annotations[ResourceDistributionPolicy] = p.Name
	p.Annotations[ResourceDistributionId] = random.RandLower(8)
	over := p.Spec.OverrideRules
	if len(over) > 0 {
		for i := range over {
			over[i].Id = random.RandLower(8)
		}
	}
	p.Spec.OverrideRules = over
	err := r.Update(ctx, p)
	if err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (r *Reconciler) applyWorks(ctx context.Context, policy *v1.ResourceDistribution) error {
	works, err := r.generateWorks(ctx, policy)
	if err != nil {
		klog.Error(err)
		return nil
	}
	for _, work := range works {
		var workObj v1.Workload
		err = r.Get(ctx, types.NamespacedName{Name: work.Name, Namespace: work.Namespace}, &workObj)
		if err != nil {
			if errors.IsNotFound(err) {
				err = r.Create(ctx, &work)
				if err != nil {
					klog.Error(err)
				}
				continue
			}
		}
		if !reflect.DeepEqual(workObj.Spec, work.Spec) {
			workObj.Spec = work.Spec
			err = r.Update(ctx, &workObj)
			if err != nil {
				klog.Error(err)
			}
			return err
		}
	}
	return err
}

func (r *Reconciler) generateWorks(ctx context.Context, policy *v1.ResourceDistribution) ([]v1.Workload, error) {
	clusterName, err := r.getClusterName(ctx, policy)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	overrideRules := policy.Spec.OverrideRules
	unstructObjArr, err := fetchResourceTemplate(r.DynamicClient, r.RestMapper, policy)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	var works []v1.Workload
	var arrObj []unstructured.Unstructured
	for _, item := range overrideRules {
		var clusterSlice []string
		for _, unstructObj := range unstructObjArr {
			deepCopyObj := unstructObj.DeepCopy()
			options := getOverrideOptions(&item)
			err = applyJSONPatch(deepCopyObj, options)
			if err != nil {
				klog.Error("apply json patch error:", err)
				return nil, err
			}

			var byLabel []string
			byLabel, err = r.getClusterNameByLabelSelector(item.TargetCluster.LabelSelector)
			if err != nil {
				return nil, err
			}
			clusterSlice = append(clusterSlice, byLabel...)
			clusterSlice = append(clusterSlice, item.TargetCluster.ClusterNames...)
			if err != nil {
				klog.Error(err)
				return nil, err
			}
			arrObj = append(arrObj, *deepCopyObj)
		}
		work, err := r.createWork(policy, arrObj, clusterSlice, item.Id)
		if err != nil {
			klog.Error(err)
			return nil, err
		}
		// 将生成的work，存到works中
		works = append(works, *work)
	}
	if len(clusterName) > 0 {
		pid := policy.Annotations[ResourceDistributionId]
		work, err := r.createWork(policy, unstructObjArr, clusterName, pid)
		if err != nil {
			klog.Error("create work error:", err)
			return nil, err
		}
		works = append(works, *work)
	}
	//}
	return works, err
}

func (r *Reconciler) getClusterNameByLabelSelector(selector *metav1.LabelSelector) ([]string, error) {
	if selector == nil {
		return nil, nil
	}
	clusterList := v1alpha1.ClusterList{}
	err := r.List(context.Background(), &clusterList, client.MatchingLabels(selector.MatchLabels))
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	var target []string
	for _, cluster := range clusterList.Items {
		target = append(target, cluster.Name)
	}
	return target, nil
}

func (r *Reconciler) getClusterName(ctx context.Context, pr *v1.ResourceDistribution) ([]string, error) {
	var target []string
	if pr.Spec.Placement.ClusterAffinity != nil {
		for _, cluster := range pr.Spec.Placement.ClusterAffinity.ClusterNames {
			target = append(target, cluster)
		}
		if pr.Spec.Placement.ClusterAffinity.LabelSelector != nil {
			var clusterList v1alpha1.ClusterList
			err := r.List(ctx, &clusterList, client.MatchingLabels(pr.Spec.Placement.ClusterAffinity.LabelSelector.MatchLabels))
			if err != nil {
				klog.Error(err)
				return nil, err
			}
			for _, cluster := range clusterList.Items {
				target = append(target, cluster.Name)
			}
		}
	}
	return target, nil
}

func applyJSONPatch(obj *unstructured.Unstructured, overrides []overrideOption) error {
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

func fetchResourceTemplate(dynamicClient dynamic.Interface, restMapper meta.RESTMapper, policy *v1.ResourceDistribution) ([]unstructured.Unstructured, error) {

	var unobjArr []unstructured.Unstructured
	gvr, err := GetGroupVersionResource(restMapper, schema.FromAPIVersionAndKind(policy.Spec.ResourceSelectors.APIVersion, policy.Spec.ResourceSelectors.Kind))
	if err != nil {
		return nil, err
	}

	ns := policy.Spec.ResourceSelectors.Namespace
	name := policy.Spec.ResourceSelectors.Name
	kind := policy.Spec.ResourceSelectors.Kind
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

func GetGroupVersionResource(restMapper meta.RESTMapper, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	restMapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return restMapping.Resource, nil
}

func ToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	uncastObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: uncastObj}, nil
}

// create work
func (r *Reconciler) createWork(policy *v1.ResourceDistribution, unstructured []unstructured.Unstructured, clusters []string, id string) (*v1.Workload, error) {

	sort.Strings(clusters)
	work := v1.Workload{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Work",
			APIVersion: distribution.GroupName + "/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      policy.GetName() + "-" + id,
			Namespace: policy.GetNamespace(),
			Annotations: map[string]string{
				SyncCluster:                strings.Join(clusters, ","),
				ResourceDistributionPolicy: policy.GetName(),
				ResourceDistributionId:     policy.Annotations[ResourceDistributionId],
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(policy.GetObjectMeta(), policy.GroupVersionKind()),
			},
		},
		//Spec: v1.WorkloadSpec{
		//	WorkloadTemplate: v1.WorkloadTemplate{
		//		Clusters: clusters,
		//	},
		//},
	}

	for _, u := range unstructured {
		deepCopy := u.DeepCopy()
		marshalJSON, err := deepCopy.MarshalJSON()
		if err != nil {
			klog.Error(err)
			return nil, err
		}
		work.Spec.WorkloadTemplate.Manifests = append(work.Spec.WorkloadTemplate.Manifests, v1.Manifest{
			RawExtension: runtime.RawExtension{
				Raw: marshalJSON,
			},
		})
	}

	return &work, nil
}

func getOverrideOptions(overrideRules *v1.RuleWithCluster) []overrideOption {
	plaintext := overrideRules.Overriders.Plaintext
	var overrideOptions []overrideOption
	for i := range plaintext {
		var temp overrideOption
		temp.Path = plaintext[i].Path
		temp.Value = plaintext[i].Value
		temp.Op = string(plaintext[i].Operator)
		overrideOptions = append(overrideOptions, temp)
	}
	return overrideOptions
}

func (r *Reconciler) createWorkWithCluster(ctx context.Context, work *v1.Workload, clusters []string) {

	for _, cluster := range clusters {
		var clusterObj = v1alpha1.Cluster{}
		err := r.Get(ctx, types.NamespacedName{Name: cluster}, &clusterObj)
		if err != nil {
			klog.Error(err)
		}
		// 创建client
		kubeconfig := clusterObj.Spec.Connection.KubeConfig
		config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
		if err != nil {
			klog.Error("Failed to create rest config for cluster %s, Error: %v", cluster, err)
		}
		tempClient, err := client.New(config, client.Options{})
		if err != nil {
			klog.Error("Failed to create client for cluster %s, Error: %v", cluster, err)
		}
		err = tempClient.Create(ctx, work)
		if err != nil {
			klog.Error("Failed to create work for cluster %s, Error: %v", cluster, err)
		}
	}
}
