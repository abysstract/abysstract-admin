package resource

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
)

type composedModel struct {
	ID   int64
	Name string
}

func TestComposeTransactionCallbacks(t *testing.T) {
	ctx := context.Background()
	old, m := &composedModel{Name: "old"}, &composedModel{Name: "new"}
	columns := NewColumnSet("name")
	models := []*composedModel{old, m}
	cases := []struct {
		name    string
		after   bool
		install func(*HookSet[composedModel], func(context.Context, Tx) error)
		invoke  func(*HookSet[composedModel]) error
	}{
		{"BeforeCreate", false, func(h *HookSet[composedModel], f func(context.Context, Tx) error) {
			h.BeforeCreate = func(c context.Context, tx Tx, x *composedModel) error {
				if x != m {
					t.Fatal("create model identity")
				}
				return f(c, tx)
			}
		}, func(h *HookSet[composedModel]) error { return h.BeforeCreate(ctx, Tx{}, m) }},
		{"AfterCreate", true, func(h *HookSet[composedModel], f func(context.Context, Tx) error) {
			h.AfterCreate = func(c context.Context, tx Tx, x *composedModel) error {
				if x != m {
					t.Fatal("create model identity")
				}
				return f(c, tx)
			}
		}, func(h *HookSet[composedModel]) error { return h.AfterCreate(ctx, Tx{}, m) }},
		{"BeforeUpdate", false, func(h *HookSet[composedModel], f func(context.Context, Tx) error) {
			h.BeforeUpdate = func(c context.Context, tx Tx, a, b *composedModel, s ColumnSet) error {
				if a != old || b != m || !reflect.DeepEqual(s.Values(), columns.Values()) {
					t.Fatal("update argument identity")
				}
				return f(c, tx)
			}
		}, func(h *HookSet[composedModel]) error { return h.BeforeUpdate(ctx, Tx{}, old, m, columns) }},
		{"AfterUpdate", true, func(h *HookSet[composedModel], f func(context.Context, Tx) error) {
			h.AfterUpdate = func(c context.Context, tx Tx, a, b *composedModel, s ColumnSet) error {
				if a != old || b != m || !reflect.DeepEqual(s.Values(), columns.Values()) {
					t.Fatal("update argument identity")
				}
				return f(c, tx)
			}
		}, func(h *HookSet[composedModel]) error { return h.AfterUpdate(ctx, Tx{}, old, m, columns) }},
		{"BeforeDelete", false, func(h *HookSet[composedModel], f func(context.Context, Tx) error) {
			h.BeforeDelete = func(c context.Context, tx Tx, x *composedModel) error {
				if x != m {
					t.Fatal("delete model identity")
				}
				return f(c, tx)
			}
		}, func(h *HookSet[composedModel]) error { return h.BeforeDelete(ctx, Tx{}, m) }},
		{"AfterDelete", true, func(h *HookSet[composedModel], f func(context.Context, Tx) error) {
			h.AfterDelete = func(c context.Context, tx Tx, x *composedModel) error {
				if x != m {
					t.Fatal("delete model identity")
				}
				return f(c, tx)
			}
		}, func(h *HookSet[composedModel]) error { return h.AfterDelete(ctx, Tx{}, m) }},
		{"BeforeBatchDelete", false, func(h *HookSet[composedModel], f func(context.Context, Tx) error) {
			h.BeforeBatchDelete = func(c context.Context, tx Tx, x []*composedModel) error {
				if &x[0] != &models[0] {
					t.Fatal("batch argument identity")
				}
				return f(c, tx)
			}
		}, func(h *HookSet[composedModel]) error { return h.BeforeBatchDelete(ctx, Tx{}, models) }},
		{"AfterBatchDelete", true, func(h *HookSet[composedModel], f func(context.Context, Tx) error) {
			h.AfterBatchDelete = func(c context.Context, tx Tx, x []*composedModel) error {
				if &x[0] != &models[0] {
					t.Fatal("batch argument identity")
				}
				return f(c, tx)
			}
		}, func(h *HookSet[composedModel]) error { return h.AfterBatchDelete(ctx, Tx{}, models) }},
	}
	sentinel := errors.New("callback error")
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			for _, failAt := range []string{"", "system", "business"} {
				var order []string
				a, b := &HookSet[composedModel]{}, &HookSet[composedModel]{}
				makeFn := func(name string) func(context.Context, Tx) error {
					return func(c context.Context, tx Tx) error {
						if c != ctx || tx.DB() != nil {
							t.Fatal("context or Tx changed")
						}
						order = append(order, name)
						if failAt == name {
							return sentinel
						}
						return nil
					}
				}
				tt.install(a, makeFn("system"))
				tt.install(b, makeFn("business"))
				combined := ComposeHooks(a, b)
				// The composition owns callback fields, independently of later assignment.
				tt.install(a, func(context.Context, Tx) error { t.Fatal("retained mutable HookSet pointer"); return nil })
				err := tt.invoke(combined)
				want := []string{"system", "business"}
				if tt.after {
					want = []string{"business", "system"}
				}
				if failAt == want[0] {
					want = want[:1]
				}
				if !reflect.DeepEqual(order, want) {
					t.Fatalf("failAt=%q order=%v want=%v", failAt, order, want)
				}
				if (err != nil) != (failAt != "") || err != nil && !errors.Is(err, sentinel) {
					t.Fatalf("error identity: %v", err)
				}
			}
		})
	}
}

