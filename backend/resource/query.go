package resource

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Order is a structured, comparable sort item resolved from the Sortable
// whitelist. It is produced only from Build-generated column constants or
// whitelist-validated DBNames; it never retains the raw proto OrderBy.
type Order struct {
	Column Column
	Desc   bool
}

// escapeLike escapes LIKE metacharacters so keyword is matched literally.
func escapeLike(keyword string) string {
	r := strings.NewReplacer(`!`, `!!`, `%`, `!%`, `_`, `!_`)
	return r.Replace(keyword)
}

// Search builds a parameterized OR LIKE clause over the given searchable DB
// columns. Empty columns or a keyword that trims to empty produce a zero-value
// no-op clause. Constructors never fail: illegal columns are rejected before
// construction (OrderBy.field whitelist at the Controller boundary, or
// Build-generated column constants inside adapters).
func Search(columns []Column, keyword string) QueryClause {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || len(columns) == 0 {
		return QueryClause{}
	}
	pattern := "%" + escapeLike(keyword) + "%"
	names := make([]string, len(columns))
	for i, c := range columns {
		names[i] = string(c)
	}
	return QueryClause{apply: func(db *gorm.DB) *gorm.DB {
		exprs := make([]clause.Expression, len(names))
		for i, name := range names {
			// 需求固定为参数化 LIKE；值走绑定变量，列名来自白名单/列常量。
			exprs[i] = clause.Expr{SQL: "LOWER(?) LIKE LOWER(?) ESCAPE '!'", Vars: []any{clause.Column{Name: name}, pattern}, WithoutParentheses: true}
		}
		return db.Where(clause.Or(exprs...))
	}}
}

// Equal builds a parameterized equality clause on one whitelisted DB column.
// Presence semantics live at the Controller: a nil optional request field
// means "no filter" and never reaches this constructor.
func Equal(column Column, value any) QueryClause {
	name := string(column)
	return QueryClause{apply: func(db *gorm.DB) *gorm.DB {
		return db.Where(clause.Eq{Column: clause.Column{Name: name}, Value: value})
	}}
}

// OrderClause builds an ORDER BY item from a whitelist-resolved Order.
func OrderClause(order Order) QueryClause {
	return QueryClause{apply: func(db *gorm.DB) *gorm.DB {
		return db.Order(clause.OrderByColumn{
			Column: clause.Column{Name: string(order.Column)},
			Desc:   order.Desc,
		})
	}}
}

// StableOrders returns a private copy of the requested orders with the
// resource key appended in ascending order only when the request did not
// already order by that key. It preserves request order and direction while
// making pagination deterministic without duplicate ORDER BY columns.
func StableOrders(orders []Order, key Column) []Order {
	stable := append([]Order(nil), orders...)
	for _, order := range stable {
		if order.Column == key {
			return stable
		}
	}
	return append(stable, Order{Column: key})
}
