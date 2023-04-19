package policy

import (
	"context"
	"fmt"
	"github.com/Gentleelephant/custom-controller/pkg/apis/cluster/v1alpha1"
	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/policy/v1"
	wv1 "github.com/Gentleelephant/custom-controller/pkg/apis/work/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/dynamic"
	schema "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PropagationReconciler struct {
	client.Client
	// DynamicClient used to fetch arbitrary resources.
	dynamicClient dynamic.Interface
}

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=policy.kubesphere.io,resources=propagationpolicies;resourcebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=policy.kubesphere.io,resources=propagationpolicies/status;resourcebindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=policy.kubesphere.io,resources=propagationpolicies/finalizers;resourcebindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get

func (r *PropagationReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	policy := &v1.PropagationPolicy{}
	err := r.Get(ctx, req.NamespacedName, policy)
	if errors.IsNotFound(err) {
		klog.Error("Propagation not found", err)
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not fetch ReplicaSet: %+v", err)
	}

	if !policy.DeletionTimestamp.IsZero() {
		klog.V(4).Infof("Begin to delete works owned by binding(%s).", req.NamespacedName.String())
	}

	err = r.createResourceBinding(ctx, policy)
	if err != nil {
		klog.Error(err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil

}

// TODO
func (r *PropagationReconciler) removeFinalizer(pr *v1.PropagationPolicy) (controllerruntime.Result, error) {
	return reconcile.Result{}, nil
}

func (r *PropagationReconciler) createResourceBinding(ctx context.Context, pr *v1.PropagationPolicy) error {

	// TODO: 根据propagation的策略创建resourcebinding
	var resourceBindings []*wv1.ResourceBinding
	var target []wv1.TargetCluster
	// cluster name 方式
	for _, cluster := range pr.Spec.Placement.ClusterAffinity.ClusterNames {
		target = append(target, wv1.TargetCluster{
			Name: cluster,
		})
	}

	if pr.Spec.Placement.ClusterAffinity.LabelSelector != nil {
		// labe方式查询cluster的name
		var clusterList v1alpha1.ClusterList
		err := r.List(ctx, &clusterList, client.MatchingLabels(pr.Spec.Placement.ClusterAffinity.LabelSelector.MatchLabels))
		if err != nil {
			return err
		}
		for _, cluster := range clusterList.Items {
			klog.Infof("cluster name: %s", cluster.Name)
			target = append(target, wv1.TargetCluster{
				Name: cluster.Name,
			})
		}
	}
	// 遍历所有的资源选择器，创建resourcebinding，方式有两种，根据name和根据label
	for _, se := range pr.Spec.ResourceSelectors {
		klog.Infof("resource selector: %+v", se)
		temp := &wv1.ResourceBinding{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ResourceBinding",
				APIVersion: "work.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      se.Name,
				Namespace: se.Namespace,
			},
			Spec: wv1.ResourceBindingSpec{
				Resource: wv1.ObjectReference{
					APIVersion:      se.APIVersion,
					Kind:            se.Kind,
					Namespace:       se.Namespace,
					Name:            se.Name,
					ResourceVersion: string(uuid.NewUUID()),
				},
				Clusters: target,
			},
		}
		resourceBindings = append(resourceBindings, temp)
	}
	// 如果不存在则创建，如果存在则更新
	for _, rb := range resourceBindings {
		fmt.Printf("resourcebinding: %+v", rb)
		var rtb wv1.ResourceBinding
		err := r.Get(ctx, types.NamespacedName{Name: rb.Name, Namespace: pr.Namespace}, &rtb)
		if errors.IsNotFound(err) {
			err = r.Create(ctx, rb)
			if err != nil {
				return err
			}
		}
		rtb = *rb
		err = r.Update(ctx, &rtb)
		if err != nil {
			return err
		}
		err = controllerutil.SetControllerReference(pr, &rtb, schema.Scheme)
		if err != nil {
			return err
		}
	}
	return nil
}
