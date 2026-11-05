package modelcfg

import (
	"testing"

	"gorm.io/gorm"
)

// base names the embedded interface so the fake can define ColumnType().
type base = gorm.ColumnType

// column implements only the ColumnType method the mappings read.
type column struct {
	base // nil; other methods are unused
	full string
}

func (c column) ColumnType() (string, bool) { return c.full, c.full != "" }

func TestIntegerTypesWidenMySQLUnsigned(t *testing.T) {
	cases := []struct {
		dialect, dbType, full, want string
	}{
		{"mysql", "bigint", "bigint unsigned", "int64"},
		{"mysql", "bigint", "bigint", "int64"},
		{"mysql", "int", "int unsigned", "int64"},
		{"mysql", "int", "int(10) unsigned", "int64"},
		{"mysql", "int", "int", "int32"},
		{"mysql", "smallint", "smallint unsigned", "int32"},
		{"mysql", "smallint", "smallint", "int16"},
		{"mysql", "INT", "INT UNSIGNED", "int64"},
		// PostgreSQL and SQLite have no unsigned integers; their widths are unchanged.
		{"postgres", "int8", "int8", "int64"},
		{"postgres", "int4", "int4", "int32"},
		{"postgres", "int2", "int2", "int16"},
		{"sqlite", "integer", "integer", "int32"},
		{"sqlite", "int", "int unsigned", "int32"},
		{"sqlite", "smallint", "smallint unsigned", "int16"},
	}
	for _, c := range cases {
		mapping, ok := IntegerTypes(c.dialect)[c.dbType]
		if !ok {
			t.Fatalf("%s %s: no mapping", c.dialect, c.dbType)
		}
		if got := mapping(column{full: c.full}); got != c.want {
			t.Errorf("%s %s (%s) = %s, want %s", c.dialect, c.dbType, c.full, got, c.want)
		}
	}
}
