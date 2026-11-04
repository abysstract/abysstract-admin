//go:build integration

package repo

import (
	"context"
	"reflect"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

func TestLiteralLogSearch(t *testing.T) {
	r := newRepositoryFixture(t)
	ctx := context.Background()
	texts := []string{`rate 50% complete`, `rate 500 complete`, `value a_b`, `value axb`, `path a\b`, `path ab`, `Alpha 中文`, `unrelated`, `mark a!b`, `mark ab`, `mark a!%_b`}
	for i, text := range texts {
		if err := r.AddOperLog(ctx, &model.SysOperLog{ID: int64(9000 + i), Title: text}); err != nil {
			t.Fatal(err)
		}
		if err := r.AddLoginLog(ctx, &model.SysLoginLog{ID: int64(9000 + i), Username: text}); err != nil {
			t.Fatal(err)
		}
	}
	for keyword, want := range map[string][]int64{"50%": {9000}, "a_b": {9002}, `a\b`: {9004}, "alpha": {9006}, "ALPHA": {9006}, "中文": {9006}, "a!b": {9008}, "a!%_b": {9010}} {
		logs, n, err := r.ListOperLogs(ctx, keyword, 1, 100)
		if err != nil {
			t.Fatal(err)
		}
		ids := []int64{}
		for _, row := range logs {
			ids = append(ids, row.ID)
		}
		if n != int64(len(want)) || !reflect.DeepEqual(ids, want) {
			t.Fatalf("oper %q: %v %d", keyword, ids, n)
		}
		logins, n, err := r.ListLoginLogs(ctx, keyword, 1, 100)
		if err != nil {
			t.Fatal(err)
		}
		ids = []int64{}
		for _, row := range logins {
			ids = append(ids, row.ID)
		}
		if n != int64(len(want)) || !reflect.DeepEqual(ids, want) {
			t.Fatalf("login %q: %v %d", keyword, ids, n)
		}
	}
}
