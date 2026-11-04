package resource

import public "github.com/abysstract/abysstract-admin/backend/resource"

type HookSet[M any] = public.HookSet[M]

func ComposeHooks[M any](system, business *HookSet[M]) *HookSet[M] {
	return public.ComposeHooks(system, business)
}
