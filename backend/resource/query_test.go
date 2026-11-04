package resource

import (
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// dryRunDB returns a GORM session that compiles SQL without a live connection.
func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: nil}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("postgres dialector without conn: %v", err)
	}
	return db.Session(&gorm.Session{DryRun: true})
}

type queryContractModel struct {
	ID   int64
	Name string
}

func (queryContractModel) TableName() string { return "t_query_contract" }

func TestSearchBuildsEscapedParameterizedOrLike(t *testing.T) {
	db := dryRunDB(t)
	q := Search([]Column{"name", "id"}, `  50%_x\  `)
	if q.IsNoop() {
		t.Fatal("non-empty keyword Search must produce a non-zero clause")
	}
	stmt := q.Apply(db.Model(&queryContractModel{})).Find(&[]queryContractModel{}).Statement
	sql := stmt.SQL.String()
	if !strings.Contains(sql, "LOWER(") || !strings.Contains(strings.ToUpper(sql), " OR ") {
		t.Fatalf("Search SQL must be OR LOWER/LIKE, got: %s", sql)
	}
	for _, v := range stmt.Vars {
		s, _ := v.(string)
		if s != `%50!%!_x\%` {
			t.Fatalf("keyword metacharacters must be escaped literally, got var %q", s)
		}
	}
	// 客户端关键字必须参数化，不得直接拼进 SQL。
	if strings.Contains(sql, "50") {
		t.Fatalf("keyword must be parameterized, got SQL: %s", sql)
	}
}

func TestSearchEmptyInputIsNoop(t *testing.T) {
	for name, q := range map[string]QueryClause{
		"empty columns": Search(nil, "kw"),
		"empty keyword": Search([]Column{"name"}, "   "),
	} {
		if !q.IsNoop() {
			t.Fatalf("%s must be a zero no-op clause", name)
		}
	}
}

func TestEqualBuildsParameterizedEquality(t *testing.T) {
	db := dryRunDB(t)
	q := Equal(Column("name"), "abc'; DROP TABLE t;--")
	stmt := q.Apply(db.Model(&queryContractModel{})).Find(&[]queryContractModel{}).Statement
	if !strings.Contains(stmt.SQL.String(), `"name" =`) {
		t.Fatalf("Equal SQL must compare the column, got: %s", stmt.SQL.String())
	}
	if strings.Contains(stmt.SQL.String(), "DROP") {
		t.Fatalf("value must be parameterized, got SQL: %s", stmt.SQL.String())
	}
	if len(stmt.Vars) != 1 {
		t.Fatalf("Equal must bind exactly one value, got %v", stmt.Vars)
	}
}

func TestOrderClauseBuildsDirectionalOrderBy(t *testing.T) {
	db := dryRunDB(t)
	asc := OrderClause(Order{Column: "id"})
	if sql := asc.Apply(db.Model(&queryContractModel{})).Find(&[]queryContractModel{}).Statement.SQL.String(); !strings.Contains(sql, `ORDER BY "id"`) || strings.Contains(sql, "DESC") {
		t.Fatalf("asc order SQL wrong: %s", sql)
	}
	desc := OrderClause(Order{Column: "name", Desc: true})
	if sql := desc.Apply(db.Model(&queryContractModel{})).Find(&[]queryContractModel{}).Statement.SQL.String(); !strings.Contains(sql, `ORDER BY "name" DESC`) {
		t.Fatalf("desc order SQL wrong: %s", sql)
	}
}
