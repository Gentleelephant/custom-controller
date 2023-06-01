package distribution

import (
	"github.com/duke-git/lancet/v2/maputil"
	"github.com/duke-git/lancet/v2/slice"
	"sync"
)

type RelationStore struct {
	mu sync.Mutex

	// apps/v1/namespace/deployments/nginx  -> policy1,policy2,policy3
	resourceToRd map[string][]string

	// policy1 -> apps/v1/namespace/deployments/nginx
	rdToResource map[string]string
}

func (r *RelationStore) StoreReToRd(key string, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resourceToRd[key] = append(r.resourceToRd[key], value)
}

func (r *RelationStore) StoreRdToRe(key string, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rdToResource[key] = value
}

func (r *RelationStore) GetReByRd(key string) string {
	return r.rdToResource[key]
}

func (r *RelationStore) GetRdsByRe(key string) []string {
	return r.resourceToRd[key]
}

func (r *RelationStore) GetAllResourceKey() []string {
	return maputil.Keys(r.resourceToRd)
}

func (r *RelationStore) RemoveRdByRe(key string, remove string) {

	r.mu.Lock()
	defer r.mu.Unlock()
	array := r.resourceToRd[key]
	indexOf := slice.IndexOf(array, remove)
	if indexOf != -1 {
		slice.DeleteAt(array, indexOf)
	}
	r.resourceToRd[key] = array
}

func (r *RelationStore) RemoveReByRd(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rdToResource, key)
}

func NewRelationStore() *RelationStore {
	return &RelationStore{
		resourceToRd: make(map[string][]string),
		rdToResource: make(map[string]string),
	}
}