func TestComposeCommittedCallbacksPanicAndSnapshots(t *testing.T) {
	var logs bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(original)
	ctx := context.Background()
	var calls []string
	a, b := &HookSet[composedModel]{}, &HookSet[composedModel]{}
	a.AfterCommitCreate = func(context.Context, composedModel) { calls = append(calls, "system-create"); panic("first") }
	b.AfterCommitCreate = func(context.Context, composedModel) { calls = append(calls, "business-create"); panic("second") }
	a.AfterCommitUpdate = func(context.Context, composedModel) { calls = append(calls, "system-update"); panic("first") }
	b.AfterCommitUpdate = func(context.Context, composedModel) { calls = append(calls, "business-update"); panic("second") }
	a.AfterCommitDelete = func(context.Context, composedModel) { calls = append(calls, "system-delete"); panic("first") }
	b.AfterCommitDelete = func(context.Context, composedModel) { calls = append(calls, "business-delete"); panic("second") }
	input := []composedModel{{Name: "snapshot"}}
	a.AfterCommitBatchDelete = func(_ context.Context, x []composedModel) {
		calls = append(calls, "system-batch")
		x[0].Name = "system"
		panic("first")
	}
	b.AfterCommitBatchDelete = func(_ context.Context, x []composedModel) {
		calls = append(calls, "business-batch")
		if x[0].Name != "snapshot" {
			t.Fatal("batch snapshots share storage")
		}
		x[0].Name = "business"
		panic("second")
	}
	combined := ComposeHooks(a, b)
	combined.AfterCommitCreate(ctx, composedModel{})
	combined.AfterCommitUpdate(ctx, composedModel{})
	combined.AfterCommitDelete(ctx, composedModel{})
	combined.AfterCommitBatchDelete(ctx, input)
	want := []string{"system-create", "business-create", "system-update", "business-update", "system-delete", "business-delete", "system-batch", "business-batch"}
	if !reflect.DeepEqual(calls, want) || input[0].Name != "snapshot" {
		t.Fatalf("calls=%v input=%v", calls, input)
	}
	if strings.Count(logs.String(), "resource after-commit hook panicked") != 8 {
		t.Fatalf("panic log count: %s", logs.String())
	}
}

func TestComposeEmptyAndOneSidedHooks(t *testing.T) {
	empty := ComposeHooks[composedModel](nil, nil)
	if !reflect.DeepEqual(*empty, HookSet[composedModel]{}) {
		t.Fatal("empty callbacks must remain optional")
	}
	calls := 0
	one := &HookSet[composedModel]{BeforeCreate: func(context.Context, Tx, *composedModel) error { calls++; return nil }}
	for _, h := range []*HookSet[composedModel]{ComposeHooks(one, nil), ComposeHooks(nil, one)} {
		if err := h.BeforeCreate(context.Background(), Tx{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatalf("single-sided callbacks called %d times", calls)
	}
}
