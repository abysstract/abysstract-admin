package resource

import "sync"

// VersionedCache is a shared cache-aside primitive. Each invalidation advances
// an epoch, so a DB read that began before a committed write cannot refill an
// entry with stale data after that write.
type VersionedCache[K comparable, V any] struct {
	mu      sync.RWMutex
	epoch   uint64
	entries map[K]V
}

func NewVersionedCache[K comparable, V any]() *VersionedCache[K, V] {
	return &VersionedCache[K, V]{entries: make(map[K]V)}
}

func (c *VersionedCache[K, V]) Version() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.epoch
}

func (c *VersionedCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.entries[key]
	return value, ok
}

func (c *VersionedCache[K, V]) SetIfVersion(version uint64, key K, value V) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.epoch != version {
		return false
	}
	c.entries[key] = value
	return true
}

func (c *VersionedCache[K, V]) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch++
	c.entries = make(map[K]V)
}
