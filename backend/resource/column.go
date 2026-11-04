package resource

import "fmt"

// Column is a validated database column name. Its v1 identity is only the
// DBName; it intentionally carries no Resource provenance.
type Column string

// ColumnSet is an immutable set-like sequence of selective-update columns.
// Construction and reads copy their slices so callers and Hooks cannot mutate
// the columns retained by a Controller.
type ColumnSet struct {
	values []Column
}

// NewColumnSet copies columns into an immutable ColumnSet.
func NewColumnSet(columns ...Column) ColumnSet {
	values := make([]Column, len(columns))
	copy(values, columns)
	return ColumnSet{values: values}
}

// Len returns the number of columns in the set.
func (s ColumnSet) Len() int {
	return len(s.values)
}

// Contains reports whether column is present.
func (s ColumnSet) Contains(column Column) bool {
	for _, candidate := range s.values {
		if candidate == column {
			return true
		}
	}
	return false
}

// Values returns a copy of the columns. Mutating the result never changes s.
func (s ColumnSet) Values() []Column {
	values := make([]Column, len(s.values))
	copy(values, s.values)
	return values
}

// ColumnPolicy is an immutable whitelist of updatable DB columns for one
// resource. The allowed set is private and never exposed to callers.
type ColumnPolicy struct {
	allowed map[Column]struct{}
}

// NewColumnPolicy copies columns into an immutable ColumnPolicy.
func NewColumnPolicy(columns ...Column) ColumnPolicy {
	allowed := make(map[Column]struct{}, len(columns))
	for _, column := range columns {
		allowed[column] = struct{}{}
	}
	return ColumnPolicy{allowed: allowed}
}

// Validate reports whether every column in columns is allowed by the policy.
// An empty ColumnSet is rejected; duplicate detection is the caller's job so
// it can produce resource-scoped diagnostics.
func (p ColumnPolicy) Validate(columns ColumnSet) error {
	if columns.Len() == 0 {
		return fmt.Errorf("update ColumnSet 不能为空")
	}
	for _, column := range columns.Values() {
		if _, ok := p.allowed[column]; !ok {
			return fmt.Errorf("update 列 %q 不在可更新白名单", column)
		}
	}
	return nil
}
