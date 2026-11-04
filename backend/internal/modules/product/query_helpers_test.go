package product

import (
	"context"

	"github.com/abysstract/abysstract-admin/backend/resource"
)

type candidateScopeProvider struct{ scope resource.AccessScope }

func (p candidateScopeProvider) DataScope(context.Context, string, string) resource.AccessScope {
	return p.scope
}
