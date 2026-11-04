package system

import (
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/permission"
)

func registerSystemPermissions(reg permission.Registrar) error {
	if err := interceptor.RegisterBuiltinPermissions(reg); err != nil {
		return err
	}
	registrations := []func(permission.Registrar) error{
		resourcegen.RegisterDictTypePermissions,
		resourcegen.RegisterDictDataPermissions,
		resourcegen.RegisterConfigPermissions,
		resourcegen.RegisterNoticePermissions,
		resourcegen.RegisterDeptPermissions,
		resourcegen.RegisterMenuPermissions,
		resourcegen.RegisterRolePermissions,
		resourcegen.RegisterUserPermissions,
	}
	for _, register := range registrations {
		if err := register(reg); err != nil {
			return err
		}
	}
	return nil
}
