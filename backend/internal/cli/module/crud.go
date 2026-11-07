package module

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/jinzhu/inflection"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/abysstract/abysstract-admin/backend/internal/codegen/modelcfg"
	"github.com/abysstract/abysstract-admin/backend/moduleseed"
)

// The database owns storage types; this input owns API exposure and field behavior.
type crudSpec struct {
	Name       string          `json:"name"`
	Table      string          `json:"table"`
	Title      string          `json:"title"`
	UpdatedAt  string          `json:"updatedAt"`
	Fields     []crudField     `json:"fields"`
	Operations []crudOperation `json:"operations,omitempty"`
}
type crudOperation struct {
	Method   string `json:"method"`
	Request  string `json:"request"`
	Response string `json:"response"`
	Action   string `json:"action"`
	Title    string `json:"title"`
	Proto    string `json:"proto"`
	Scope    string `json:"scope"`
}
type crudField struct {
	Column     string `json:"column"`
	Label      string `json:"label"`
	Required   bool   `json:"required,omitempty"`
	Searchable bool   `json:"searchable,omitempty"`
	Sortable   bool   `json:"sortable,omitempty"`
	MaxLength  int    `json:"maxLength,omitempty"`
}
type crudFieldView struct {
	Column, GoName, ProtoType  string
	Nullable                   bool
	Managed                    bool
	EntityNumber, CreateNumber int
	Options                    []string
}
type crudView struct {
	moduleView
	File, TSFile, Var, Name, Plural, Model, Title, AuditGo, AuditJSON string
	Timestamp                                                         bool
	Resources                                                         []crudView
	Reserved                                                          []retiredField
	Operations                                                        []crudOperation
	CustomImports                                                     []string
	CustomProtoMappings                                               []protoMapping
	Fields                                                            []crudFieldView
}

// moduleView holds the module paths from its declaration. Generated Go code
// imports the application's packages by these paths, and generated pages
// reach the protocol output relative to the module's frontend directory.
type moduleView struct {
	Module string
	// Backend and Frontend are the module's backend package and frontend
	// directories; Resource implementations are generated below them.
	Backend, Frontend string
	// DefinitionDir and ProtoDir hold the Resource declarations and the
	// protocol sources.
	DefinitionDir, ProtoDir string
	// Import is the backend package; the other imports name generated packages.
	Import, DefinitionImport, ProtoImport, ModelImport, QueryImport string
	// ProtoFromPages and ProtoFromResource reach the TypeScript protocol
	// output from the pages and generated Resource directories.
	ProtoFromPages, ProtoFromResource string
}

func newModuleView(layout appLayout, d descriptor) (moduleView, error) {
	if d.Generation == nil || d.Resource == nil {
		return moduleView{}, fmt.Errorf("module %s requires generation and resource configuration", d.Name)
	}
	g := d.Generation
	view := moduleView{
		Module: d.Name, Backend: path.Clean(d.Backend.Package), Frontend: path.Dir(d.Frontend.Entry),
		DefinitionDir: path.Clean(d.Resource.Package), ProtoDir: path.Clean(g.ProtoSource),
		Import: layout.importPath(d.Backend.Package), DefinitionImport: layout.importPath(d.Resource.Package),
		ProtoImport: layout.importPath(g.GoOutput) + "/" + strings.TrimSuffix(g.Prefix, "/"),
		ModelImport: layout.importPath(g.ModelOutput) + "/model", QueryImport: layout.importPath(g.ModelOutput) + "/query",
	}
	if !strings.HasPrefix(path.Clean(g.GoOutput), "backend/") || !strings.HasPrefix(path.Clean(g.ModelOutput), "backend/") {
		return moduleView{}, fmt.Errorf("module %s: generation goOutput and modelOutput must be under backend/", d.Name)
	}
	var err error
	if view.ProtoFromPages, err = relativeImport(view.Frontend+"/pages", g.TSOutput); err != nil {
		return moduleView{}, err
	}
	if view.ProtoFromResource, err = relativeImport(view.Frontend+"/gen/resource", g.TSOutput); err != nil {
		return moduleView{}, err
	}
	return view, nil
}

