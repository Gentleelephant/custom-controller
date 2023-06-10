package serve

import (
	"context"
	"github.com/Gentleelephant/custom-controller/pkg/api"
	clientset "github.com/Gentleelephant/custom-controller/pkg/client/clientset/versioned"
	informers "github.com/Gentleelephant/custom-controller/pkg/client/informers/externalversions"
	"github.com/Gentleelephant/custom-controller/pkg/controller/distribution"
	webhook2 "github.com/Gentleelephant/custom-controller/pkg/webhook"
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
	"sigs.k8s.io/controller-runtime/pkg/webhook"
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

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())

	distributionController := distribution.NewDistributionController(ctx,
		mgr.GetClient(),
		kubeClient,
		clientsets,
		mgr.GetScheme(),
		mgr.GetRESTMapper(),
		dynamicClient,
		*discoveryClient,
		informerFactory.Distribution().V1().ResourceDistributions(),
		informerFactory.Cluster().V1alpha1().Clusters(),
	)

	//workloadController := distribution.NewController(ctx,
	//	mgr.GetClient(),
	//	kubeClient,
	//	clientsets,
	//	mgr.GetScheme(),
	//	informerFactory.Cluster().V1alpha1().Clusters(),
	//	mgr.GetRESTMapper(),
	//	informerFactory.Distribution().V1().Workloads())

	informerFactory.Start(ctx.Done())

	err = mgr.Add(distributionController)
	if err != nil {
		log.Fatalln(err)
		return
	}

	// Add Webhook
	mgr.GetWebhookServer().Register("/mutate-v1-rd", &webhook.Admission{
		Handler: &webhook2.ResourceDistributionWebhook{
			Client: mgr.GetClient(),
		},
	})

	//err = mgr.Add(workloadController)
	//if err != nil {
	//	log.Fatalln(err)
	//	return
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
