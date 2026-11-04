package interceptor

import (
	"context"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/identity"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
)

func TestWithClaimsWritesPublicIdentity(t *testing.T) {
	ctx := WithClaims(context.Background(), &jwt.Claims{UserID: 9, Username: "bob"})
	actor, ok := identity.FromContext(ctx)
	if !ok {
		t.Fatal("missing identity")
	}
	if actor.UserID != 9 || actor.Username != "bob" {
		t.Fatalf("%#v", actor)
	}
	claims, ok := ClaimsFromContext(ctx)
	if !ok || claims.UserID != 9 {
		t.Fatalf("claims = %#v", claims)
	}
	actor.Username = "mutated"
	again, _ := identity.FromContext(ctx)
	if again.Username != "bob" {
		t.Fatal("identity mutated")
	}
}