type protoMapping struct{ Source, Target string }

type retiredField struct {
	Name           string
	Entity, Create int
	Managed        bool
}

var columnName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)

func validateCRUDHeader(module string, spec crudSpec) error {
	if !exportedName.MatchString(spec.Name) || !columnName.MatchString(spec.Table) || !strings.HasPrefix(spec.Table, module+"_") || spec.Title == "" || !columnName.MatchString(spec.UpdatedAt) || spec.UpdatedAt == "id" {
		return fmt.Errorf("resource declaration requires an exported name, a %s_ table, title and updatedAt audit column", module)
	}
	return nil
}

func validateCRUD(module string, spec crudSpec) error {
	if err := validateCRUDHeader(module, spec); err != nil {
		return err
	}
	if len(spec.Fields) == 0 {
		return fmt.Errorf("resource declaration requires fields")
	}
	methods := map[string]bool{"List" + inflection.Plural(spec.Name): true, "Get" + spec.Name: true, "Create" + spec.Name: true, "Update" + spec.Name: true, "Delete" + spec.Name: true, "BatchDelete" + inflection.Plural(spec.Name): true}
	actions := map[string]bool{"list": true, "add": true, "edit": true, "remove": true}
	for _, op := range spec.Operations {
		if !exportedName.MatchString(op.Method) || !exportedName.MatchString(op.Request) || !exportedName.MatchString(op.Response) || !columnName.MatchString(op.Action) || op.Title == "" || op.Scope != "exempt" || methods[op.Method] || actions[op.Action] {
			return fmt.Errorf("custom operation requires unique method/action, request/response names, title and explicit scope=exempt")
		}
		if !strings.HasPrefix(op.Proto, "company/"+module+"/v1/") || !strings.HasSuffix(op.Proto, ".proto") || strings.Contains(op.Proto, "..") {
			return fmt.Errorf("custom operation %s: proto must be a module-local protocol path", op.Method)
		}
		methods[op.Method], actions[op.Action] = true, true
	}
	seen := map[string]bool{"id": true, spec.UpdatedAt: true}
	for _, f := range spec.Fields {
		if !columnName.MatchString(f.Column) || seen[f.Column] || f.Label == "" || f.MaxLength < 0 {
			return fmt.Errorf("invalid or duplicate field %q: use a snake_case column, label and nonnegative maxLength; id and updatedAt are server-managed", f.Column)
		}
		seen[f.Column] = true
	}
	return nil
}

// crudModel keeps the model generator's field selection and overrides for both commands.
type crudModel struct {
	ModelStructName string
	Fields          []crudModelField
	Columns         []gorm.ColumnType
}
type crudModelField struct{ Name, ColumnName, Type string }

func readCRUDModel(db *gorm.DB, driver, table, output string) (crudModel, error) {
	columns, err := db.Migrator().ColumnTypes(table)
	if err != nil {
		return crudModel{}, fmt.Errorf("read table %s: %w", table, err)
	}
	opts, err := modelcfg.TableOptions(db, driver, table)
	if err != nil {
		return crudModel{}, err
	}
	meta := modelcfg.NewGenerator(db, output).GenerateModel(table, opts...)
	result := crudModel{ModelStructName: meta.ModelStructName, Columns: columns}
	for _, f := range meta.Fields {
		result.Fields = append(result.Fields, crudModelField{Name: f.Name, ColumnName: f.ColumnName, Type: f.Type})
	}
	return result, nil
}

func inspectCRUD(db *gorm.DB, driver string, module moduleView, modelOutput string, spec crudSpec) (crudView, error) {
	meta, err := readCRUDModel(db, driver, spec.Table, modelOutput)
	if err != nil {
		return crudView{}, err
	}
	view, err := inspectCRUDModel(module.Module, spec, driver, meta)
	view.moduleView = module
	return view, err
}

