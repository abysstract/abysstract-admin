package repo

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestPublicResourceErrorsRetainIdentity(t *testing.T) {
	for _, pair := range [][2]error{
		{ErrNotFound, resource.ErrNotFound},
		{ErrEmptyPatch, resource.ErrEmptyPatch},
		{ErrInvalidUpdateColumns, resource.ErrInvalidUpdateColumns},
		{ErrDuplicate, resource.ErrDuplicate},
		{ErrCycle, resource.ErrCycle},
		{ErrHasChildren, resource.ErrHasChildren},
	} {
		if !errors.Is(fmt.Errorf("wrapped: %w", pair[0]), pair[1]) || pair[0] != pair[1] {
			t.Fatalf("error identity mismatch: %v, %v", pair[0], pair[1])
		}
	}
	for _, err := range []error{nil, errors.New("connection failure"), &pgconn.PgError{Code: "23505"}} {
		if got, want := wrapDBErr(err), resource.NormalizeDBError(err); got != want {
			t.Fatalf("normalization mismatch: %v, %v", got, want)
		}
	}
}

var (
	_ resource.Session     = (*Repo)(nil)
	_ resource.ColumnSet   = NewColumnSet("name")
	_ resource.AccessScope = UnrestrictedAccessScope()
	_ resource.QueryClause = Equal("id", 1)
)
