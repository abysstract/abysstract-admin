package resource

import (
	"testing"

	"gorm.io/gorm"
)

func TestZeroAccessScopeIsDenyAllAndZeroQueryClauseIsNoop(t *testing.T) {
	var access AccessScope
	if CanUseGlobalCache(access) {
		t.Fatalf("zero AccessScope must deny all, got %#v", access)
	}
	var query QueryClause
	if !query.IsNoop() {
		t.Fatalf("zero QueryClause must be a no-op, got %#v", query)
	}
}

func TestCanUseGlobalCacheOnlyForUnrestrictedScope(t *testing.T) {
	if CanUseGlobalCache(AccessScope{}) {
		t.Fatal("zero deny-all scope must not use global cache")
	}
	rowLevel := NewAccessScope(func(db *gorm.DB) *gorm.DB { return db.Where("id = ?", 1) })
	if CanUseGlobalCache(rowLevel) {
		t.Fatal("row-level scope must not use global cache")
	}
	if !CanUseGlobalCache(UnrestrictedAccessScope()) {
		t.Fatal("explicit unrestricted scope must use global cache")
	}
}
