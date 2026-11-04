package identity

import (
	"context"

	"github.com/abysstract/abysstract-admin/backend/internal/identityctx"
)

// Actor is a copy of the authenticated user for the current request.
type Actor struct {
	UserID   int64
	Username string
}

// FromContext returns the authenticated actor. Anonymous requests return false.
func FromContext(ctx context.Context) (Actor, bool) {
	userID, username, ok := identityctx.Load(ctx)
	if !ok {
		return Actor{}, false
	}
	return Actor{UserID: userID, Username: username}, true
}
