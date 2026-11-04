package coredefs

import (
	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// ConfigResource declares the generated sys_config behavior.
// The API supports keyword search through ConfigName and ConfigKey.
var ConfigResource = &schema.Resource{
	Name:       "Config",
	PermPrefix: "system:config",
	Mode:       schema.Generated,
	Table:      model.TableNameSysConfig,

	Model:     (*model.SysConfig)(nil),
	Entity:    (*adminv1.Config)(nil),
	CreateReq: (*adminv1.CreateConfigRequest)(nil),
	UpdateReq: (*adminv1.UpdateConfigRequest)(nil),

	Fields: []schema.Field{
		schema.F("ID", schema.ServerManaged()),
		schema.F("ConfigName", schema.Required(), schema.Searchable(), schema.MaxLen(100)),
		schema.F("ConfigKey", schema.Identifier(), schema.Required(), schema.Searchable(), schema.MaxLen(100), schema.CreateOnly()),
		schema.F("ConfigValue", schema.Required(), schema.MaxLen(500)),
		schema.F("ConfigType", schema.CreateOnly()),
		schema.F("Remark", schema.MaxLen(500)),
		schema.F("CreateTime", schema.ServerManaged()),
		schema.F("UpdateTime", schema.ServerManaged()),
	},

	Service: schema.ServiceConfig{
		Descriptor: adminv1.File_abysstract_admin_v1_config_proto.Services().ByName("ConfigService"),
		Handler:    (*adminv1connect.ConfigServiceHandler)(nil),
	},
	Repository: schema.RepositoryConfig{Implementation: schema.RepositoryInjected},
	Operations: schema.StandardOperations(schema.StandardOperationsConfig{
		List: schema.StandardOperation{
			Method:   "ListConfigs",
			Request:  (*adminv1.ListConfigsRequest)(nil),
			Response: (*adminv1.ListConfigsResponse)(nil),
			Binding: schema.OperationBinding{List: schema.ListBinding{
				Page: "Page", Keyword: "Keyword",
			}},
		},
		Create:      schema.StandardOperation{Method: "CreateConfig", Request: (*adminv1.CreateConfigRequest)(nil), Response: (*adminv1.CreateConfigResponse)(nil)},
		Get:         schema.StandardOperation{Method: "GetConfig", Request: (*adminv1.GetConfigRequest)(nil), Response: (*adminv1.GetConfigResponse)(nil)},
		Update:      schema.StandardOperation{Method: "UpdateConfig", Request: (*adminv1.UpdateConfigRequest)(nil), Response: (*adminv1.UpdateConfigResponse)(nil)},
		Delete:      schema.StandardOperation{Method: "DeleteConfig", Request: (*adminv1.DeleteConfigRequest)(nil), Response: (*adminv1.DeleteConfigResponse)(nil)},
		BatchDelete: schema.StandardOperation{Method: "BatchDeleteConfigs", Request: (*adminv1.BatchDeleteConfigsRequest)(nil), Response: (*adminv1.BatchDeleteConfigsResponse)(nil)},
	}),
	Key:   schema.KeyConfig{ModelField: "ID"},
	Audit: schema.AuditConfig{UpdatedAtField: "UpdateTime"},
}
