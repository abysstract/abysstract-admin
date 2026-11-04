package testutil

import (
	"testing"

	"github.com/abysstract/abysstract-admin/backend/resource"
)

// Store constructs a borrowed resource Store for an already-owned test session.
// It opens no connection and preserves the exact supplied GORM handle.
func Store(t testing.TB, session resource.Session) *resource.Store {
	t.Helper()
	if session == nil {
		return nil
	}
	store, err := resource.NewStore(session.DB())
	if err != nil {
		t.Fatal(err)
	}
	return store
}
