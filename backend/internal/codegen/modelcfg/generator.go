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

	integerTypes := map[string]func(gorm.ColumnType) string{}
	for dbType, goType := range map[string]string{"int8": "int64", "bigint": "int64", "int4": "int32", "integer": "int32", "int": "int32", "int2": "int16", "smallint": "int16"} {
		mapping := func(gorm.ColumnType) string { return goType }
		integerTypes[dbType] = mapping
		integerTypes[strings.ToUpper(dbType)] = mapping
	}
	g.WithDataTypeMap(integerTypes)
	return g
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
