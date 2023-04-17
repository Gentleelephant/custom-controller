package policy

import (
	"context"
	"fmt"
	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/policy/v1"
	wv1 "github.com/Gentleelephant/custom-controller/pkg/apis/work/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	schema "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PropagationReconciler struct {
	client.Client
}

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
	klog.Infof("Propagation: %s/%s, %d replicas, status: %+v", policy.Namespace, policy.Name, policy.Spec, policy.Status)

	if !policy.DeletionTimestamp.IsZero() {
		klog.V(4).Infof("Begin to delete works owned by binding(%s).", req.NamespacedName.String())
	}

	err = r.syncPropagation(ctx, policy)
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

func (r *PropagationReconciler) syncPropagation(ctx context.Context, pr *v1.PropagationPolicy) error {

	// TODO: 根据propagation的策略创建resourcebinding
	var resourceBindings []*wv1.ResourceBinding
	for _, se := range pr.Spec.ResourceSelectors {
		var target []wv1.TargetCluster
		for _, cluster := range pr.Spec.Placement.ClusterAffinity.ClusterNames {
			target = append(target, wv1.TargetCluster{
				Name: cluster,
			})
		}
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
		var rtb wv1.ResourceBinding
		err := r.Get(ctx, types.NamespacedName{Name: rb.Name, Namespace: rb.Namespace}, &rtb)
		if errors.IsNotFound(err) {
			err = r.Create(ctx, rb)
			if err != nil {
				return err
			}
		}
		err = r.Update(ctx, rb)
		if err != nil {
			return err
		}
		err = controllerutil.SetOwnerReference(&v1.PropagationPolicy{}, &wv1.ResourceBinding{}, schema.Scheme)
		if err != nil {
			return err
		}
	}
	return nil
}
