package system

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/module"
)

const moduleName = "system"

// NewModule returns the built-in system module declaration. It does not open a
// database, construct services, or listen.
func NewModule() (module.Definition, error) {
	services, err := declaredServices()
	if err != nil {
		return module.Definition{}, err
	}
	return module.Definition{
		Name:        moduleName,
		Services:    services,
		Permissions: registerSystemPermissions,
		Create:      createSystem,
	}, nil
}

func declaredServices() ([]module.ConnectService, error) {
	specs := []struct {
		file protoreflect.FileDescriptor
		name string
	}{
		{adminv1.File_abysstract_admin_v1_auth_proto, "AuthService"},
		{adminv1.File_abysstract_admin_v1_user_proto, "UserService"},
		{adminv1.File_abysstract_admin_v1_role_proto, "RoleService"},
		{adminv1.File_abysstract_admin_v1_menu_proto, "MenuService"},
		{adminv1.File_abysstract_admin_v1_dept_proto, "DeptService"},
		{adminv1.File_abysstract_admin_v1_dict_proto, "DictService"},
		{adminv1.File_abysstract_admin_v1_config_proto, "ConfigService"},
		{adminv1.File_abysstract_admin_v1_notice_proto, "NoticeService"},
		{adminv1.File_abysstract_admin_v1_log_proto, "LogService"},
		{adminv1.File_abysstract_admin_v1_upload_proto, "UploadService"},
		{adminv1.File_abysstract_admin_v1_common_export_proto, "CommonExportService"},
		{adminv1.File_abysstract_admin_v1_profile_proto, "ProfileService"},
		{adminv1.File_abysstract_admin_v1_dashboard_proto, "DashboardService"},
	}
	out := make([]module.ConnectService, 0, len(specs))
	for _, spec := range specs {
		desc, err := serviceDescriptor(spec.file, spec.name)
		if err != nil {
			return nil, err
		}
		out = append(out, module.ConnectService{Descriptor: desc})
	}
	return out, nil
}

func serviceDescriptor(file protoreflect.FileDescriptor, name string) (protoreflect.ServiceDescriptor, error) {
	if file == nil {
		return nil, fmt.Errorf("system: missing descriptor file for %s", name)
	}
	svc := file.Services().ByName(protoreflect.Name(name))
	if svc == nil {
		return nil, fmt.Errorf("system: missing service %s", name)
	}
	return svc, nil
}
