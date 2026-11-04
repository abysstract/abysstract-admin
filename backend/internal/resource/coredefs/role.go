package coredefs

import (
	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// RoleResource separates standard role-field CRUD from authorization relation
// writes. menu_ids is deliberately excluded from generated scalar conversion;
// the typed Get/UpdateRoleMenus CUSTOM operations own that many-to-many path.
var RoleResource = &schema.Resource{
	Name:       "Role",
	PermPrefix: "system:role",
	Mode:       schema.Partial,
	Table:      model.TableNameSysRole,

	Model:     (*model.SysRole)(nil),
	Entity:    (*adminv1.Role)(nil),
	CreateReq: (*adminv1.CreateRoleRequest)(nil),
	UpdateReq: (*adminv1.UpdateRoleRequest)(nil),

	Fields: []schema.Field{
		schema.F("ID", schema.ServerManaged()),
		schema.F("RoleName", schema.Required(), schema.Searchable(), schema.MaxLen(64)),
		schema.F("RoleKey", schema.Identifier(), schema.Required(), schema.Searchable(), schema.MaxLen(64), schema.CreateOnly()),
		schema.F("RoleSort"),
		schema.F("DataScope"),
		schema.F("Status", schema.Filterable(), schema.EnumMap(map[string]int16{
			"STATUS_UNSPECIFIED": 0,
			"STATUS_ENABLED":     1,
			"STATUS_DISABLED":    2,
		}), schema.ReadOnly()),
		schema.F("Remark", schema.MaxLen(500)),
		schema.F("CreateTime", schema.ServerManaged()),
		schema.F("UpdateTime", schema.ServerManaged()),
	},
	EntityIgnore: []string{"menu_ids"},

	Service: schema.ServiceConfig{
		Descriptor: adminv1.File_abysstract_admin_v1_role_proto.Services().ByName("RoleService"),
		Handler:    (*adminv1connect.RoleServiceHandler)(nil),
	},
	Repository: schema.RepositoryConfig{Implementation: schema.RepositoryInjected},
	Operations: append(schema.StandardOperations(schema.StandardOperationsConfig{
		List: schema.StandardOperation{
			Method:   "ListRoles",
			Request:  (*adminv1.ListRolesRequest)(nil),
			Response: (*adminv1.ListRolesResponse)(nil),
			Binding: schema.OperationBinding{List: schema.ListBinding{
				Page: "Page", Keyword: "Keyword",
				Filters: map[string]string{"Status": "Status"},
			}},
		},
		Create: schema.StandardOperation{
			Method:   "CreateRole",
			Request:  (*adminv1.CreateRoleRequest)(nil),
			Response: (*adminv1.CreateRoleResponse)(nil),
			Binding:  schema.OperationBinding{Item: "Role"},
		},
		Get: schema.StandardOperation{
			Method:           "GetRole",
			Request:          (*adminv1.GetRoleRequest)(nil),
			Response:         (*adminv1.GetRoleResponse)(nil),
			Binding:          schema.OperationBinding{Item: "Role"},
			LegacyPermission: schema.LegacyPermissionGetQuery,
		},
		Update: schema.StandardOperation{
			Method:   "UpdateRole",
			Request:  (*adminv1.UpdateRoleRequest)(nil),
			Response: (*adminv1.UpdateRoleResponse)(nil),
			Binding:  schema.OperationBinding{Identifier: "Id", Item: "Role"},
		},
		Delete: schema.StandardOperation{
			Method:   "DeleteRole",
			Request:  (*adminv1.DeleteRoleRequest)(nil),
			Response: (*adminv1.DeleteRoleResponse)(nil),
		},
		BatchDelete: schema.StandardOperation{
			Method:   "BatchDeleteRoles",
			Request:  (*adminv1.BatchDeleteRolesRequest)(nil),
			Response: (*adminv1.BatchDeleteRolesResponse)(nil),
		},
	}),
		schema.Operation{
			Kind:           schema.OperationCustom,
			Method:         "ChangeRoleStatus",
			Request:        (*adminv1.ChangeRoleStatusRequest)(nil),
			Response:       (*adminv1.ChangeRoleStatusResponse)(nil),
			Implementation: schema.OperationOverride,
			Action:         "edit",
			ScopePolicy:    schema.ScopePolicyExempt,
		},
		schema.Operation{
			Kind:           schema.OperationCustom,
			Method:         "GetRoleMenus",
			Request:        (*adminv1.GetRoleMenusRequest)(nil),
			Response:       (*adminv1.GetRoleMenusResponse)(nil),
			Implementation: schema.OperationOverride,
			Action:         "list",
			ScopePolicy:    schema.ScopePolicyExempt,
		},
		schema.Operation{
			Kind:           schema.OperationCustom,
			Method:         "UpdateRoleMenus",
			Request:        (*adminv1.UpdateRoleMenusRequest)(nil),
			Response:       (*adminv1.UpdateRoleMenusResponse)(nil),
			Implementation: schema.OperationOverride,
			Action:         "edit",
			ScopePolicy:    schema.ScopePolicyExempt,
		},
	),
	Key:   schema.KeyConfig{ModelField: "ID"},
	Audit: schema.AuditConfig{UpdatedAtField: "UpdateTime"},
}
