package modelcfg

import (
	"path/filepath"
	"strings"

	"gorm.io/gen"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

// NewGenerator shares model naming, nullable fields and integer widths with scaffolding.
func NewGenerator(db *gorm.DB, output string) *gen.Generator {
	g := gen.NewGenerator(gen.Config{OutPath: filepath.Join(output, "query"), ModelPkgPath: filepath.Join(output, "model"), FieldNullable: true, FieldWithDefaultTag: true})
	g.UseDB(db)

	g.WithDataTypeMap(IntegerTypes(db.Dialector.Name()))
	return g
}

// IntegerTypes maps database integer types to signed Go widths. The admin
// wire contract (Proto int32/int64, TypeScript number/bigint) is signed, so
// MySQL/MariaDB UNSIGNED columns widen to the next signed type that holds their
// whole range; BIGINT UNSIGNED stays int64 and supports values up to
// 9223372036854775807. Other dialects have no unsigned integer types and keep
// their existing mapping.
func IntegerTypes(dialect string) map[string]func(gorm.ColumnType) string {
	integerTypes := map[string]func(gorm.ColumnType) string{}
	unsignedWidths := map[string]string{"int": "int64", "smallint": "int32"}
	for dbType, goType := range map[string]string{"int8": "int64", "bigint": "int64", "int4": "int32", "integer": "int32", "int": "int32", "int2": "int16", "smallint": "int16"} {
		mapping := func(gorm.ColumnType) string { return goType }
		if wide, ok := unsignedWidths[dbType]; ok && dialect == "mysql" {
			mapping = func(column gorm.ColumnType) string {
				if Unsigned(column) {
					return wide
				}
				return goType
			}
		}
		integerTypes[dbType] = mapping
		integerTypes[strings.ToUpper(dbType)] = mapping
	}
	return integerTypes
}

// Unsigned reports a MySQL/MariaDB UNSIGNED integer column.
func Unsigned(column gorm.ColumnType) bool {
	full, ok := column.ColumnType()
	return ok && strings.Contains(strings.ToLower(full), "unsigned")
}

// TableOptions preserves database key semantics and application overrides.
func TableOptions(db *gorm.DB, driver, table string) ([]gen.ModelOpt, error) {
	columns, err := db.Migrator().ColumnTypes(table)
	if err != nil {
		return nil, err
	}
	opts := []gen.ModelOpt{gen.FieldIgnore("active_marker")}
	for _, column := range columns {
		if primary, _ := column.PrimaryKey(); primary {
			auto, _ := column.AutoIncrement()
			// SQLite INTEGER PRIMARY KEY is a 64-bit row identity. Its driver
			// omits auto-increment and reports the column as nullable.
			if driver == "sqlite" && strings.EqualFold(column.DatabaseTypeName(), "integer") {
				opts = append(opts, gen.FieldType(column.Name(), "int64"))
				auto = true
			}
			if auto {
				opts = append(opts, gen.FieldGORMTag(column.Name(), func(tag field.GormTag) field.GormTag {
					tag.Set("primaryKey")
					tag.Set("autoIncrement", "true")
					tag.Set("not null")
					return tag
				}))
			}
		}
		if value, ok := column.DefaultValue(); ok && (strings.EqualFold(value, "current_timestamp()") || strings.EqualFold(value, "now()")) {
			opts = append(opts, gen.FieldGORMTag(column.Name(), func(tag field.GormTag) field.GormTag { tag.Set("default", "CURRENT_TIMESTAMP"); return tag }))
		}
	}
	opts = append(opts, Overrides[table]...)
	return opts, nil
}
