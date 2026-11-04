package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/quick"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

// fakeDeleter 记录每次 Delete 调用的 ids，便于断言「>1000 时未调用」。
type fakeDeleter struct {
	mu    sync.Mutex
	calls [][]int64
}

func (d *fakeDeleter) Delete(_ context.Context, ids []int64) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	clone := append([]int64(nil), ids...)
	d.calls = append(d.calls, clone)
	return int64(len(ids)), nil
}

func (d *fakeDeleter) CallCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.calls)
}

// 属性 14：批量删除上限。
// 对任意 n：n ≤ 1000 时删除被执行；n > 1000 时请求被拒绝且 deleter 未被调用。
func TestPropertyBatchDeleteLimit(t *testing.T) {
	rng := []int{0, 1, 2, 999, 1000, 1001, 1002, 5000, 100_000}
	for _, n := range rng {
		name := "n_" + itoa(n)
		t.Run(name, func(t *testing.T) {
			deleter := &fakeDeleter{}
			svc := NewCommonExportService(nil, nil)
			svc.RegisterDeleter("test:res", deleter)

			ids := make([]int64, n)
			for i := range ids {
				ids[i] = int64(i + 1)
			}
			_, err := svc.BatchDelete(context.Background(),
				&adminv1.BatchDeleteRequest{Resource: "test:res", Ids: ids})

			if n == 0 {
				// 空列表：InvalidArgument，未调用 deleter。
				if err == nil {
					t.Fatalf("expected error for empty ids")
				}
				if deleter.CallCount() != 0 {
					t.Errorf("deleter should not be called for empty ids")
				}
				return
			}

			if n <= maxBatchDelete {
				// 接受：删除被调用一次，参数与 ids 一致。
				if err != nil {
					t.Fatalf("unexpected err for n=%d: %v", n, err)
				}
				if deleter.CallCount() != 1 {
					t.Fatalf("deleter called %d times, want 1", deleter.CallCount())
				}
				if len(deleter.calls[0]) != n {
					t.Errorf("deleter got %d ids, want %d", len(deleter.calls[0]), n)
				}
			} else {
				// 拒绝：err 为 InvalidArgument，deleter 从未被调用。
				if err == nil {
					t.Fatalf("expected rejection for n=%d", n)
				}
				if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodeInvalidArgument {
					t.Errorf("expected InvalidArgument, got %v", err)
				}
				if deleter.CallCount() != 0 {
					t.Errorf("deleter must NOT be called when n > 1000; got %d calls", deleter.CallCount())
				}
			}
		})
	}

	// 随机属性测试（快速 500 次）：n 在 [0, 2000] 之间随机，验证同样不变式。
	if err := quick.Check(func(n int) bool {
		if n < 0 {
			n = -n
		}
		n = n % 2001 // [0, 2000]
		deleter := &fakeDeleter{}
		svc := NewCommonExportService(nil, nil)
		svc.RegisterDeleter("test:res", deleter)
		ids := make([]int64, n)
		for i := range ids {
			ids[i] = int64(i + 1)
		}
		_, err := svc.BatchDelete(context.Background(),
			&adminv1.BatchDeleteRequest{Resource: "test:res", Ids: ids})
		if n == 0 {
			return err != nil && deleter.CallCount() == 0
		}
		if n <= maxBatchDelete {
			return err == nil && deleter.CallCount() == 1
		}
		// n > 1000
		return err != nil && deleter.CallCount() == 0
	}, &quick.Config{MaxCount: 500}); err != nil {
		t.Fatalf("quick check failed: %v", err)
	}
}

// 属性 14 推论：未知资源类型应返回 InvalidArgument 且不调用 deleter。
func TestBatchDeleteUnknownResource(t *testing.T) {
	svc := NewCommonExportService(nil, nil)
	_, err := svc.BatchDelete(context.Background(),
		&adminv1.BatchDeleteRequest{Resource: "nope:none", Ids: []int64{1}})
	if err == nil {
		t.Fatalf("expected error for unknown resource")
	}
	if ce, ok := asConnectErr(err); !ok || ce.Code() != connect.CodeInvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
	if !errors.Is(err, errBatchDeleteUnknownRes) {
		t.Errorf("expected errBatchDeleteUnknownRes wrap; got %v", err)
	}
}

// itoa 简易整数转字符串（避免引入 strconv 仅此一处）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
