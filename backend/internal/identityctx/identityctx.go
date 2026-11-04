package identityctx

import "context"

type contextKey struct{}

type actor struct {
	userID   int64
	username string
}

// Store writes the verified identity values used by the public identity package.
func Store(ctx context.Context, userID int64, username string) context.Context {
	return context.WithValue(ctx, contextKey{}, actor{userID: userID, username: username})
}

// Load returns the verified identity values. Callers receive copies of the
// stored numbers and strings.
func Load(ctx context.Context) (userID int64, username string, ok bool) {
	value, ok := ctx.Value(contextKey{}).(actor)
	if !ok {
		return 0, "", false
	}
	return value.userID, value.username, true
}
