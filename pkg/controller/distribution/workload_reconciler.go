package distribution

import (
	"context"
	"github.com/Gentleelephant/custom-controller/pkg/apis/cluster/v1alpha1"
	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/distribution/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type WorkReconciler struct {
	client.Client
}

func (r *WorkReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {

	klog.Info("reconcile work...")
	w := &v1.Workload{}
	err := r.Get(ctx, req.NamespacedName, w)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	if w.ObjectMeta.DeletionTimestamp.IsZero() {
		// 如果当前对象没有 finalizer， 说明其没有处于正被删除的状态。
		// 接着让我们添加 finalizer 并更新对象，相当于注册我们的 finalizer。
		if !controllerutil.ContainsFinalizer(w, SyncFinalize) {
			w.ObjectMeta.Finalizers = append(w.ObjectMeta.Finalizers, SyncFinalize)
			if err := r.Update(context.Background(), w); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// 这个对象将要被删除
		if controllerutil.ContainsFinalizer(w, SyncFinalize) {
			// 我们的 finalizer 就在这, 接下来就是处理外部依赖
			if err := r.deleteExternalResources(w); err != nil {
				// 如果无法在此处删除外部依赖项，则返回错误
				// 以便可以重试
				return ctrl.Result{}, err
			}
			// 从列表中删除我们的 finalizer 并进行更新。
			controllerutil.RemoveFinalizer(w, SyncFinalize)
			if err := r.Update(context.Background(), w); err != nil {
				return ctrl.Result{}, err
			}
		}
		// 当它们被删除的时候停止 reconciliation
		return ctrl.Result{}, nil
	}

	r.syncWork(ctx, w)
	return reconcile.Result{}, nil
}

func (r *WorkReconciler) deleteExternalResources(w *v1.Workload) error {
	// 获取集群客户端
	klog.Error("删除同步的资源", "work", w.Name)
	clients := r.getClusterClient(context.Background(), w)
	workload := &unstructured.Unstructured{}
	manifests := w.Spec.WorkloadTemplate.Manifests
	for _, manifest := range manifests {
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Error(err)
		}
		for _, c := range clients {
			err = c.Delete(context.Background(), workload)
			if err != nil {
				if errors.IsNotFound(err) {
					continue
				}
				klog.Error(err)
			}
		}
	}
	return nil
}

func (r *WorkReconciler) syncWork(ctx context.Context, work *v1.Workload) {
	workload := &unstructured.Unstructured{}
	manifests := work.Spec.WorkloadTemplate.Manifests
	//message := v1.SyncMessage{}
	// 获取集群客户端
	clients := r.getClusterClient(ctx, work)
	for _, manifest := range manifests {
		err := workload.UnmarshalJSON(manifest.Raw)
		if err != nil {
			klog.Errorf("Failed to unmarshal workload, error is: %v", err)
		}
		for _, c := range clients {
			dpwl := workload.DeepCopy()
			err = c.Get(ctx, client.ObjectKeyFromObject(dpwl), dpwl)
			if err != nil {
				if errors.IsNotFound(err) {
					workload.SetResourceVersion("")
					err = c.Create(ctx, workload)
					if err != nil {
						klog.Errorf("failed to create workload", err)
					}
					continue
				}
				klog.Error("get workload failed:", err)
				continue
			}
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if !reflect.DeepEqual(dpwl.Object["spec"], workload.Object["spec"]) {
					dpwl.Object["spec"] = workload.Object["spec"]
					err = c.Update(ctx, dpwl)
					if err != nil {
						klog.Error("update workload failed:", err)
						return err
					}
				}
				return nil
			})
			if err != nil {
				klog.Error("update workload failed:", err)
			}
		}
	}
}

func (r *WorkReconciler) getClusterClient(ctx context.Context, work *v1.Workload) map[string]client.Client {
	var clientsByName = make(map[string]client.Client)
	clustersName := work.Spec.WorkloadTemplate.Clusters
	for _, c := range clustersName {
		cluster := v1alpha1.Cluster{}
		err := r.Get(ctx, client.ObjectKey{Name: c}, &cluster)
		if err != nil {
			klog.Error(err)
			continue
		}
		config := string(cluster.Spec.Connection.KubeConfig)
		restConfigFromKubeConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(config))
		if err != nil {
			klog.Error("failed to get rest config from kubeconfig: ", err)
		}
		ct, err := client.New(restConfigFromKubeConfig, client.Options{})
		if err != nil {
			klog.Error("failed to create ct: ", err)
		}
		clientsByName[c] = ct
	}
	return clientsByName
}
