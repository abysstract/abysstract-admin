package tableprefix

import (
	"strings"
	"testing"
)

func TestMatchUsesUnderscoreBoundary(t *testing.T) {
	cases := []struct {
		prefix, table string
		want          bool
	}{
		{"shop_", "shop_item", true},
		{"shop", "shop_item", true},
		{"shop_", "shop_order_lines", true},
		{"shop", "shopx_card", false},
		{"shop_", "shopx_card", false},
		{"shop_", "shop_", false},
		{"shop", "shop", false},
		{"shop_order_", "shop_item", false},
		{"sys_", "sys_user", true},
	}
	for _, c := range cases {
		if got := Match(c.prefix, c.table); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.prefix, c.table, got, c.want)
		}
	}
}

func TestCheckOverlapNamesConflictingModules(t *testing.T) {
	err := CheckOverlap([]Owner{{"system", "sys_"}, {"shop", "shop_"}, {"shoporder", "shop_order_"}})
	if err == nil || !strings.Contains(err.Error(), `module shop tablePrefix "shop_" overlaps module shoporder tablePrefix "shop_order_"`) {
		t.Fatalf("nested prefixes accepted: %v", err)
	}
	// "shop" and "shop_" select the same namespace.
	if err := CheckOverlap([]Owner{{"a", "shop"}, {"b", "shop_"}}); err == nil {
		t.Fatal("equal normalized prefixes accepted")
	}
	// Without the boundary "shop" would have selected shopx_* tables.
	if err := CheckOverlap([]Owner{{"shop", "shop"}, {"shopx", "shopx_"}, {"system", "sys_"}, {"product", "product_"}}); err != nil {
		t.Fatalf("disjoint namespaces rejected: %v", err)
	}
	// The same module listed twice (installed and explicit target) is not a conflict.
	if err := CheckOverlap([]Owner{{"shop", "shop_"}, {"shop", "shop_"}}); err != nil {
		t.Fatalf("one module conflicts with itself: %v", err)
	}
}

func TestValidate(t *testing.T) {
	for _, ok := range []string{"sys_", "shop", "shop_order_", "a1_b2"} {
		if err := Validate(ok); err != nil {
			t.Errorf("Validate(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{"", "_", "SHOP_", "shop__", "shop-", "1shop_", "shop_%"} {
		if err := Validate(bad); err == nil {
			t.Errorf("Validate(%q) accepted", bad)
		}
	}
}
