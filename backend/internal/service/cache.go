package service

import (
	"sync"
	"sync/atomic"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// dictCache 是 service 层的内存缓存（需求 9.11）。
// 未发生变更时 ListDictData 命中缓存不查库；任何写操作通过 Invalidate 失效。
// 配置热失效（需求 9.12）：参数配置更新触发对应 cache.Invalidate。
type dictCache struct {
	mu     sync.RWMutex
	data   map[string][]*model.SysDictDatum
	hits   atomic.Int64
	misses atomic.Int64
}

func newDictCache() *dictCache {
	return &dictCache{data: make(map[string][]*model.SysDictDatum)}
}

// Get 命中返回数据 + true；未命中返回 nil + false。
func (c *dictCache) Get(dictType string) ([]*model.SysDictDatum, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if v, ok := c.data[dictType]; ok {
		c.hits.Add(1)
		return v, true
	}
	c.misses.Add(1)
	return nil, false
}

// Set 写入缓存（覆盖）。
func (c *dictCache) Set(dictType string, items []*model.SysDictDatum) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[dictType] = items
}

// Invalidate 使指定 dictType 失效（需求 9.12 等价语义）。
func (c *dictCache) Invalidate(dictType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, dictType)
}

// InvalidateAll 清空所有缓存（DictType 改名 / 删除时使用）。
func (c *dictCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string][]*model.SysDictDatum)
}

// Stats 返回命中/未命中计数，便于测试断言。
func (c *dictCache) Stats() (hits, misses int) {
	return int(c.hits.Load()), int(c.misses.Load())
}
