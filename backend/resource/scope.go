package resource

import "gorm.io/gorm"

// AccessScope is an opaque authorization predicate for protected repository
// operations. Its zero value denies all rows. Controllers may pass a scope but
// cannot inspect or convert it into ordinary query conditions.
type AccessScope struct {
	apply        func(*gorm.DB) *gorm.DB
	unrestricted bool
}

// QueryClause is an opaque ordinary query predicate. Its zero value is a
// no-op. Only repository-provided Search, Equal, and Order constructors may
// create non-zero clauses.
type QueryClause struct {
	apply func(*gorm.DB) *gorm.DB
}

// Apply adds the authorization predicate to db. Invalid filters deny every row.
func (s AccessScope) Apply(db *gorm.DB) *gorm.DB {
	if s.apply == nil {
		return db.Where("1 = 0")
	}
	filtered := s.apply(db)
	if filtered == nil {
		return db.Where("1 = 0")
	}
	return filtered
}

// IsNoop reports whether q is the zero no-op clause.
func (q QueryClause) IsNoop() bool { return q.apply == nil }

// Apply adds an ordinary predicate, preserving db for the zero clause.
func (q QueryClause) Apply(db *gorm.DB) *gorm.DB {
	if q.apply == nil {
		return db
	}
	return q.apply(db)
}

// UnrestrictedAccessScope returns the explicit no-row-restriction scope. Only
// generated Controllers of resources with data_scope=false may call it.
func UnrestrictedAccessScope() AccessScope {
	return AccessScope{apply: func(db *gorm.DB) *gorm.DB { return db }, unrestricted: true}
}

// CanUseGlobalCache reports whether scope was created by
// UnrestrictedAccessScope. The private marker cannot be forged outside resource;
// deny-all and every row-level scope must bypass shared cache entries.
func CanUseGlobalCache(scope AccessScope) bool {
	return scope.unrestricted
}

// NewAccessScope wraps a trusted authorization filter. Dynamic scopes always
// use row-level cache semantics, including filters that return all rows.
func NewAccessScope(filter func(*gorm.DB) *gorm.DB) AccessScope {
	return AccessScope{apply: filter}
}
