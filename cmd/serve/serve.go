package serve

import (
	"context"
	"github.com/Gentleelephant/custom-controller/pkg/api"
	clientset "github.com/Gentleelephant/custom-controller/pkg/client/clientset/versioned"
	informers "github.com/Gentleelephant/custom-controller/pkg/client/informers/externalversions"
	"github.com/Gentleelephant/custom-controller/pkg/controller/distribution"
	"github.com/Gentleelephant/custom-controller/pkg/utils/genericmanager"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"log"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"time"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func Start() {

	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalln(err)
	}

	mgr, err := ctrl.NewManager(cfg, manager.Options{
		Scheme:                 scheme,
		Port:                   9096,
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		log.Fatalln(err, "==>unable to create manager")
	}
	err = api.AddToScheme(mgr.GetScheme())
	if err != nil {
		log.Fatalln(err, "==>unable to add scheme")
	}

	dynamicClient, err := dynamic.NewForConfig(cfg)

	clientsets, err := clientset.NewForConfig(cfg)
	if err != nil {
		return
	}

	ctx := context.Background()

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	informerFactory := informers.NewSharedInformerFactory(clientsets, time.Second*30)
	controlPlaneInformerManager := genericmanager.NewSingleClusterInformerManager(dynamicClient, 0, ctx.Done())
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())

	distributionController := distribution.NewDistributionController(ctx,
		mgr.GetClient(),
		kubeClient,
		mgr.GetScheme(),
		mgr.GetRESTMapper(),
		dynamicClient,
		*discoveryClient,
		informerFactory.Distribution().V1().ResourceDistributions(),
		informerFactory.Cluster().V1alpha1().Clusters(),
		controlPlaneInformerManager,
	)

	workloadController := distribution.NewController(ctx, mgr.GetClient(), kubeClient, mgr.GetScheme(), informerFactory.Distribution().V1().Workloads())

	informerFactory.Start(ctx.Done())

	err = mgr.Add(distributionController)
	if err != nil {
		log.Fatalln(err)
		return
	}

	err = mgr.Add(workloadController)
	if err != nil {
		log.Fatalln(err)
		return
	}

	//c, err := controller.New("resource-distribution", mgr, controller.Options{
	//	Reconciler: &distribution.Reconciler{Client: mgr.GetClient(),
	//		RestMapper:    mgr.GetRESTMapper(),
	//		Scheme:        mgr.GetScheme(),
	//		DynamicClient: dynamic.NewForConfigOrDie(cfg),
	//		Cache:         mgr.GetCache(),
	//	},
	//})
	//if err != nil {
	//	klog.Error("create controller error", "error", err)
	//	return
	//}

	//err = c.Watch(&source.Kind{Type: &v1.ResourceDistribution{}}, &handler.EnqueueRequestForObject{})
	//if err != nil {
	//	klog.Error("watch resource distribution error", "error", err)
	//	return
	//}
	//err = c.Watch(&source.Kind{Type: &v1alpha1.Cluster{}}, &handler.EnqueueRequestForObject{})
	//if err != nil {
	//	klog.Error("watch cluster error", "error", err)
	//	return
	//}

	//err = ctrl.NewControllerManagedBy(mgr).
	//	For(&v1.ResourceDistribution{}).
	//	Watches(&source.Kind{Type: &v1alpha1.Cluster{}}, &handler.EnqueueRequestForObject{}).
	//	Complete(&distribution.Reconciler{Client: mgr.GetClient(),
	//		RestMapper:    mgr.GetRESTMapper(),
	//		Scheme:        mgr.GetScheme(),
	//		DynamicClient: dynamic.NewForConfigOrDie(cfg),
	//	})
	//if err != nil {
	//	log.Fatalln("create controller failed", err)
	//}

	//err = ctrl.NewControllerManagedBy(mgr).
	//	For(&v1.Workload{}).
	//	Complete(&distribution.WorkReconciler{Client: mgr.GetClient()})
	//if err != nil {
	//	log.Fatalln("create work controller failed", err)
	//}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		os.Exit(1)
	}

	ct := make(chan struct{})
	err = mgr.Start(ctrl.SetupSignalHandler())
	if err != nil {
		log.Fatalln(err, "unable to start manager")
	}
	<-ct
}
