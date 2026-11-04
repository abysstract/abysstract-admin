package coredefs

import (
	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// NoticeResource declares the flat, standard Notice CRUD resource. The
// notice_type value is intentionally kept as a plain int32 wire field: its
// 1=通知/2=公告 semantic and create-time default are Notice-specific Hook
// behavior, not a protobuf enum or shared Resource DSL concern.
var NoticeResource = &schema.Resource{
	Name:       "Notice",
	PermPrefix: "system:notice",
	Mode:       schema.Generated,
	Table:      model.TableNameSysNotice,

	Model:     (*model.SysNotice)(nil),
	Entity:    (*adminv1.Notice)(nil),
	CreateReq: (*adminv1.CreateNoticeRequest)(nil),
	UpdateReq: (*adminv1.UpdateNoticeRequest)(nil),

	Fields: []schema.Field{
		schema.F("ID", schema.ServerManaged()),
		schema.F("NoticeTitle", schema.Required(), schema.Searchable(), schema.MaxLen(100)),
		schema.F("NoticeType"),
		schema.F("NoticeContent", schema.Required()),
		schema.F("Status", schema.EnumMap(map[string]int16{
			"STATUS_UNSPECIFIED": 0,
			"STATUS_ENABLED":     1,
			"STATUS_DISABLED":    2,
		}), schema.ReadOnly()),
		schema.F("Remark", schema.MaxLen(500)),
		schema.F("CreateTime", schema.ServerManaged()),
		schema.F("UpdateTime", schema.ServerManaged()),
	},

	Service: schema.ServiceConfig{
		Descriptor: adminv1.File_abysstract_admin_v1_notice_proto.Services().ByName("NoticeService"),
		Handler:    (*adminv1connect.NoticeServiceHandler)(nil),
	},
	Repository: schema.RepositoryConfig{Implementation: schema.RepositoryGenerated},
	Operations: schema.StandardOperations(schema.StandardOperationsConfig{
		List: schema.StandardOperation{
			Method:   "ListNotices",
			Request:  (*adminv1.ListNoticesRequest)(nil),
			Response: (*adminv1.ListNoticesResponse)(nil),
			Binding: schema.OperationBinding{List: schema.ListBinding{
				Page: "Page", Keyword: "Keyword",
				DefaultOrders: []schema.ListDefaultOrder{{Field: "ID", Desc: false}},
			}},
		},
		Create:      schema.StandardOperation{Method: "CreateNotice", Request: (*adminv1.CreateNoticeRequest)(nil), Response: (*adminv1.CreateNoticeResponse)(nil)},
		Get:         schema.StandardOperation{Method: "GetNotice", Request: (*adminv1.GetNoticeRequest)(nil), Response: (*adminv1.GetNoticeResponse)(nil)},
		Update:      schema.StandardOperation{Method: "UpdateNotice", Request: (*adminv1.UpdateNoticeRequest)(nil), Response: (*adminv1.UpdateNoticeResponse)(nil)},
		Delete:      schema.StandardOperation{Method: "DeleteNotice", Request: (*adminv1.DeleteNoticeRequest)(nil), Response: (*adminv1.DeleteNoticeResponse)(nil)},
		BatchDelete: schema.StandardOperation{Method: "BatchDeleteNotices", Request: (*adminv1.BatchDeleteNoticesRequest)(nil), Response: (*adminv1.BatchDeleteNoticesResponse)(nil)},
	}),
	Key:   schema.KeyConfig{ModelField: "ID"},
	Audit: schema.AuditConfig{UpdatedAtField: "UpdateTime"},
}
