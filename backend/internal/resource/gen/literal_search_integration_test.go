//go:build integration

package gen

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestResourceLiteralSearchAndScope(t *testing.T) {
	a, r := setupRepo(t)
	ctx := context.Background()
	stamp := resource.NormalizeTimestamp(time.Now())
	texts := []string{`rate 50% complete`, `rate 500 complete`, `value a_b`, `value axb`, `path a\b`, `path ab`, `Alpha 中文`, `unrelated`, `mark a!b`, `mark ab`, `mark a!%_b`}
	for i, text := range texts {
		if err := a.Create(ctx, r, &model.SysDictType{ID: int64(9000 + i), CreateTime: stamp, DictName: text, DictType: fmt.Sprintf("p3-search-%d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	scope := resource.NewAccessScope(func(db *gorm.DB) *gorm.DB { return db.Where("id>=9000") })
	for word, want := range map[string][]int64{"50%": {9000}, "a_b": {9002}, `a\b`: {9004}, "alpha": {9006}, "ALPHA": {9006}, "中文": {9006}, "a!b": {9008}, "a!%_b": {9010}} {
		rows, n, err := a.List(ctx, r, DictTypeListQuery{Keyword: word, PageSize: 100}, scope)
		if err != nil {
			t.Fatal(err)
		}
		ids := []int64{}
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		if n != int64(len(want)) || !reflect.DeepEqual(ids, want) {
			t.Fatalf("word=%q ids=%v total=%d", word, ids, n)
		}
	}
	if err := a.Create(ctx, r, &model.SysDictType{ID: 9011, CreateTime: stamp, DictName: "second field", DictType: "p3-search-ALPHA"}); err != nil {
		t.Fatal(err)
	}
	rows, n, err := a.List(ctx, r, DictTypeListQuery{Keyword: "alpha", PageSize: 100}, scope)
	if err != nil || n != 2 || len(rows) != 2 {
		t.Fatalf("OR fields %d %v", n, err)
	}
	denied := resource.NewAccessScope(func(db *gorm.DB) *gorm.DB { return db.Where("id=9000") })
	rows, n, err = a.List(ctx, r, DictTypeListQuery{Keyword: "alpha", PageSize: 100}, denied)
	if err != nil || n != 0 || len(rows) != 0 {
		t.Fatalf("OR escaped scope %d %v", n, err)
	}
	for i := 0; i < 3; i++ {
		rows, n, err = a.List(ctx, r, DictTypeListQuery{PageNum: 2, PageSize: 1, Orders: []resource.Order{{Column: "create_time"}}}, scope)
		if err != nil || n != 12 || len(rows) != 1 || rows[0].ID != 9001 {
			t.Fatalf("pagination %d %v", n, err)
		}
	}
}
