package repo

import (
	"context"

	"gorm.io/gorm"
)

// Keep the formal transaction boundary compile-time stable for generated
// Resource controllers without requiring a live database in this contract test.
var _ func(*Repo, context.Context, func(*Repo) error) error = (*Repo).InTransaction

var _ func(*Repo, *gorm.DB) *Repo = (*Repo).WithDB
