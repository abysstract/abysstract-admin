package repo

import "github.com/abysstract/abysstract-admin/backend/resource"

type Column = resource.Column
type ColumnSet = resource.ColumnSet
type ColumnPolicy = resource.ColumnPolicy

func NewColumnSet(columns ...Column) ColumnSet       { return resource.NewColumnSet(columns...) }
func NewColumnPolicy(columns ...Column) ColumnPolicy { return resource.NewColumnPolicy(columns...) }
