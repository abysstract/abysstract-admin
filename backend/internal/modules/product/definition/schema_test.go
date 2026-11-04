package productdef

import (
	"testing"

	gen "github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/resource"
	"github.com/abysstract/abysstract-admin/backend/permission"
)

type recordingRegistrar struct {
	entries []permission.Entry
}

func (r *recordingRegistrar) RegisterGenerated(entries ...permission.Entry) error {
	r.entries = append(r.entries, entries...)
	return nil
}

func (r *recordingRegistrar) RegisterPublic(...string) error { return nil }

func TestProductResourceBuild(t *testing.T) {
	plan, err := ProductResource.Build()
	if err != nil {
		t.Fatal(err)
	}
	if plan.Res.Name != "Product" || plan.Res.PermPrefix != "biz:product" {
		t.Fatalf("plan resource = %+v", plan.Res)
	}
	if plan.DeptDBName != "dept_id" || plan.CreatorDBName != "create_by" {
		t.Fatalf("scope columns = %q %q", plan.DeptDBName, plan.CreatorDBName)
	}
	if plan.Key.DBName != "id" {
		t.Fatalf("key = %+v", plan.Key)
	}
}

func TestGeneratedPermissionsMatchSchema(t *testing.T) {
	var registrar recordingRegistrar
	if err := gen.RegisterProductPermissions(&registrar); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"/company.product.v1.ProductService/ListProducts":        "biz:product:list",
		"/company.product.v1.ProductService/GetProduct":          "biz:product:list",
		"/company.product.v1.ProductService/CreateProduct":       "biz:product:add",
		"/company.product.v1.ProductService/UpdateProduct":       "biz:product:edit",
		"/company.product.v1.ProductService/DeleteProduct":       "biz:product:remove",
		"/company.product.v1.ProductService/BatchDeleteProducts": "biz:product:remove",
	}
	if len(registrar.entries) != len(want) {
		t.Fatalf("entries = %#v", registrar.entries)
	}
	for _, entry := range registrar.entries {
		if want[entry.Procedure] != entry.Permission {
			t.Fatalf("permission %s = %s", entry.Procedure, entry.Permission)
		}
	}
}
