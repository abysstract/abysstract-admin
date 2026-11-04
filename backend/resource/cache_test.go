package resource

import (
	"sync"
	"testing"
)

func TestVersionedCacheRejectsStaleFillAfterInvalidation(t *testing.T) {
	cache := NewVersionedCache[string, string]()
	version := cache.Version()
	if !cache.SetIfVersion(version, "widgets", "old") {
		t.Fatal("initial fill rejected")
	}
	cache.InvalidateAll()
	if cache.SetIfVersion(version, "widgets", "stale") {
		t.Fatal("stale fill after invalidation must be rejected")
	}
	if _, ok := cache.Get("widgets"); ok {
		t.Fatal("stale value must not enter cache")
	}
}

func TestVersionedCacheBarrierPreventsOldReadRefill(t *testing.T) {
	cache := NewVersionedCache[string, string]()
	readVersion := cache.Version()
	ready, release := make(chan struct{}), make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); close(ready); <-release; cache.SetIfVersion(readVersion, "widgets", "old") }()
	<-ready
	cache.InvalidateAll()
	close(release)
	wg.Wait()
	if _, ok := cache.Get("widgets"); ok {
		t.Fatal("old query refilled cache after commit invalidation")
	}
}
