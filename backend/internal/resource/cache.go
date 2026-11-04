package resource

import public "github.com/abysstract/abysstract-admin/backend/resource"

type VersionedCache[K comparable, V any] = public.VersionedCache[K, V]

func NewVersionedCache[K comparable, V any]() *VersionedCache[K, V] {
	return public.NewVersionedCache[K, V]()
}
