package coredefs

import (
	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// UserResource is intentionally a low-risk Partial Resource. Only List enters
// the generated controller in this phase; detail Get, password, role assignment,
// create, update, delete, status, and reset-password remain typed CUSTOM
// workflows until their security gates are independently reviewed.
var UserResource = &schema.Resource{
	Name:       "User",
	PermPrefix: "system:user",
	Mode:       schema.Partial,
	Table:      model.TableNameSysUser,

	Model:     (*model.SysUser)(nil),
	Entity:    (*adminv1.User)(nil),
	CreateReq: (*adminv1.CreateUserRequest)(nil),
	UpdateReq: (*adminv1.UpdateUserRequest)(nil),

	Fields: []schema.Field{
		schema.F("ID", schema.ServerManaged()),
		schema.F("Username", schema.Required(), schema.Searchable(), schema.MaxLen(64), schema.CreateOnly()),
		schema.F("Nickname", schema.Searchable(), schema.MaxLen(64)),
		schema.F("Email", schema.MaxLen(128)),
		schema.F("Phone", schema.MaxLen(32)),
		schema.F("Status", schema.Filterable(), schema.EnumMap(map[string]int16{
			"STATUS_UNSPECIFIED": 0,
			"STATUS_ENABLED":     1,
			"STATUS_DISABLED":    2,
		}), schema.ReadOnly()),
		schema.F("DeptID", schema.Filterable()),
		schema.F("Remark", schema.MaxLen(500)),
		schema.F("CreateTime", schema.ServerManaged()),
		schema.F("UpdateTime", schema.ServerManaged()),
	},
	// password/role_ids are deliberately not scalar Resource fields. The
	// remaining model-only security/session fields are not projected either.
	Ignore:       []string{"password", "role_ids"},
	EntityIgnore: []string{"role_ids"},

	Service: schema.ServiceConfig{
		Descriptor: adminv1.File_abysstract_admin_v1_user_proto.Services().ByName("UserService"),
		Handler:    (*adminv1connect.UserServiceHandler)(nil),
	},
	Repository: schema.RepositoryConfig{Implementation: schema.RepositoryInjected},
	Operations: []schema.Operation{
		{
			Kind:           schema.OperationList,
			Method:         "ListUsers",
			Request:        (*adminv1.ListUsersRequest)(nil),
			Response:       (*adminv1.ListUsersResponse)(nil),
			Binding:        schema.OperationBinding{List: schema.ListBinding{Page: "Page", Keyword: "Keyword", Items: "Items", PageInfo: "Page", Filters: map[string]string{"Status": "Status", "DeptID": "DeptId"}}},
			Implementation: schema.OperationGenerated,
		},
		{
			Kind:           schema.OperationCustom,
			Method:         "GetUser",
			Request:        (*adminv1.GetUserRequest)(nil),
			Response:       (*adminv1.GetUserResponse)(nil),
			Implementation: schema.OperationOverride,
			Action:         "query",
			ScopePolicy:    schema.ScopePolicyExempt,
		},
		{
			Kind:           schema.OperationCustom,
			Method:         "CreateUser",
			Request:        (*adminv1.CreateUserRequest)(nil),
			Response:       (*adminv1.CreateUserResponse)(nil),
			Implementation: schema.OperationOverride,
			Action:         "add",
			ScopePolicy:    schema.ScopePolicyExempt,
		},
		{
			Kind:           schema.OperationCustom,
			Method:         "UpdateUser",
			Request:        (*adminv1.UpdateUserRequest)(nil),
			Response:       (*adminv1.UpdateUserResponse)(nil),
			Binding:        schema.OperationBinding{Identifier: "Id"},
			Implementation: schema.OperationOverride,
			Action:         "edit",
			ScopePolicy:    schema.ScopePolicyExempt,
		},
		{
			Kind:           schema.OperationCustom,
			Method:         "DeleteUser",
			Request:        (*adminv1.DeleteUserRequest)(nil),
			Response:       (*adminv1.DeleteUserResponse)(nil),
			Implementation: schema.OperationOverride,
			Action:         "remove",
			ScopePolicy:    schema.ScopePolicyExempt,
		},
		{
			Kind:           schema.OperationCustom,
			Method:         "BatchDeleteUsers",
			Request:        (*adminv1.BatchDeleteUsersRequest)(nil),
			Response:       (*adminv1.BatchDeleteUsersResponse)(nil),
			Binding:        schema.OperationBinding{BatchIDs: "Ids"},
			Implementation: schema.OperationOverride,
			Action:         "remove",
			ScopePolicy:    schema.ScopePolicyExempt,
		},
		{
			Kind:           schema.OperationCustom,
			Method:         "ChangeUserStatus",
			Request:        (*adminv1.ChangeUserStatusRequest)(nil),
			Response:       (*adminv1.ChangeUserStatusResponse)(nil),
			Implementation: schema.OperationOverride,
			Action:         "edit",
			ScopePolicy:    schema.ScopePolicyExempt,
		},
		{
			Kind:           schema.OperationCustom,
			Method:         "ResetPassword",
			Request:        (*adminv1.ResetPasswordRequest)(nil),
			Response:       (*adminv1.ResetPasswordResponse)(nil),
			Implementation: schema.OperationOverride,
			Action:         "resetPwd",
			ScopePolicy:    schema.ScopePolicyExempt,
		},
	},
	Auth: schema.AuthConfig{
		DataScope:    true,
		DeptField:    "DeptID",
		CreatorField: "CreateBy",
	},
	Key:   schema.KeyConfig{ModelField: "ID"},
	Audit: schema.AuditConfig{UpdatedAtField: "UpdateTime"},
}
