package serve

import (
	"github.com/Gentleelephant/custom-controller/pkg/api"
	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/policy/v1"
	"github.com/Gentleelephant/custom-controller/pkg/controller/policy"
	"k8s.io/apimachinery/pkg/runtime"
	"log"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	scheme = runtime.NewScheme()
)

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

	err = ctrl.NewControllerManagedBy(mgr).
		For(&v1.PropagationPolicy{}).
		Complete(&policy.PropagationReconciler{Client: mgr.GetClient()})
	if err != nil {
		log.Fatalln("create controller failed", err)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		os.Exit(1)
	}

	c := make(chan struct{})
	err = mgr.Start(ctrl.SetupSignalHandler())
	if err != nil {
		log.Fatalln(err, "unable to start manager")
	}
	<-c
}
