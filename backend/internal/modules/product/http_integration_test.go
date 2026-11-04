//go:build integration

package product_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/modules/product"
	producttest "github.com/abysstract/abysstract-admin/backend/internal/modules/product/internal/testkit"
)

// Typed CRUD/scope/permission cases live in service_integration_test.go. Keep
// raw JSON and transport limits here so browser setup only prepares its data.
func TestProductRawJSONAndRequestLimit(t *testing.T) {
	db := producttest.OpenMigrated(t)
	env := startApp(t, db, product.ModuleOptions{})
	token := env.login(t, "admin", "admin123")
	for _, tc := range []struct {
		name, body string
		oversized  bool
	}{
		{"bigint", `{"productNo":"raw-bigint","name":"raw","deptId":"1","priceCents":"9007199254740993","createBy":"999"}`, false},
		{"request limit", string(bytes.Repeat([]byte("x"), 2<<20)), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, env.base+"/company.product.v1.ProductService/CreateProduct", bytes.NewBufferString(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Connect-Protocol-Version", "1")
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if tc.oversized {
				if resp.StatusCode < 400 {
					t.Fatalf("oversized HTTP %d", resp.StatusCode)
				}
				return
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("HTTP %d: %s", resp.StatusCode, data)
			}
			var payload struct {
				Item struct {
					PriceCents string `json:"priceCents"`
					CreateBy   string `json:"createBy"`
				} `json:"item"`
			}
			if err := json.Unmarshal(data, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Item.PriceCents != "9007199254740993" || payload.Item.CreateBy != "1" {
				t.Fatalf("JSON: %s", data)
			}
		})
	}
}
