package service

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

// fakePermChecker 根据 allow map 决定是否放行。
type fakePermChecker struct {
	allow map[string]bool
	calls []string
}

func (f *fakePermChecker) check(_ context.Context, perm string) error {
	f.calls = append(f.calls, perm)
	if f.allow[perm] {
		return nil
	}
	return connect.NewError(connect.CodePermissionDenied, "缺少权限: "+perm)
}

// TestCommonExportResourcePerm 校验三个通用 RPC 在 service 层按 resource 派发前
// 校验对应权限（:remove / :list / :add）。
func TestCommonExportResourcePerm(t *testing.T) {
	t.Run("BatchDelete 校验 system:user:remove", func(t *testing.T) {
		chk := &fakePermChecker{allow: map[string]bool{}}
		svc := NewCommonExportService(nil, chk.check)
		svc.RegisterDeleter("system:user", &fakeDeleter{})

		_, err := svc.BatchDelete(context.Background(),
			&adminv1.BatchDeleteRequest{Resource: "system:user", Ids: []int64{1}})
		if err == nil {
			t.Fatal("无权限时应返回错误")
		}
		if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodePermissionDenied {
			t.Errorf("want PermissionDenied, got %v", err)
		}
		if len(chk.calls) != 1 || chk.calls[0] != "system:user:remove" {
			t.Errorf("应校验 system:user:remove, got %v", chk.calls)
		}
	})

	t.Run("ExportExcel 校验 system:user:list", func(t *testing.T) {
		chk := &fakePermChecker{allow: map[string]bool{}}
		svc := NewCommonExportService(nil, chk.check)
		svc.RegisterExporter("system:user", &stubExporter{})

		_, err := svc.ExportExcel(context.Background(),
			&adminv1.ExportExcelRequest{Resource: "system:user"})
		if err == nil {
			t.Fatal("无权限时应返回错误")
		}
		if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodePermissionDenied {
			t.Errorf("want PermissionDenied, got %v", err)
		}
		if len(chk.calls) != 1 || chk.calls[0] != "system:user:list" {
			t.Errorf("应校验 system:user:list, got %v", chk.calls)
		}
	})

	t.Run("ImportExcel 校验 system:user:add", func(t *testing.T) {
		chk := &fakePermChecker{allow: map[string]bool{}}
		svc := NewCommonExportService(nil, chk.check)
		svc.RegisterImporter("system:user", &stubImporter{})

		_, err := svc.ImportExcel(context.Background(),
			&adminv1.ImportExcelRequest{Resource: "system:user", Content: []byte("fake")})
		if err == nil {
			t.Fatal("无权限时应返回错误")
		}
		if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodePermissionDenied {
			t.Errorf("want PermissionDenied, got %v", err)
		}
		if len(chk.calls) != 1 || chk.calls[0] != "system:user:add" {
			t.Errorf("应校验 system:user:add, got %v", chk.calls)
		}
	})

	t.Run("有权限时放行并进入资源派发", func(t *testing.T) {
		chk := &fakePermChecker{allow: map[string]bool{"system:user:remove": true}}
		svc := NewCommonExportService(nil, chk.check)
		deleter := &fakeDeleter{}
		svc.RegisterDeleter("system:user", deleter)

		_, err := svc.BatchDelete(context.Background(),
			&adminv1.BatchDeleteRequest{Resource: "system:user", Ids: []int64{1, 2}})
		if err != nil {
			t.Fatalf("有权限时应放行, got %v", err)
		}
		if deleter.CallCount() != 1 {
			t.Errorf("deleter 应被调用 1 次, got %d", deleter.CallCount())
		}
	})

	t.Run("NewPermChecker 无 claims → Unauthenticated", func(t *testing.T) {
		chk := NewPermChecker(func(context.Context, int64) ([]string, error) { return nil, nil })
		err := chk(context.Background(), "system:user:remove")
		if err == nil {
			t.Fatal("应返回错误")
		}
		if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodeUnauthenticated {
			t.Errorf("want Unauthenticated, got %v", err)
		}
	})

	t.Run("NewPermChecker 有 claims 有权限放行", func(t *testing.T) {
		chk := NewPermChecker(func(context.Context, int64) ([]string, error) {
			return []string{"system:user:remove"}, nil
		})
		ctx := interceptor.WithClaims(context.Background(), &jwt.Claims{UserID: 1})
		if err := chk(ctx, "system:user:remove"); err != nil {
			t.Fatalf("有权限应放行, got %v", err)
		}
	})

	t.Run("NewPermChecker 有 claims 无权限拒绝", func(t *testing.T) {
		chk := NewPermChecker(func(context.Context, int64) ([]string, error) {
			return []string{"system:user:list"}, nil
		})
		ctx := interceptor.WithClaims(context.Background(), &jwt.Claims{UserID: 1})
		err := chk(ctx, "system:user:remove")
		if err == nil {
			t.Fatal("无权限应拒绝")
		}
		if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodePermissionDenied {
			t.Errorf("want PermissionDenied, got %v", err)
		}
	})
}

// stubExporter / stubImporter 供权限测试使用（不实际执行导出导入）。
type stubExporter struct{}

func (e *stubExporter) Header() []string { return []string{"ID"} }
func (e *stubExporter) Rows(context.Context, string, int16) ([][]string, error) {
	return [][]string{{"1"}}, nil
}

type stubImporter struct{}

func (i *stubImporter) Header() []string { return []string{"username"} }
func (i *stubImporter) Import(_ context.Context, rows [][]string) ([]importRowResult, error) {
	out := make([]importRowResult, len(rows))
	for i := range out {
		out[i] = importRowResult{ok: true}
	}
	return out, nil
}

func TestCommonExportDoesNotRegisterUserBatchDelete(t *testing.T) {
	svc := NewCommonExportService(repo.New(nil), nil)
	if _, ok := svc.deleters["system:user"]; ok {
		t.Fatal("CommonExport must not own the User batch-delete write path")
	}
}
