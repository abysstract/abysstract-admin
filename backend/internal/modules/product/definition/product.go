package productdef

import (
	productv1 "github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/proto/company/product/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/proto/company/product/v1/productv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/modules/product/model"

	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

// ProductResource is the business Product resource definition. Schema is the
// source of field lifecycle, search, sort, permission, audit, and data scope.
var ProductResource = &schema.Resource{
	Name:       "Product",
	PermPrefix: "biz:product",
	Mode:       schema.Generated,
	Table:      model.TableNameProductItem,
	Model:      (*model.ProductItem)(nil),
	Entity:     (*productv1.Product)(nil),
	CreateReq:  (*productv1.CreateProductRequest)(nil),
	UpdateReq:  (*productv1.UpdateProductRequest)(nil),
	Fields: []schema.Field{
		schema.F("ID", schema.ServerManaged()),
		schema.F("ProductNo", schema.Label("商品编号"), schema.Required(), schema.MaxLen(64), schema.CreateOnly()),
		schema.F("Name", schema.Label("商品名称"), schema.Required(), schema.Searchable(), schema.MaxLen(100)),
		schema.F("PriceCents", schema.Label("价格(分)"), schema.SortableOpt()),
		schema.F("DeptID", schema.Label("部门ID"), schema.Filterable(), schema.CreateOnly()),
		schema.F("CreateBy", schema.ServerManaged()),
		schema.F("Remark", schema.Label("备注"), schema.MaxLen(500)),
		schema.F("CreateTime", schema.ServerManaged()),
		schema.F("UpdateTime", schema.ServerManaged()),
	},
	Service: schema.ServiceConfig{
		Descriptor: productv1.File_company_product_v1_product_proto.Services().ByName("ProductService"),
		Handler:    (*productv1connect.ProductServiceHandler)(nil),
	},
	Operations: schema.StandardOperations(schema.StandardOperationsConfig{
		List: schema.StandardOperation{
			Method:   "ListProducts",
			Request:  (*productv1.ListProductsRequest)(nil),
			Response: (*productv1.ListProductsResponse)(nil),
			Binding: schema.OperationBinding{List: schema.ListBinding{
				Page: "Page", Keyword: "Keyword", Sort: "Orders",
				Filters:       map[string]string{"DeptID": "DeptId"},
				DefaultOrders: []schema.ListDefaultOrder{{Field: "ID"}},
			}},
		},
		Create: schema.StandardOperation{Method: "CreateProduct", Request: (*productv1.CreateProductRequest)(nil), Response: (*productv1.CreateProductResponse)(nil)},
		Get:    schema.StandardOperation{Method: "GetProduct", Request: (*productv1.GetProductRequest)(nil), Response: (*productv1.GetProductResponse)(nil)},
		Update: schema.StandardOperation{
			Method:   "UpdateProduct",
			Request:  (*productv1.UpdateProductRequest)(nil),
			Response: (*productv1.UpdateProductResponse)(nil),
			Binding:  schema.OperationBinding{Identifier: "Id"},
		},
		Delete:      schema.StandardOperation{Method: "DeleteProduct", Request: (*productv1.DeleteProductRequest)(nil), Response: (*productv1.DeleteProductResponse)(nil)},
		BatchDelete: schema.StandardOperation{Method: "BatchDeleteProducts", Request: (*productv1.BatchDeleteProductsRequest)(nil), Response: (*productv1.BatchDeleteProductsResponse)(nil)},
	}),
	Auth: schema.AuthConfig{
		DataScope:    true,
		DeptField:    "DeptID",
		CreatorField: "CreateBy",
	},
	Key:   schema.KeyConfig{ModelField: "ID"},
	Audit: schema.AuditConfig{UpdatedAtField: "UpdateTime"},
}

func Resources() []*schema.Resource {
	return []*schema.Resource{ProductResource}
}
