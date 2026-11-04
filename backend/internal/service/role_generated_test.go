package service

import (
	"context"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/resource"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
)

func TestRoleHooksPreserveDefaultsAndDomainBoundaries(t *testing.T) {
	hooks := newRoleHooks()
	created := &model.SysRole{}
	if err := hooks.BeforeCreate(context.Background(), resource.Tx{}, created); err != nil {
		t.Fatalf("default Role create rejected: %v", err)
	}
	if created.Status != 1 || created.DataScope != 1 {
		t.Fatalf("default Role create = status=%d data_scope=%d, want 1/1", created.Status, created.DataScope)
	}

	for _, value := range []int16{1, 2, 3, 4, 5} {
		role := &model.SysRole{DataScope: value}
		if err := hooks.BeforeCreate(context.Background(), resource.Tx{}, role); err != nil {
			t.Fatalf("valid Role data_scope %d rejected: %v", value, err)
		}
	}
	for _, value := range []int16{-1, 6} {
		role := &model.SysRole{DataScope: value}
		err := hooks.BeforeCreate(context.Background(), resource.Tx{}, role)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("invalid Role data_scope %d code = %v err=%v, want InvalidArgument", value, connect.CodeOf(err), err)
		}
	}
	if err := repo.ValidateRoleStatus(1); err != nil {
		t.Fatalf("enabled Role status rejected: %v", err)
	}
	if err := repo.ValidateRoleStatus(3); err == nil {
		t.Fatal("unknown Role status unexpectedly accepted")
	}
	if err := repo.ValidateRoleDelete(&model.SysRole{RoleKey: repo.SuperAdminRoleKey}); err != repo.ErrProtectedRole {
		t.Fatalf("super-admin Role delete guard = %v, want %v", err, repo.ErrProtectedRole)
	}
	if err := repo.ValidateRoleDelete(&model.SysRole{RoleKey: "operator"}); err != nil {
		t.Fatalf("ordinary Role delete guard rejected: %v", err)
	}
	if _, err := roleStatusValue(adminv1.Status(65537)); err == nil {
		t.Fatal("out-of-range Role status unexpectedly narrowed into a valid value")
	}
	if _, err := NewGeneratedRoleService(nil); err == nil {
		t.Fatal("NewGeneratedRoleService(nil) unexpectedly succeeded")
	}
	if err := resourcegen.RoleValidateUpdateColumns(repo.NewColumnSet(resourcegen.RoleColumnDataScope)); err != nil {
		t.Fatalf("generated Role data_scope ColumnPolicy rejected valid column: %v", err)
	}
}
