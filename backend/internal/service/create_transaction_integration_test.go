//go:build integration

package service

import (
	"context"
	"errors"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestCreateRollbackKeepsConfigCache(t *testing.T) {
	fail := false
	sentinel := errors.New("create hook failed")
	svc, counter := setupGeneratedConfigAdapter(t, func(h *resource.HookSet[model.SysConfig]) {
		h.AfterCreate = func(context.Context, resource.Tx, *model.SysConfig) error {
			if fail {
				return sentinel
			}
			return nil
		}
	})
	seedGeneratedConfigFixture(t, svc)
	warmGeneratedConfigList(t, svc, counter, 1)
	fail = true
	_, err := svc.CreateConfig(context.Background(), &adminv1.CreateConfigRequest{ConfigName: "rolled back", ConfigKey: "config.create.rollback", ConfigValue: "value"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("create: %v", err)
	}
	warmGeneratedConfigList(t, svc, counter, 1)
}
