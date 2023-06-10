package distribution

import (
	"github.com/Gentleelephant/custom-controller/pkg/utils/keys"
	"github.com/duke-git/lancet/v2/maputil"
	"github.com/duke-git/lancet/v2/slice"
	kvcache "github.com/patrickmn/go-cache"
	"sync"
)

type DistributionStore struct {
	store *kvcache.Cache
}

func (d DistributionStore) StoreResourcePointToDistribution(key, value string) {
	m := make(map[string]struct{})
	resul, exist := d.store.Get(key)
	if !exist {
		m[value] = struct{}{}
		d.store.Set(key, m, kvcache.NoExpiration)
	} else {
		mps, ok := resul.(map[string]struct{})
		if !ok {
			return
		}
		mps[value] = struct{}{}
		d.store.Set(key, mps, kvcache.NoExpiration)
	}
}

// StoreDistributionPointToResource policy1 -> apps/v1/namespace/deployments/nginx
func (d DistributionStore) StoreDistributionPointToResource(key, value string) {
	d.store.Set(key, value, kvcache.NoExpiration)
}

// GetResourceByDistribution Find the resource pointed to by distribution
func (d DistributionStore) GetResourceByDistribution(key string) (string, bool) {
	result, exist := d.store.Get(key)
	if !exist {
		return "", false
	}
	s, ok := result.(string)
	if !ok {
		return "", false
	}
	return s, true
}

// GetDistributionsByResource apps/v1/namespace/deployments/nginx  -> policy1,policy2,policy3
func (d DistributionStore) GetDistributionsByResource(key string) []string {
	result, exist := d.store.Get(key)
	if !exist {
		return nil
	}
	m, ok := result.(map[string]struct{})
	if !ok {
		return nil
	}
	return maputil.Keys(m)
}

// IsExistDistributionByResource 根据资源找到判断是否存在对应的分发策略
func (d DistributionStore) IsExistDistributionByResource(key string) bool {
	result, exist := d.store.Get(key)
	if !exist {
		return false
	}
	m, ok := result.(map[string]struct{})
	if !ok {
		return false
	}
	return len(m) > 0
}

// RemoveDistributionByResource 根据资源找到对应的分发策略并删除
// apps/v1/namespace/deployments/nginx  -> policy1,policy2,policy3
// 删除 policy1
func (d DistributionStore) RemoveDistributionByResource(key, value string) {
	result, exist := d.store.Get(key)
	if !exist {
		return
	}
	m, ok := result.(map[string]struct{})
	if !ok {
		return
	}
	delete(m, value)
	d.store.Set(key, m, kvcache.NoExpiration)
}

// 返回所有的distribution
func (d DistributionStore) GetAllDistributions() []string {
	var result []string
	for _, v := range d.store.Items() {
		if s, ok := v.Object.(map[string]struct{}); ok {
			result = append(result, maputil.Keys(s)...)
		}
	}
	return slice.Unique(result)
}

// 返回所有的resource
func (d DistributionStore) GetAllResources() []string {
	var result []string
	for _, v := range d.store.Items() {
		if s, ok := v.Object.(string); ok {
			result = append(result, s)
		}
	}
	return slice.Unique(result)
}

// RemoveResourceByDistribution 根据分发策略找到对应的资源并删除
// policy1 -> apps/v1/namespace/deployments/nginx
func (d DistributionStore) RemoveResourceByDistribution(key string) {
	d.store.Delete(key)
}

// NewDistributionStore create a new DistributionStore
func NewDistributionStore() DistributionStore {
	return DistributionStore{
		store: kvcache.New(kvcache.NoExpiration, kvcache.NoExpiration),
	}
}

type KeyStore struct {
	mu sync.Mutex

	templateToDistribution map[keys.ClusterWideKey]map[string]struct{}

	distributionToTemplate map[string]keys.ClusterWideKey
}

func (r *KeyStore) StoreRelationship(key keys.ClusterWideKey, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	m1, ok := r.templateToDistribution[key]
	if !ok {
		m1 = make(map[string]struct{})
	}
	m1[value] = struct{}{}
	r.templateToDistribution[key] = m1
	r.distributionToTemplate[value] = key
}

func (r *KeyStore) RemoveRelationship(key keys.ClusterWideKey, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	m1, ok := r.templateToDistribution[key]
	if !ok {
		return
	}
	delete(m1, value)
	r.templateToDistribution[key] = m1
	delete(r.distributionToTemplate, value)
}

func (r *KeyStore) GetDistributions(key keys.ClusterWideKey) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	m1, ok := r.templateToDistribution[key]
	if !ok {
		return []string{}
	}
	return maputil.Keys(m1)
}

func (r *KeyStore) GetAllTemplates() []keys.ClusterWideKey {
	r.mu.Lock()
	defer r.mu.Unlock()

	return maputil.Values(r.distributionToTemplate)
}

func NewKeyStore() *KeyStore {
	return &KeyStore{
		mu:                     sync.Mutex{},
		templateToDistribution: make(map[keys.ClusterWideKey]map[string]struct{}),
		distributionToTemplate: make(map[string]keys.ClusterWideKey),
	}
}
