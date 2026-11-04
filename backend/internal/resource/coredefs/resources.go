// Package coredefs contains the reference application's system resource declarations.
package coredefs

import "github.com/abysstract/abysstract-admin/backend/resource/schema"

// Resources returns the explicit system generation input. Each call owns its
// slice, while resource pointers preserve cross-resource relationship identity.
func Resources() []*schema.Resource {
	return []*schema.Resource{
		DictTypeResource, DictDataResource, ConfigResource, NoticeResource,
		DeptResource, MenuResource, RoleResource, UserResource,
	}
}