func inspectCRUDModel(module string, spec crudSpec, driver string, meta crudModel) (crudView, error) {
	view := crudView{moduleView: moduleView{Module: module}, Name: spec.Name, Plural: inflection.Plural(spec.Name), Title: spec.Title}
	view.Operations = spec.Operations
	imports := map[string]bool{}
	for _, op := range spec.Operations {
		if !imports[op.Proto] {
			view.CustomImports = append(view.CustomImports, op.Proto)
			imports[op.Proto] = true
		}
	}
	view.File = schema.NamingStrategy{}.ColumnName("", spec.Name)
	var tsName strings.Builder
	for i, r := range spec.Name {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				tsName.WriteByte('-')
			}
			tsName.WriteRune(unicode.ToLower(r))
		} else {
			tsName.WriteRune(r)
		}
	}
	view.TSFile = tsName.String()
	view.Var = string(unicode.ToLower(rune(spec.Name[0]))) + spec.Name[1:]
	view.Model = meta.ModelStructName
	if err := validateCRUDModel(spec, driver, meta); err != nil {
		return view, err
	}
	for i, f := range spec.Fields {
		found := false
		for _, m := range meta.Fields {
			if m.ColumnName != f.Column {
				continue
			}
			found = true
			v, err := inspectCRUDField(f, m, i)
			if err != nil {
				return view, err
			}
			view.Timestamp = view.Timestamp || v.ProtoType == "google.protobuf.Timestamp"
			view.Fields = append(view.Fields, v)
		}
		if !found {
			return view, fmt.Errorf("column %s is absent from generated model %s", f.Column, view.Model)
		}
	}
	auditFound := false
	for _, m := range meta.Fields {
		if m.ColumnName != spec.UpdatedAt {
			continue
		}
		if m.Type != "time.Time" {
			return view, fmt.Errorf("audit column %s must be a nonnullable time.Time", spec.UpdatedAt)
		}
		auditFound = true
		view.AuditGo = m.Name
		parts := strings.Split(spec.UpdatedAt, "_")
		view.AuditJSON = parts[0]
		for _, part := range parts[1:] {
			view.AuditJSON += strings.ToUpper(part[:1]) + part[1:]
		}
		view.Timestamp = true
		view.Fields = append(view.Fields, crudFieldView{Column: spec.UpdatedAt, GoName: m.Name, ProtoType: "google.protobuf.Timestamp", Managed: true, EntityNumber: len(spec.Fields) + 2, Options: []string{"schema.ServerManaged()"}})
	}
	if !auditFound {
		return view, fmt.Errorf("audit column %s is absent from generated model", spec.UpdatedAt)
	}
	return view, nil
}

func renderCRUD(view crudView) (map[string][]byte, []moduleseed.Menu, error) {
	files := map[string][]byte{}
	targets := map[string]string{
		view.Backend + "/" + view.File + ".gen.go":                                "module.go.tmpl",
		view.DefinitionDir + "/" + view.File + ".gen.go":                          "resource.go.tmpl",
		view.ProtoDir + "/company/" + view.Module + "/v1/" + view.File + ".proto": "resource.proto.tmpl",
		view.Frontend + "/pages/" + view.Name + "List.tsx":                        "page.tsx.tmpl",
	}
	for path, t := range targets {
		data, err := renderScaffold(t, view)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", path, err)
		}
		files[path] = data
	}
	root := view.Module + ".root"
	prefix := view.Module + "." + view.File
	page := prefix + ".list"
	menus := []moduleseed.Menu{
		{Key: page, Parent: root, Name: view.Title + "列表", Path: view.File, Type: "C", Component: "biz/" + view.Module + "/" + view.File, Permission: "biz:" + view.Module + ":" + view.File + ":list"},
	}
	for i, action := range []struct{ key, title string }{{"list", "查询"}, {"add", "新增"}, {"edit", "修改"}, {"remove", "删除"}} {
		menus = append(menus, moduleseed.Menu{Key: prefix + "." + action.key + ".button", Parent: page, Name: view.Title + action.title, Type: "F", Permission: "biz:" + view.Module + ":" + view.File + ":" + action.key, Order: i + 1})
	}
	for _, op := range view.Operations {
		menus = append(menus, moduleseed.Menu{Key: prefix + "." + op.Action + ".button", Parent: page, Name: op.Title, Type: "F", Permission: "biz:" + view.Module + ":" + view.File + ":" + op.Action})
	}
	return files, menus, nil
}

