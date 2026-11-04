package gen

import (
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

// 10.2b 契约：生成的 ListQuery/adapter 与冻结的公共 API 形状一致。
func TestDictTypeListQueryShape(t *testing.T) {
	q := DictTypeListQuery{PageNum: 2, PageSize: 20, Keyword: "  审计  ", Orders: []repo.Order{{Column: DictTypeColumnId}}}
	if q.PageNum != 2 || q.PageSize != 20 || q.Keyword != "  审计  " {
		t.Fatalf("query fields wrong: %#v", q)
	}
	if len(q.Orders) != 1 || q.Orders[0].Desc {
		t.Fatalf("orders must be []repo.Order in request order: %#v", q.Orders)
	}
}

func TestDictTypeResolveOrderWhitelist(t *testing.T) {
	// DictType v1 无 Sortable 字段：任何字段名都必须被拒绝。
	if _, err := DictTypeResolveOrder("id", false); err == nil {
		t.Fatal("unknown sort field must be rejected")
	}
	if _, err := DictTypeResolveOrder("'; DROP TABLE sys_dict_type;--", true); err == nil {
		t.Fatal("injected field must be rejected")
	}
}

func TestDictTypeValidateUpdateColumns(t *testing.T) {
	cases := []struct {
		name    string
		columns repo.ColumnSet
		wantErr bool
	}{
		{"empty", repo.NewColumnSet(), true},
		{"duplicate", repo.NewColumnSet(DictTypeColumnDictName, DictTypeColumnDictName), true},
		{"key column", repo.NewColumnSet(DictTypeColumnId), true},
		{"server managed", repo.NewColumnSet(DictTypeColumnUpdateTime), true},
		{"unknown column", repo.NewColumnSet(repo.Column("dict_type")), true}, // CreateOnly 不可更新
		{"ok", repo.NewColumnSet(DictTypeColumnDictName, DictTypeColumnRemark), false},
		{"same DBName from any const", repo.NewColumnSet(repo.Column("dict_name")), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := DictTypeValidateUpdateColumns(tc.columns)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// Column identity is its DBName rather than a resource
// provenance marker. The same DBName constants from different Resources must
// therefore be accepted or rejected according to the receiving policy only.
func TestColumnPolicyUsesDBNameAcrossResources(t *testing.T) {
	if DictTypeColumnRemark != ConfigColumnRemark {
		t.Fatalf("fixture requires shared DBName: DictType=%q Config=%q", DictTypeColumnRemark, ConfigColumnRemark)
	}
	if err := DictTypeValidateUpdateColumns(repo.NewColumnSet(ConfigColumnRemark)); err != nil {
		t.Fatalf("DictType policy rejected Config remark DBName: %v", err)
	}
	if err := ConfigValidateUpdateColumns(repo.NewColumnSet(DictTypeColumnRemark)); err != nil {
		t.Fatalf("Config policy rejected DictType remark DBName: %v", err)
	}
	if err := ConfigValidateUpdateColumns(repo.NewColumnSet(DictDataColumnRemark)); err != nil {
		t.Fatalf("Config policy rejected DictData remark DBName: %v", err)
	}
}
