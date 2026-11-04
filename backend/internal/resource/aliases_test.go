package resource

import (
	"context"
	"errors"
	"testing"

	public "github.com/abysstract/abysstract-admin/backend/resource"
)

type aliasModel struct{}

var _ *public.HookSet[aliasModel] = (*HookSet[aliasModel])(nil)
var _ public.CreateScopeValidator[aliasModel] = CreateScopeValidator[aliasModel](func(context.Context, public.Tx, *aliasModel) error { return nil })
var _ public.ReadRepository[aliasModel, int] = (ReadRepository[aliasModel, int])(nil)

func TestScopeErrorAliasPreservesIdentity(t *testing.T) {
	if ErrScopeDenied != public.ErrScopeDenied || !errors.Is(ErrScopeDenied, public.ErrScopeDenied) {
		t.Fatal("scope error identity changed")
	}
}