func validateCRUDModel(spec crudSpec, driver string, meta crudModel) error {
	primaryCount := 0
	selected := map[string]bool{"id": true, spec.UpdatedAt: true}
	for _, f := range spec.Fields {
		selected[f.Column] = true
	}
	for _, c := range meta.Columns {
		if pk, _ := c.PrimaryKey(); pk {
			primaryCount++
			auto, _ := c.AutoIncrement()
			sqliteIdentity := driver == "sqlite" && strings.EqualFold(c.DatabaseTypeName(), "integer")
			if c.Name() != "id" || (!auto && !sqliteIdentity) {
				return fmt.Errorf("%s: CRUD requires an auto-increment id primary key (BIGINT, BIGSERIAL, MySQL/MariaDB BIGINT UNSIGNED or INT UNSIGNED, SQLite INTEGER)", spec.Table)
			}
		}
		nullable, _ := c.Nullable()
		_, hasDefault := c.DefaultValue()
		if !selected[c.Name()] && !nullable && !hasDefault {
			return fmt.Errorf("column %s requires a value: expose it in resource declaration or define its database default", c.Name())
		}
	}
	if primaryCount != 1 {
		return fmt.Errorf("%s: CRUD requires exactly one auto-increment id primary key", spec.Table)
	}
	idValid := false
	for _, f := range meta.Fields {
		if f.ColumnName == "id" && f.Name == "ID" && f.Type == "int64" {
			idValid = true
		}
	}
	if !idValid {
		return fmt.Errorf("%s.id must generate an int64 ID model field; use a 64-bit integer column or an UNSIGNED MySQL/MariaDB integer", spec.Table)
	}

	return nil
}

func inspectCRUDField(f crudField, m crudModelField, i int) (crudFieldView, error) {
	protoTypes := map[string]string{"string": "string", "bool": "bool", "int8": "int32", "int16": "int32", "int32": "int32", "int": "int64", "int64": "int64", "uint8": "uint32", "uint16": "uint32", "uint32": "uint32", "uint": "uint64", "uint64": "uint64", "float32": "float", "float64": "double", "time.Time": "google.protobuf.Timestamp"}
	typ := strings.TrimPrefix(m.Type, "*")
	switch typ {
	case "string", "bool", "int16", "int32", "int64", "time.Time":
	default:
		return crudFieldView{}, fmt.Errorf("column %s: Resource v1 supports string, bool, int16, int32, int64 and time.Time; define a dedicated contract for %s", f.Column, m.Type)
	}
	if strings.HasPrefix(m.Type, "*") && (typ == "time.Time" || typ == "int16") {
		return crudFieldView{}, fmt.Errorf("column %s: nullable %s requires a dedicated Resource conversion", f.Column, typ)
	}

	pt, ok := protoTypes[typ]
	if !ok {
		return crudFieldView{}, fmt.Errorf("column %s: unsupported CRUD type %s; define its Proto/Resource explicitly", f.Column, m.Type)
	}
	if (f.Required || f.Searchable || f.MaxLength > 0) && typ != "string" {
		return crudFieldView{}, fmt.Errorf("column %s: required, searchable and maxLength require string storage", f.Column)
	}
	v := crudFieldView{Column: f.Column, GoName: m.Name, ProtoType: pt, Nullable: strings.HasPrefix(m.Type, "*"), EntityNumber: i + 2, CreateNumber: i + 1, Options: []string{"schema.Label(" + strconv.Quote(f.Label) + ")"}}
	if f.Required {
		v.Options = append(v.Options, "schema.Required()")
	}
	if f.Searchable {
		v.Options = append(v.Options, "schema.Searchable()")
	}
	if f.Sortable {
		v.Options = append(v.Options, "schema.SortableOpt()")
	}
	if f.MaxLength > 0 {
		v.Options = append(v.Options, fmt.Sprintf("schema.MaxLen(%d)", f.MaxLength))
	}
	return v, nil
}
