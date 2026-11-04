package repo

import "github.com/abysstract/abysstract-admin/backend/resource"

type Order = resource.Order

func Search(columns []Column, keyword string) QueryClause { return resource.Search(columns, keyword) }
func Equal(column Column, value any) QueryClause          { return resource.Equal(column, value) }
func OrderClause(order Order) QueryClause                 { return resource.OrderClause(order) }
func StableOrders(orders []Order, key Column) []Order     { return resource.StableOrders(orders, key) }
