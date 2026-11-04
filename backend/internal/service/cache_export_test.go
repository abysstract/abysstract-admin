package service

import (
	"context"
	"sync"
	"testing"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// 字典缓存：Set 后命中、Invalidate 后未命中。
func TestDictCacheHitAndInvalidate(t *testing.T) {
	c := newDictCache()
	if _, ok := c.Get("sys_user_sex"); ok {
		t.Fatalf("fresh cache should miss")
	}

	items := []*model.SysDictDatum{{ID: 1, DictType: "sys_user_sex", DictLabel: "男"}}
	c.Set("sys_user_sex", items)

	got, ok := c.Get("sys_user_sex")
	if !ok {
		t.Fatalf("expected hit after Set")
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Errorf("unexpected data: %+v", got)
	}
	hits, misses := c.Stats()
	if hits != 1 || misses != 1 {
		t.Errorf("stats (h,m) = (%d,%d), want (1,1)", hits, misses)
	}

	c.Invalidate("sys_user_sex")
	if _, ok := c.Get("sys_user_sex"); ok {
		t.Errorf("expected miss after Invalidate")
	}
}

// 字典缓存：InvalidateAll 清空所有 key。
func TestDictCacheInvalidateAll(t *testing.T) {
	c := newDictCache()
	c.Set("a", []*model.SysDictDatum{{ID: 1}})
	c.Set("b", []*model.SysDictDatum{{ID: 2}})
	c.InvalidateAll()
	if _, ok := c.Get("a"); ok {
		t.Errorf("a should be invalidated")
	}
	if _, ok := c.Get("b"); ok {
		t.Errorf("b should be invalidated")
	}
}

// Excel 导出：fakeExporter + ExportExcel 走 happy path + 行数上限。
func TestExportExcelHappyPath(t *testing.T) {
	svc := NewCommonExportService(nil, nil)
	svc.RegisterExporter("test:res", &fakeExporter{})

	resp, err := svc.ExportExcel(context.Background(),
		&adminv1.ExportExcelRequest{Resource: "test:res", Keyword: "k"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", resp.RowCount)
	}
	if len(resp.Content) == 0 {
		t.Errorf("expected non-empty xlsx content")
	}
	if resp.Filename == "" {
		t.Errorf("expected non-empty filename")
	}
}

// fakeExporter 用于测试：根据 keyword 决定行数。
type fakeExporter struct{}

func (fakeExporter) Header() []string { return []string{"ID", "Name"} }

func (fakeExporter) Rows(_ context.Context, keyword string, _ int16) ([][]string, error) {
	if keyword == "huge" {
		// 超过 maxExportRows 触发拒绝
		rows := make([][]string, maxExportRows+1)
		for i := range rows {
			rows[i] = []string{"1", "x"}
		}
		return rows, nil
	}
	return [][]string{{"1", "alice"}, {"2", "bob"}}, nil
}

func TestExportExcelRejectsTooMany(t *testing.T) {
	svc := NewCommonExportService(nil, nil)
	svc.RegisterExporter("test:res", &fakeExporter{})
	_, err := svc.ExportExcel(context.Background(),
		&adminv1.ExportExcelRequest{Resource: "test:res", Keyword: "huge"})
	if err == nil {
		t.Fatalf("expected rejection for too many rows")
	}
	if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodeInvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

// 导出未知资源应返回 InvalidArgument。
func TestExportExcelUnknownResource(t *testing.T) {
	svc := NewCommonExportService(nil, nil)
	_, err := svc.ExportExcel(context.Background(),
		&adminv1.ExportExcelRequest{Resource: "nope:none"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodeInvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

// 统计计数和缓存读取可并发执行；-race 必须保持无数据竞争，计数也必须完整。
func TestCacheStatsConcurrentAccess(t *testing.T) {
	const workers = 16
	const iterations = 200

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "dict",
			run: func(t *testing.T) {
				c := newDictCache()
				c.Set("hit", []*model.SysDictDatum{{ID: 1}})
				var wg sync.WaitGroup
				for range workers {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for range iterations {
							_, _ = c.Get("hit")
							_, _ = c.Get("miss")
						}
					}()
				}
				wg.Wait()
				hits, misses := c.Stats()
				if hits != workers*iterations || misses != workers*iterations {
					t.Fatalf("stats (h,m) = (%d,%d), want (%d,%d)", hits, misses, workers*iterations, workers*iterations)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
