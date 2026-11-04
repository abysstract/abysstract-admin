package coredefs

import (
	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// DictTypeResource declares the formal generated DictType Resource metadata.
// Proto owns the wire contract; this schema owns CRUD behavior, permissions,
// search/filter/sort reachability, and field lifecycle.
var DictTypeResource = &schema.Resource{
	Name:       "DictType",
	PermPrefix: "system:dict",
	Mode:       schema.Generated,
	Table:      model.TableNameSysDictType,

	Model:     (*model.SysDictType)(nil),
	Entity:    (*adminv1.DictType)(nil),
	CreateReq: (*adminv1.CreateDictTypeRequest)(nil),
	UpdateReq: (*adminv1.UpdateDictTypeRequest)(nil),

	Fields: []schema.Field{
		schema.F("ID", schema.ServerManaged()),
		schema.F("DictName", schema.Required(), schema.Searchable(), schema.MaxLen(100)),
		schema.F("DictType", schema.Identifier(), schema.Required(), schema.Searchable(), schema.Filterable(), schema.MaxLen(100), schema.CreateOnly()),
		schema.F("Status", schema.EnumMap(map[string]int16{"STATUS_ENABLED": 1, "STATUS_DISABLED": 2}), schema.ReadOnly()),
		schema.F("Remark", schema.MaxLen(500)),
		schema.F("CreateTime", schema.ServerManaged()),
		schema.F("UpdateTime", schema.ServerManaged()),
	},

	Service: schema.ServiceConfig{
		Descriptor: adminv1.File_abysstract_admin_v1_dict_proto.Services().ByName("DictService"),
		Handler:    (*adminv1connect.DictServiceHandler)(nil),
	},
	Repository: schema.RepositoryConfig{Implementation: schema.RepositoryGenerated},
	Operations: schema.StandardOperations(schema.StandardOperationsConfig{
		List: schema.StandardOperation{
			Method:   "ListDictTypes",
			Request:  (*adminv1.ListDictTypesRequest)(nil),
			Response: (*adminv1.ListDictTypesResponse)(nil),
			Binding: schema.OperationBinding{List: schema.ListBinding{
				Page: "Page", Keyword: "Keyword", Sort: "Orders",
				Filters: map[string]string{"DictType": "DictType"},
			}},
		},
		Create:      schema.StandardOperation{Method: "CreateDictType", Request: (*adminv1.CreateDictTypeRequest)(nil), Response: (*adminv1.CreateDictTypeResponse)(nil)},
		Get:         schema.StandardOperation{Method: "GetDictType", Request: (*adminv1.GetDictTypeRequest)(nil), Response: (*adminv1.GetDictTypeResponse)(nil)},
		Update:      schema.StandardOperation{Method: "UpdateDictType", Request: (*adminv1.UpdateDictTypeRequest)(nil), Response: (*adminv1.UpdateDictTypeResponse)(nil)},
		Delete:      schema.StandardOperation{Method: "DeleteDictType", Request: (*adminv1.DeleteDictTypeRequest)(nil), Response: (*adminv1.DeleteDictTypeResponse)(nil)},
		BatchDelete: schema.StandardOperation{Method: "BatchDeleteDictTypes", Request: (*adminv1.BatchDeleteDictTypesRequest)(nil), Response: (*adminv1.BatchDeleteDictTypesResponse)(nil)},
	}),
	Key:   schema.KeyConfig{ModelField: "ID"},
	Audit: schema.AuditConfig{UpdatedAtField: "UpdateTime"},
}

// DictDataResource declares the formal generated DictData Resource metadata.
// The resource keeps CRUD behavior generated while allowing the business layer to
// inject a typed repository decorator for list/cache semantics in later phases.
var DictDataResource = &schema.Resource{
	Name:       "DictData",
	PermPrefix: "system:dict",
	Mode:       schema.Generated,
	Table:      model.TableNameSysDictDatum,

	Model:     (*model.SysDictDatum)(nil),
	Entity:    (*adminv1.DictData)(nil),
	CreateReq: (*adminv1.CreateDictDataRequest)(nil),
	UpdateReq: (*adminv1.UpdateDictDataRequest)(nil),

	Fields: []schema.Field{
		schema.F("ID", schema.ServerManaged()),
		schema.F("DictSort", schema.SortableOpt()),
		schema.F("DictLabel", schema.Required(), schema.Searchable(), schema.MaxLen(100)),
		schema.F("DictValue", schema.Required(), schema.MaxLen(100)),
		schema.F("DictType", schema.Identifier(), schema.Required(), schema.Filterable(), schema.MaxLen(100), schema.CreateOnly(),
			schema.RelationTo(func() *schema.Resource { return DictTypeResource }, "DictType", "DictName")),
		schema.F("CSSClass", schema.MaxLen(100), schema.ReadOnly()),
		schema.F("ListClass", schema.MaxLen(100), schema.ReadOnly()),
		schema.F("IsDefault"),
		schema.F("Status", schema.EnumMap(map[string]int16{"STATUS_ENABLED": 1, "STATUS_DISABLED": 2}), schema.ReadOnly()),
		schema.F("Remark", schema.MaxLen(500)),
		schema.F("CreateTime", schema.ServerManaged()),
		schema.F("UpdateTime", schema.ServerManaged()),
	},

	Service: schema.ServiceConfig{
		Descriptor: adminv1.File_abysstract_admin_v1_dict_proto.Services().ByName("DictService"),
		Handler:    (*adminv1connect.DictServiceHandler)(nil),
	},
	Repository: schema.RepositoryConfig{Implementation: schema.RepositoryInjected},
	Operations: schema.StandardOperations(schema.StandardOperationsConfig{
		List: schema.StandardOperation{
			Method:   "ListDictData",
			Request:  (*adminv1.ListDictDataRequest)(nil),
			Response: (*adminv1.ListDictDataResponse)(nil),
			Binding: schema.OperationBinding{List: schema.ListBinding{
				Page: "Page", Keyword: "Keyword", Sort: "Orders",
				Filters: map[string]string{"DictType": "DictType"},
			}},
		},
		Create:      schema.StandardOperation{Method: "CreateDictData", Request: (*adminv1.CreateDictDataRequest)(nil), Response: (*adminv1.CreateDictDataResponse)(nil)},
		Get:         schema.StandardOperation{Method: "GetDictData", Request: (*adminv1.GetDictDataRequest)(nil), Response: (*adminv1.GetDictDataResponse)(nil)},
		Update:      schema.StandardOperation{Method: "UpdateDictData", Request: (*adminv1.UpdateDictDataRequest)(nil), Response: (*adminv1.UpdateDictDataResponse)(nil)},
		Delete:      schema.StandardOperation{Method: "DeleteDictData", Request: (*adminv1.DeleteDictDataRequest)(nil), Response: (*adminv1.DeleteDictDataResponse)(nil)},
		BatchDelete: schema.StandardOperation{Method: "BatchDeleteDictData", Request: (*adminv1.BatchDeleteDictDataRequest)(nil), Response: (*adminv1.BatchDeleteDictDataResponse)(nil)},
	}),
	Key:   schema.KeyConfig{ModelField: "ID"},
	Audit: schema.AuditConfig{UpdatedAtField: "UpdateTime"},
}
