package main

import (
	"reflect"
	"testing"
)

func TestSelectTablesUsesModuleBoundary(t *testing.T) {
	tables := []string{"shop", "shop_item", "shop_order_lines", "shopx_card", "sys_user"}
	for prefix, want := range map[string][]string{
		"shop_":       {"shop_item", "shop_order_lines"},
		"shop":        {"shop_item", "shop_order_lines"},
		"shopx":       {"shopx_card"},
		"shop_order_": {"shop_order_lines"},
		"sys_":        {"sys_user"},
	} {
		if got := selectTables(tables, prefix); !reflect.DeepEqual(got, want) {
			t.Errorf("selectTables(%q) = %v, want %v", prefix, got, want)
		}
	}
}
