// Package tableprefix selects module tables by their "<prefix>_" boundary and
// rejects module declarations whose table namespaces overlap.
package tableprefix

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var valid = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*_?$`)

// Owner is one module's declared table prefix.
type Owner struct {
	Module string
	Prefix string
}

// Normalize returns the prefix with exactly one trailing underscore, so "shop"
// and "shop_" both select tables named shop_<table>.
func Normalize(prefix string) string {
	return strings.TrimSuffix(prefix, "_") + "_"
}

// Validate checks that a declared prefix is a snake_case table namespace.
func Validate(prefix string) error {
	if !valid.MatchString(prefix) {
		return fmt.Errorf("tablePrefix %q must be snake_case, for example %q", prefix, "notes_")
	}
	return nil
}

// Match reports whether table belongs to prefix at the underscore boundary.
// The prefix alone is not a module table.
func Match(prefix, table string) bool {
	p := Normalize(prefix)
	return len(table) > len(p) && strings.HasPrefix(table, p)
}

// CheckOverlap rejects modules whose normalized prefixes are equal or where one
// is a prefix of another: "shop_" would otherwise also select "shop_order_*".
func CheckOverlap(owners []Owner) error {
	sorted := append([]Owner(nil), owners...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Module < sorted[j].Module })
	var conflicts []string
	for i, a := range sorted {
		for _, b := range sorted[i+1:] {
			if a.Module == b.Module {
				continue
			}
			pa, pb := Normalize(a.Prefix), Normalize(b.Prefix)
			if strings.HasPrefix(pa, pb) || strings.HasPrefix(pb, pa) {
				conflicts = append(conflicts, fmt.Sprintf("module %s tablePrefix %q overlaps module %s tablePrefix %q", a.Module, a.Prefix, b.Module, b.Prefix))
			}
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("%s; give each module a table namespace that is not a prefix of another", strings.Join(conflicts, "; "))
	}
	return nil
}
