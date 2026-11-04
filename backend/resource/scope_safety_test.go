package resource

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestAccessScopeApplyAndCachePolicy(t *testing.T) {
	for _, tt := range []struct {
		name           string
		scope          AccessScope
		denied, cached bool
	}{
		{"zero", AccessScope{}, true, false},
		{"nil filter", NewAccessScope(nil), true, false},
		{"nil result", NewAccessScope(func(*gorm.DB) *gorm.DB { return nil }), true, false},
		{"dynamic all", NewAccessScope(func(db *gorm.DB) *gorm.DB { return db }), false, false},
		{"explicit all", UnrestrictedAccessScope(), false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db := dryRunDB(t).Model(&queryContractModel{})
			sql := tt.scope.Apply(db).Find(&[]queryContractModel{}).Statement.SQL.String()
			if strings.Contains(sql, "1 = 0") != tt.denied {
				t.Fatalf("SQL = %s, denied = %v", sql, tt.denied)
			}
			if CanUseGlobalCache(tt.scope) != tt.cached {
				t.Fatalf("cache eligibility = %v", CanUseGlobalCache(tt.scope))
			}
		})
	}
	db := dryRunDB(t).Model(&queryContractModel{}).Where("name = ?", "kept")
	var q QueryClause
	if q.Apply(db) != db {
		t.Fatal("zero clause must preserve the original query")
	}
}

func TestNormalizePageAndColumnPolicy(t *testing.T) {
	for _, tt := range []struct{ num, size, wantNum, wantSize int }{
		{0, 0, 1, 10}, {-1, -1, 1, 10}, {2, 101, 2, 100}, {3, 5, 3, 5},
	} {
		n, s := NormalizePage(tt.num, tt.size)
		if n != tt.wantNum || s != tt.wantSize {
			t.Fatalf("NormalizePage(%d,%d) = %d,%d", tt.num, tt.size, n, s)
		}
	}
	columns := []Column{"name"}
	policy := NewColumnPolicy(columns...)
	columns[0] = "password"
	if err := policy.Validate(NewColumnSet("name")); err != nil {
		t.Fatal(err)
	}
	for _, set := range []ColumnSet{{}, NewColumnSet("password")} {
		if err := policy.Validate(set); err == nil {
			t.Fatalf("expected whitelist rejection for %v", set.Values())
		}
	}
}
