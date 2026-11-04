//go:build integration

package service

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/proto"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

// 生成校验返回普通 error；正式 generated Dict service 必须在访问 repo 前
// 统一映射为 RPC InvalidArgument。
func TestGeneratedDictServiceWrapsValidationErrorsAsInvalidArgument(t *testing.T) {
	h := setupGeneratedDictHarness(t, generatedDictHookMutators{})
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "create",
			call: func() error {
				_, err := h.svc.CreateDictData(ctx, &adminv1.CreateDictDataRequest{
					DictType: "category", DictLabel: "", DictValue: "value",
				})
				return err
			},
		},
		{
			name: "update",
			call: func() error {
				_, err := h.svc.UpdateDictData(ctx, &adminv1.UpdateDictDataRequest{
					Id: 1, DictLabel: proto.String(""),
				})
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil || connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("error = %v, want InvalidArgument", err)
			}
		})
	}
}
