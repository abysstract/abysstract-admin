package coredefs

import (
	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// DeptResource declares the standard flat Resource contract. The generated
// Controller owns all six CRUD operations; the injected Repository and Hooks
// retain only Dept data-integrity semantics (soft delete, ancestors, and
// parent/child protection).
var DeptResource = &schema.Resource{
	Name:       "Dept",
	PermPrefix: "system:dept",
	Mode:       schema.Generated,
	Table:      model.TableNameSysDept,

	Model:     (*model.SysDept)(nil),
	Entity:    (*adminv1.Dept)(nil),
	CreateReq: (*adminv1.CreateDeptRequest)(nil),
	UpdateReq: (*adminv1.UpdateDeptRequest)(nil),

	Fields: []schema.Field{
		schema.F("ID", schema.ServerManaged()),
		schema.F("ParentID", schema.Filterable()),
		schema.F("DeptName", schema.Required(), schema.Searchable(), schema.MaxLen(64)),
		schema.F("OrderNum", schema.SortableOpt()),
		schema.F("Leader", schema.Searchable(), schema.MaxLen(64)),
		schema.F("Phone", schema.Searchable(), schema.MaxLen(32)),
		schema.F("Email", schema.Searchable(), schema.MaxLen(128)),
		schema.F("Status", schema.Filterable(), schema.EnumMap(map[string]int16{
			"STATUS_UNSPECIFIED": 0,
			"STATUS_ENABLED":     1,
			"STATUS_DISABLED":    2,
		}), schema.ReadOnly()),
		schema.F("CreateTime", schema.ServerManaged()),
		schema.F("UpdateTime", schema.ServerManaged()),
	},
	// Ancestors and DelFlag are persistence-only model state. Children is a
	// frontend tree projection assembled from the standard flat List result.
	EntityIgnore: []string{"children"},

	Service: schema.ServiceConfig{
		Descriptor: adminv1.File_abysstract_admin_v1_dept_proto.Services().ByName("DeptService"),
		Handler:    (*adminv1connect.DeptServiceHandler)(nil),
	},
	Repository: schema.RepositoryConfig{Implementation: schema.RepositoryInjected},
	Operations: schema.StandardOperations(schema.StandardOperationsConfig{
		List: schema.StandardOperation{
			Method:   "ListDeptRows",
			Request:  (*adminv1.ListDeptRowsRequest)(nil),
			Response: (*adminv1.ListDeptRowsResponse)(nil),
			Binding: schema.OperationBinding{List: schema.ListBinding{
				Page: "Page", Keyword: "Keyword", Sort: "Orders",
				Filters: map[string]string{"ParentID": "ParentId", "Status": "Status"},
			}},
		},
		Create: schema.StandardOperation{
			Method:   "CreateDept",
			Request:  (*adminv1.CreateDeptRequest)(nil),
			Response: (*adminv1.CreateDeptResponse)(nil),
			Binding:  schema.OperationBinding{Item: "Dept"},
		},
		Get: schema.StandardOperation{
			Method:           "GetDept",
			Request:          (*adminv1.GetDeptRequest)(nil),
			Response:         (*adminv1.GetDeptResponse)(nil),
			Binding:          schema.OperationBinding{Item: "Dept"},
			LegacyPermission: schema.LegacyPermissionGetQuery,
		},
		Update: schema.StandardOperation{
			Method:   "UpdateDept",
			Request:  (*adminv1.UpdateDeptRequest)(nil),
			Response: (*adminv1.UpdateDeptResponse)(nil),
			Binding:  schema.OperationBinding{Identifier: "Id", Item: "Dept"},
		},
		Delete: schema.StandardOperation{
			Method:   "DeleteDept",
			Request:  (*adminv1.DeleteDeptRequest)(nil),
			Response: (*adminv1.DeleteDeptResponse)(nil),
		},
		BatchDelete: schema.StandardOperation{
			Method:   "BatchDeleteDepts",
			Request:  (*adminv1.BatchDeleteDeptsRequest)(nil),
			Response: (*adminv1.BatchDeleteDeptsResponse)(nil),
		},
	}),
	Key:   schema.KeyConfig{ModelField: "ID"},
	Audit: schema.AuditConfig{UpdatedAtField: "UpdateTime"},
}
