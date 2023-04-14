package serve

import (
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	scheme = runtime.NewScheme()
)

func Start() {

	//获得k8s config,然后创建client
	//cfg, err := clientcmd.BuildConfigFromFlags("", "/Users/zhangpeng/.kube/config")
	//if err != nil {
	//	log.Println("load outside config error")
	//	inClusterConfig, err := rest.InClusterConfig()
	//	if err != nil {
	//		log.Fatalln(err)
	//	}
	//	cfg = inClusterConfig
	//}
	//log.Println("load config success")
	//kubeClientset, err := kubernetes.NewForConfig(cfg)
	//if err != nil {
	//	log.Fatalln(err)
	//}
	//cfg := config.GetConfigOrDie()
	//exampleClientset, err := clientset.NewForConfig(cfg)
	//if err != nil {
	//	log.Fatalln(err)
	//}
	////kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClientset, 0)
	//exampleInformerFactory := exampleinformers.NewSharedInformerFactory(exampleClientset, 0)
	//
	//mgr, err := manager.New(cfg, manager.Options{
	//	Scheme: scheme,
	//	Port:   9096,
	//})
	//if err != nil {
	//	log.Fatalln(err, "unable to create manager")
	//}
	//
	//err = api.AddToScheme(mgr.GetScheme())
	//if err != nil {
	//	log.Fatalln(err, "unable to add scheme")
	//}
	//
	//err = mgr.Add(propagation.NewController(mgr.GetCache(), exampleClientset, exampleInformerFactory.Customcontroller().V1().Foos()))
	//if err != nil {
	//	log.Fatalln(err, "unable to add controller")
	//}
	//
	//err = mgr.Start(ctrl.SetupSignalHandler())
	//if err != nil {
	//	log.Fatalln(err, "unable to start manager")
	//}

}
