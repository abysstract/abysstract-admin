package identity_test

import (
	"context"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/identity"
	"github.com/abysstract/abysstract-admin/backend/internal/identityctx"
)

func TestFromContextAnonymous(t *testing.T) {
	if _, ok := identity.FromContext(context.Background()); ok {
		t.Fatal("anonymous context returned actor")
	}
}

func TestFromContextReturnsCopy(t *testing.T) {
	ctx := identityctx.Store(context.Background(), 7, "alice")
	actor, ok := identity.FromContext(ctx)
	if !ok {
		t.Fatal("missing actor")
	}
	if actor.UserID != 7 || actor.Username != "alice" {
		t.Fatalf("actor = %#v", actor)
	}
	actor.Username = "mutated"
	again, _ := identity.FromContext(ctx)
	if again.Username != "alice" {
		t.Fatalf("context mutated: %#v", again)
	}
}
