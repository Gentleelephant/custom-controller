package distribution

import (
	"github.com/duke-git/lancet/v2/maputil"
	kvcache "github.com/patrickmn/go-cache"
)

type WorkloadStore struct {
	store *kvcache.Cache
}

func (d WorkloadStore) StoreResourcePointToWorkload(key, value string) {
	d.store.Set(key, value, kvcache.NoExpiration)
}

func (d WorkloadStore) StoreWorkloadPointToResource(key, value string) {
	result, exist := d.store.Get(key)
	if !exist {
		m := make(map[string]struct{})
		m[value] = struct{}{}
		d.store.Set(key, m, kvcache.NoExpiration)
		return
	}
	m, ok := result.(map[string]struct{})
	if !ok {
		return
	}
	m[value] = struct{}{}
}

func (d WorkloadStore) GetWorkloadByResource(key string) (string, bool) {
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

func (d WorkloadStore) GetResourcesByWorkload(key string) []string {
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

// remove the resource from the workload
func (d WorkloadStore) RemoveResourceWorkloadRelation(workloadname string) {
	// 根据key获取value
	result, exist := d.store.Get(workloadname)
	if !exist {
		return
	}
	// 将value转换为map
	m, ok := result.(map[string]struct{})
	if !ok {
		return
	}
	// 遍历map，删除value中的workloadname
	for k := range m {
		d.store.Delete(k)
	}
	// 将map也删除
	d.store.Delete(workloadname)
}

func NewWorkloadStore() WorkloadStore {
	return WorkloadStore{
		store: kvcache.New(kvcache.NoExpiration, kvcache.NoExpiration),
	}
}
