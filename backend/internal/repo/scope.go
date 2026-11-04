package repo

import (
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/resource"
)

type AccessScope = resource.AccessScope
type QueryClause = resource.QueryClause

func NewAccessScope(filter func(*gorm.DB) *gorm.DB) AccessScope {
	return resource.NewAccessScope(filter)
}
func UnrestrictedAccessScope() AccessScope     { return resource.UnrestrictedAccessScope() }
func CanUseGlobalCache(scope AccessScope) bool { return resource.CanUseGlobalCache(scope) }
