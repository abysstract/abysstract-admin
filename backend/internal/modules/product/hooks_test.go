package product

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"

	gen "github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/resource"
	"github.com/abysstract/abysstract-admin/backend/internal/modules/product/model"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestNormalizeProductNo(t *testing.T) {
	got, err := normalizeProductNo("  ab-12 ")
	if err != nil || got != "ab-12" {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := normalizeProductNo("ab_12"); err == nil {
		t.Fatal("underscore accepted")
	}
	if _, err := normalizeProductNo(""); err == nil {
		t.Fatal("empty accepted")
	}
}

func TestNormalizeNameAndPrice(t *testing.T) {
	got, err := normalizeName("  商品名称  ")
	if err != nil || got != "商品名称" {
		t.Fatalf("name %q err %v", got, err)
	}
	if _, err := normalizeName("   "); err == nil {
		t.Fatal("blank name accepted")
	}
	if err := validatePrice(0); err != nil {
		t.Fatalf("zero amount: %v", err)
	}
	if err := validatePrice(-1); err == nil {
		t.Fatal("negative amount accepted")
	}
	if err := validatePrice(9007199254740993); err != nil {
		t.Fatalf("large amount: %v", err)
	}
}

func TestBeforeCreateRequiresIdentity(t *testing.T) {
	m := &model.ProductItem{ProductNo: "AB-1", Name: "t", DeptID: 1}
	err := beforeCreate(context.Background(), resource.Tx{}, m)
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("err = %v", err)
	}
}

func TestBeforeUpdateKeepsOmittedFields(t *testing.T) {
	m := &model.ProductItem{Name: "  keep  ", PriceCents: -5, Remark: "old"}
	if err := beforeUpdate(context.Background(), resource.Tx{}, nil, m, resource.NewColumnSet(gen.ProductColumnRemark)); err != nil {
		t.Fatal(err)
	}
	if m.Name != "  keep  " || m.PriceCents != -5 {
		t.Fatalf("omitted fields mutated: %+v", m)
	}
	if err := beforeUpdate(context.Background(), resource.Tx{}, nil, m, resource.NewColumnSet(gen.ProductColumnName)); err != nil {
		t.Fatal(err)
	}
	if m.Name != "keep" {
		t.Fatalf("name = %q", m.Name)
	}
	err := beforeUpdate(context.Background(), resource.Tx{}, nil, m, resource.NewColumnSet(gen.ProductColumnPriceCents))
	if err == nil || !strings.Contains(err.Error(), "价格") {
		t.Fatalf("amount err = %v", err)
	}
}
