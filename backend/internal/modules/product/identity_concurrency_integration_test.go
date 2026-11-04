//go:build integration

package product_test

import (
	"testing"

	producttest "github.com/abysstract/abysstract-admin/backend/internal/modules/product/internal/testkit"

	"connectrpc.com/connect/v2"

	"github.com/abysstract/abysstract-admin/backend/internal/modules/product"
	productv1 "github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/proto/company/product/v1"
)

func TestConcurrentProductIdentity(t *testing.T) {
	db := producttest.OpenMigrated(t)
	env := startApp(t, db, product.ModuleOptions{})
	token := env.login(t, "admin", "admin123")
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, no := range []string{" P3-NO ", "p3-no"} {
		go func(no string) {
			<-start
			_, err := env.products.CreateProduct(authed(t, token), &productv1.CreateProductRequest{ProductNo: no, Name: "parallel", DeptId: 1})
			results <- err
		}(no)
	}
	close(start)
	success, duplicate := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if connect.CodeOf(err) == connect.CodeAlreadyExists {
			duplicate++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || duplicate != 1 {
		t.Fatalf("success=%d duplicate=%d", success, duplicate)
	}
	if producttest.Count(t, db, "SELECT COUNT(*) FROM product_item WHERE product_no='p3-no'") != 1 {
		t.Fatal("canonical identity count")
	}
}
