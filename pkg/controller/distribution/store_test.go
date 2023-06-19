package distribution

import (
	"context"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"testing"
)

func TestStore(t *testing.T) {
	var store = NewDistributionStore()
	store.StoreResourcePointToDistribution("apps/v1alpha1/namespace/deployments/nginx", "policy1")
	store.StoreResourcePointToDistribution("apps/v1alpha1/namespace/deployments/nginx", "policy2")
	store.StoreResourcePointToDistribution("apps/v1alpha1/namespace/deployments/nginx", "policy3")
	store.StoreResourcePointToDistribution("apps/v1alpha1/namespace/deployments/xxxx", "policy9")
	store.StoreResourcePointToDistribution("apps/v1alpha1/namespace/deployments/xxxx", "policy10")

	//store.StoreDistributionPointToResource("policy1", "apps/v1alpha1/namespace/deployments/nginx")
	//resource := store.GetDistributionsByResource("apps/v1alpha1/namespace/deployments/nginx")
	//log.Print("resource:",resource)
	//byResource := store.IsExistDistributionByResource("apps/v1alpha1/namespace/deployments/nginx")
	//log.Print("exist:",byResource)
	//distribution, ok := store.GetResourceByDistribution("policy1")
	//if !ok {
	//	log.Print("not exist")
	//	return
	//}
	//log.Print("distribution:",distribution)

	distributions := store.GetAllDistributions()
	log.Print("distributions:", distributions)

}

func TestClient(t *testing.T) {

	cfg := config.GetConfigOrDie()

	c, err := client.New(cfg, client.Options{})
	if err != nil {
		log.Fatal(err)
	}

	// deploy list
	deployList := &v1.DeploymentList{}

	selector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"rd":  "aaa",
			"env": "test",
		},
	}

	asSelector, err := metav1.LabelSelectorAsSelector(&selector)
	if err != nil {
		log.Fatal(err)
	}

	err = c.List(context.Background(), deployList, &client.ListOptions{
		LabelSelector: asSelector,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, deploy := range deployList.Items {
		t.Log(deploy.Name)
	}

}
