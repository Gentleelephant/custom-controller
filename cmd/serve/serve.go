package serve

import (
	"github.com/Gentleelephant/custom-controller/pkg/api"
	v1 "github.com/Gentleelephant/custom-controller/pkg/apis/policy/v1"
	"github.com/Gentleelephant/custom-controller/pkg/controller/policy"
	"k8s.io/apimachinery/pkg/runtime"
	"log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
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

	mgr, err := manager.New(cfg, manager.Options{
		Scheme: scheme,
		Port:   9096,
	})
	err = api.AddToScheme(mgr.GetScheme())
	if err != nil {
		log.Fatalln(err, "unable to add scheme")
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&v1.PropagationPolicy{}).
		Complete(&policy.PropagationReconciler{Client: mgr.GetClient()})
	if err != nil {
		log.Fatalln("create controller failed", err)
	}

	c := make(chan struct{})
	err = mgr.Start(ctrl.SetupSignalHandler())
	if err != nil {
		log.Fatalln(err, "unable to start manager")
	}
	<-c
}
