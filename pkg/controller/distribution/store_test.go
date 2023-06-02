package distribution

import (
	"log"
	"testing"
)

func TestStore(t *testing.T) {
	var store = NewDistributionStore()
	store.StoreResourcePointToDistribution("apps/v1/namespace/deployments/nginx", "policy1")
	store.StoreResourcePointToDistribution("apps/v1/namespace/deployments/nginx", "policy2")
	store.StoreResourcePointToDistribution("apps/v1/namespace/deployments/nginx", "policy3")
	store.StoreResourcePointToDistribution("apps/v1/namespace/deployments/xxxx", "policy9")
	store.StoreResourcePointToDistribution("apps/v1/namespace/deployments/xxxx", "policy10")

	//store.StoreDistributionPointToResource("policy1", "apps/v1/namespace/deployments/nginx")
	//resource := store.GetDistributionsByResource("apps/v1/namespace/deployments/nginx")
	//log.Print("resource:",resource)
	//byResource := store.IsExistDistributionByResource("apps/v1/namespace/deployments/nginx")
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
