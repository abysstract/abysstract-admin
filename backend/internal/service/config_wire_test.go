//go:build integration

package service

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/proto"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

// This test verifies the production generated Config service composition supports
// the standard-six wire before server bootstrap is switched to it.
func TestGeneratedConfigServiceP24GetBatchDeleteAndOptionalUpdateWire(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc, err := NewGeneratedConfigService(repo.New(gdb))
	if err != nil {
		t.Fatalf("NewGeneratedConfigService: %v", err)
	}
	ctx := context.Background()
	create := func(name, key, value string) int64 {
		t.Helper()
		resp, err := svc.CreateConfig(ctx, &adminv1.CreateConfigRequest{
			ConfigName:  name,
			ConfigKey:   key,
			ConfigValue: value,
		})
		if err != nil {
			t.Fatalf("CreateConfig(%s): %v", key, err)
		}
		return resp.GetItem().GetId()
	}
	firstID := create("参数一", "p20.one", "old")
	secondID := create("参数二", "p20.two", "second")

	got, err := svc.GetConfig(ctx, &adminv1.GetConfigRequest{Id: firstID})
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if got.GetItem().GetConfigKey() != "p20.one" {
		t.Fatalf("GetConfig key = %q, want p20.one", got.GetItem().GetConfigKey())
	}

	updated, err := svc.UpdateConfig(ctx, &adminv1.UpdateConfigRequest{
		Id:          firstID,
		ConfigName:  proto.String("参数一已更新"),
		ConfigValue: proto.String("new"),
		Remark:      proto.String("remark"),
	})
	if err != nil {
		t.Fatalf("UpdateConfig optional wire: %v", err)
	}
	if updated.GetItem().GetConfigValue() != "new" {
		t.Fatalf("UpdateConfig value = %q, want new", updated.GetItem().GetConfigValue())
	}

	if _, err := svc.BatchDeleteConfigs(ctx, &adminv1.BatchDeleteConfigsRequest{Ids: []int64{firstID, secondID, secondID}}); err != nil {
		t.Fatalf("BatchDeleteConfigs: %v", err)
	}
	if _, err := svc.GetConfig(ctx, &adminv1.GetConfigRequest{Id: firstID}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetConfig deleted item code = %v, want NotFound", connect.CodeOf(err))
	}
}
