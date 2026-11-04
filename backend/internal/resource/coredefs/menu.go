package coredefs

import (
	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// MenuResource declares the standard flat Menu Resource. The generated
// Controller owns all six CRUD operations; injected Repository/Hook code keeps
// role_menu cleanup, parent-cycle protection and menu_type domain rules out of
// the generic Resource kernel. children is only a frontend tree projection.
var MenuResource = &schema.Resource{
	Name:       "Menu",
	PermPrefix: "system:menu",
	Mode:       schema.Generated,
	Table:      model.TableNameSysMenu,

	Model:     (*model.SysMenu)(nil),
	Entity:    (*adminv1.Menu)(nil),
	CreateReq: (*adminv1.CreateMenuRequest)(nil),
	UpdateReq: (*adminv1.UpdateMenuRequest)(nil),

	Fields: []schema.Field{
		schema.F("ID", schema.ServerManaged()),
		schema.F("MenuName", schema.Required(), schema.Searchable(), schema.MaxLen(64)),
		schema.F("ParentID", schema.Filterable()),
		schema.F("OrderNum", schema.SortableOpt()),
		schema.F("Path", schema.Searchable(), schema.MaxLen(255)),
		schema.F("Component", schema.Searchable(), schema.MaxLen(255)),
		schema.F("Query", schema.MaxLen(255)),
		schema.F("IsFrame"),
		schema.F("IsCache"),
		schema.F("MenuType", schema.Required(), schema.Filterable(), schema.MaxLen(1)),
		schema.F("Visible"),
		schema.F("Status", schema.Filterable(), schema.EnumMap(map[string]int16{
			"STATUS_UNSPECIFIED": 0,
			"STATUS_ENABLED":     1,
			"STATUS_DISABLED":    2,
		}), schema.ReadOnly()),
		schema.F("Perms", schema.Searchable(), schema.MaxLen(128)),
		schema.F("Icon", schema.MaxLen(128)),
		schema.F("CreateTime", schema.ServerManaged()),
		schema.F("UpdateTime", schema.ServerManaged()),
	},
	EntityIgnore: []string{"children"},

	Service: schema.ServiceConfig{
		Descriptor: adminv1.File_abysstract_admin_v1_menu_proto.Services().ByName("MenuService"),
		Handler:    (*adminv1connect.MenuServiceHandler)(nil),
	},
	Repository: schema.RepositoryConfig{Implementation: schema.RepositoryInjected},
	Operations: schema.StandardOperations(schema.StandardOperationsConfig{
		List: schema.StandardOperation{
			Method:   "ListMenus",
			Request:  (*adminv1.ListMenusRequest)(nil),
			Response: (*adminv1.ListMenusResponse)(nil),
			Binding: schema.OperationBinding{List: schema.ListBinding{
				Page: "Page", Keyword: "Keyword", Sort: "Orders",
				Filters: map[string]string{"ParentID": "ParentId", "MenuType": "MenuType", "Status": "Status"},
			}},
		},
		Create: schema.StandardOperation{
			Method:   "CreateMenu",
			Request:  (*adminv1.CreateMenuRequest)(nil),
			Response: (*adminv1.CreateMenuResponse)(nil),
			Binding:  schema.OperationBinding{Item: "Menu"},
		},
		Get: schema.StandardOperation{
			Method:           "GetMenu",
			Request:          (*adminv1.GetMenuRequest)(nil),
			Response:         (*adminv1.GetMenuResponse)(nil),
			Binding:          schema.OperationBinding{Item: "Menu"},
			LegacyPermission: schema.LegacyPermissionGetQuery,
		},
		Update: schema.StandardOperation{
			Method:   "UpdateMenu",
			Request:  (*adminv1.UpdateMenuRequest)(nil),
			Response: (*adminv1.UpdateMenuResponse)(nil),
			Binding:  schema.OperationBinding{Identifier: "Id", Item: "Menu"},
		},
		Delete: schema.StandardOperation{
			Method:   "DeleteMenu",
			Request:  (*adminv1.DeleteMenuRequest)(nil),
			Response: (*adminv1.DeleteMenuResponse)(nil),
		},
		BatchDelete: schema.StandardOperation{
			Method:   "BatchDeleteMenus",
			Request:  (*adminv1.BatchDeleteMenusRequest)(nil),
			Response: (*adminv1.BatchDeleteMenusResponse)(nil),
		},
	}),
	Key:   schema.KeyConfig{ModelField: "ID"},
	Audit: schema.AuditConfig{UpdatedAtField: "UpdateTime"},
}
