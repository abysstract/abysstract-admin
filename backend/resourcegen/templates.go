package resourcegen

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"text/template"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

// conversionView 单方向转换视图（模板只读，不再二次判断）。
type conversionView struct {
	Expr        string // 赋值/比较表达式（源变量已内联）
	Nullable    bool
	IsEnum      bool   // 该方向走 enum converter
	NeedsBounds bool   // 该方向需溢出检查
	NarrowCheck string // 溢出检查 if 条件
}

// goField 预计算的模板字段视图（模板内不做函数调用，全部在 Go 侧算好）。
type goField struct {
	Identifier     bool
	ProtoGoName    string
	ModelField     string
	ProtoName      string
	DBName         string
	ColumnConst    string
	InCreate       bool
	CreateOptional bool
	ModelNullable  bool
	InUpdate       bool
	Presence       bool // UpdateRequest 中具备 protobuf presence

	// 三个方向独立的转换视图（模板按方向各自的 Plan 输出，无字段级共享标志）。
	ToEntity conversionView
	Create   conversionView
	Update   conversionView

	// CreateOnly 字段出现在 UpdateRequest：生成 presence 变更检查。
	CreateOnlyInUpdate bool

	// enum 辅助函数（仅当任一方向为 enum 时生成；字段级共享函数名）。
	HasEnum       bool
	EnumTypeName  string
	EnumModelType string // enum converter 的 model 返回类型（int16/int32/int64）
	EnumToModel   string
	EnumFromModel string
	EnumEntries   []enumEntry

	IsTime bool // ToEntity 方向为 time（决定 import 与 CheckValid）

	// 校验元数据（从 Schema Required/MaxLen 生成后端校验）。
	ValRequired bool
	ValMaxLen   int
	IsString    bool // model 字段是 string（maxLen 适用）

	Label         string
	TSName        string
	TSType        string
	Required      bool
	Searchable    bool
	Filterable    bool
	Sortable      bool
	Listable      bool
	MaxLen        int
	ServerManaged bool
	Updatable     bool
	HasRelation   bool
	RelTarget     string
	RelValueField string
	RelLabelField string
}

type goPlan struct {
	Name           string
	PermPrefix     string
	Table          string
	TSVar          string
	ModelType      string
	EntityType     string
	CreateType     string
	UpdateType     string
	UsesTime       bool
	UsesMath       bool
	UsesStrings    bool
	UsesIdentity   bool
	IdentityImport string
	Fields         []goField
	List           *listView // 标准分页 List IR；无 ListBinding 的资源为 nil

	// Controller 骨架（需求 8.1/8.5）。
	ModelRef           string // model 类型引用（由 Plan 包投影决定）
	ModelPackage       string
	ModelImportPath    string
	GenQueryImport     string
	KeyModelField      string
	ProtoPackage       string
	ProtoImportPath    string
	RepositoryInjected bool
	ReadRepository     bool // 仅声明 generated read operation 时使用最小只读 Repository contract
	Partial            bool // Mode=Partial：Deps 暴露 Overrides
	DataScope          bool // Auth.DataScope=true：Deps 暴露 DataScopeProvider
	CreateScopeCheck   bool // DataScope 且未 skip：Deps 暴露 CreateScopeValidator
	Operations         []operationView
	Permissions        []permissionView
	TSPermissions      tsPermissionView

	// Frontend operations follow protobuf/client -> raw -> UI adapter.
	TSServiceModule    string
	TSServiceName      string
	TSFileBase         string
	TSRawOperations    []tsOperationView
	TSCustomOperations []tsOperationView
	TSUpdate           *tsUpdateView
	TSFacade           *tsFacadeView
	TSReadFacade       *tsReadFacadeView
	TSRelations        []tsRelationView

	// Update 审计与列策略（需求 10.10/10.11）。
	KeyColumnConst        string
	KeyDBName             string
	UpdatedAtModelField   string
	UpdatedAtColumnConst  string
	UpdatedAtDBName       string
	DeptDBName            string
	CreatorDBName         string
	UpdatableColumnConsts []string // EffUpdatable 字段的列常量名

	GoPackage          string
	Header             string
	ResourceImport     string
	PermissionImport   string
	TSSchemaImport     string
	TSOperationsImport string
	TSRelationImport   string
	PageInfoRef        string
	PageInfoPackage    string
	PageInfoImportPath string
}

// listFilterView 单个 typed filter 的模板视图。
type listFilterView struct {
	ModelField     string
	FieldName      string // ListQuery Go 字段名（Request 字段 Go 名）
	ColumnConst    string
	DBName         string
	DeclaredInConv bool
	GoType         string // model 侧 Go 类型（string/bool/int16/...）
	ValueExpr      string // 由 *T request 值构造 model 值的表达式（v 为解引用变量）
	NeedsEnum      bool   // 值表达式需要返回 error 的 enum converter
	NeedsBounds    bool
}

// listColumnView 一个参与 List 语义的 DB 列（search/sort/filter 共用形态）。
type listColumnView struct {
	ColumnConst    string
	DBName         string
	JSONName       string // 仅 sort 使用（OrderBy.field wire 名）
	DeclaredInConv bool   // 同名常量已由 *_conv.gen.go 的 update 列常量声明
}

type listDefaultOrderView struct {
	ColumnConst string
	Desc        bool
}

// listView 标准 List 的生成视图。
type listView struct {
	UsesIdentifier bool
	QueryType      string // <Name>ListQuery
	AdapterType    string // <Name>Repository
	KeyColumnConst string
	SearchConst    string // []resource.Column 变量名（空则禁用 keyword）
	SearchFields   []listColumnView
	SortConst      string // map[string]resource.Column 白名单变量名
	SortFields     []listColumnView
	Filters        []listFilterView
	DefaultOrders  []listDefaultOrderView
	PageField      string
	KeywordField   string
	SortField      string
	ItemsField     string
	PageInfoField  string
}

// operationView 一个标准 RPC 的 Controller 方法视图。
type operationView struct {
	Method          string // RPC Go 方法名
	RequestRef      string
	ResponseRef     string
	ResponseElemRef string
	ItemField       string
	IdentifierField string
	IsCreate        bool
	IsUpdate        bool
	IsDelete        bool
	IsList          bool
	IsGet           bool
	IsBatchDelete   bool
	IsCustom        bool
	BatchIDsField   string
}

type permissionView struct {
	Procedure  string
	Permission string
}

type tsPermissionView struct {
	List        string
	Get         string
	Create      string
	Update      string
	Delete      string
	BatchDelete string
}

// tsOperationView is a direct, unmodified RPC client call.
type tsOperationView struct {
	Method string
}

// tsUpdateField is one ordinary UI-updatable field. Lifecycle exclusions are
// resolved before rendering so templates never infer permissions from names.
type tsUpdateField struct {
	Name string
	Type string
}

// tsUpdateView binds the standard Update request to a presence-aware UI adapter.
type tsUpdateView struct {
	Method          string
	IdentifierField string
	IdentifierType  string
	Fields          []tsUpdateField
}

// tsRegistryEntry exposes one target Resource's standard List raw operation.
type tsRegistryEntry struct {
	Name       string
	TSVar      string
	TSFileBase string
	ListMethod string
}

// tsRegistryView is the template data for the static operations registry: the
// raw List registry (consumed by relation loaders) plus full facade entries.
type tsRegistryView struct {
	Header  string
	Entries []tsRegistryEntry
	Facades []tsRegistryEntry
}

// tsFacadeFilter is one presence-aware List filter mapped onto the raw request.
type tsFacadeFilter struct {
	RequestJSONName string
}

// tsFacadeView is the unified typed operations facade IR (需求 12.17). It binds
// the six standard operations to the frozen ResourceOperations contract and is
// derived only from validated schema Plan data; templates never re-derive RPC
// roles or field types.
type tsFacadeView struct {
	EntityModule      string // protobuf-ES module that owns entity/create/list types
	EntityType        string
	CreateType        string
	ListRequestType   string // only needed when filters exist (Pick source)
	KeyType           string // int64 stays bigint, never narrowed to number
	KeyJSONName       string
	FilterType        string
	OrderFieldType    string // schema-derived SortableFieldName<typeof <name>Schema>
	UpdateValuesName  string
	UpdateTouchedName string

	ListMethod        string
	CreateMethod      string
	GetMethod         string
	UpdateMethod      string
	DeleteMethod      string
	BatchDeleteMethod string

	CreateItemField string
	GetItemField    string
	UpdateItemField string

	IdentifierField string
	BatchIDsField   string
	PageField       string
	KeywordField    string
	SortField       string
	ItemsField      string
	PageInfoField   string

	Filters         []tsFacadeFilter
	HasFilters      bool
	HasSort         bool
	HasKeyword      bool
	KeywordOptional bool
}

// tsReadFacadeView is the intentionally smaller frontend facade for a
// Partial Resource. It exposes the standard List contract while preserving
// CUSTOM RPCs under a typed custom namespace; it must never satisfy the full
// ResourceOperations CRUD contract by inventing absent standard mutations.
type tsReadFacadeView struct {
	EntityModule    string
	EntityType      string
	KeyType         string
	KeyJSONName     string
	ListRequestType string
	FilterType      string
	OrderFieldType  string

	ListMethod      string
	CreateMethod    string
	CreateFields    string
	PageField       string
	KeywordField    string
	SortField       string
	ItemsField      string
	PageInfoField   string
	Filters         []tsFacadeFilter
	HasFilters      bool
	HasSort         bool
	HasKeyword      bool
	KeywordOptional bool
	Update          *tsUpdateView
}

// tsRelationView is a fully-resolved cross-resource frontend relation plan.
type tsRelationView struct {
	FieldName           string
	TargetResource      string
	TargetTSVar         string
	TargetTSFileBase    string
	TargetListMethod    string
	PageField           string
	KeywordField        string
	ItemsField          string
	PageInfoField       string
	ValueField          string
	ValueType           string
	LabelField          string
	RemoteSearchEnabled bool
}

type enumEntry struct {
	Name     string
	ConstRef string
	Value    int16
}

func buildView(p *schema.Plan, cfg Config) *goPlan {
	v := &goPlan{
		Name:               p.Res.Name,
		PermPrefix:         p.Res.PermPrefix,
		Table:              p.Res.Table,
		TSVar:              lowerFirst(p.Res.Name),
		TSFileBase:         strings.ReplaceAll(snake(p.Res.Name), "_", "-"),
		ModelType:          p.ModelType.Name(),
		EntityType:         p.EntityType.Name(),
		CreateType:         p.CreateType.Name(),
		UpdateType:         p.UpdateType.Name(),
		GoPackage:          cfg.GoPackage,
		Header:             fileHeaderFrom(cfg),
		ResourceImport:     cfg.GoImports.Resource,
		GenQueryImport:     cfg.GenQueryImport,
		PermissionImport:   cfg.GoImports.Permission,
		TSSchemaImport:     cfg.TSImports.Schema,
		TSOperationsImport: cfg.TSImports.Operations,
		TSRelationImport:   cfg.TSImports.Relation,
	}
	if service := p.Res.Service.Descriptor; service != nil {
		if module, err := tsModuleForProto(cfg, protoFileOfService(service)); err == nil {
			v.TSServiceModule = module
		}
		v.TSServiceName = string(service.Name())
	}
	v.KeyColumnConst = p.Res.Name + "Column" + protoGoNameOf(p, p.Key.ModelField)
	v.KeyDBName = p.Key.DBName
	v.KeyModelField = p.Key.ModelField
	v.UpdatedAtModelField = p.UpdatedAtModelField
	v.UpdatedAtDBName = p.UpdatedAtDBName
	v.DeptDBName, v.CreatorDBName = p.DeptDBName, p.CreatorDBName
	if f := p.Field(p.UpdatedAtModelField); f != nil {
		v.UpdatedAtColumnConst = p.Res.Name + "Column" + f.ProtoGoName
	}
	v.List = buildListView(p)
	v.RepositoryInjected = p.Res.Repository.Implementation == schema.RepositoryInjected
	v.ReadRepository = hasGeneratedReadController(p)
	v.Partial = p.Res.Mode == schema.Partial
	v.DataScope = p.Res.Auth.DataScope
	v.CreateScopeCheck = p.Res.Auth.DataScope && !p.Res.Auth.SkipCreateScopeCheck && p.Operation(schema.OperationCreate) != nil
	goTypes := []reflect.Type{p.ModelType, p.EntityType, p.CreateType, p.UpdateType}
	for _, op := range p.Operations {
		goTypes = append(goTypes, op.RequestType, op.ResponseType)
	}
	if listOp := p.Operation(schema.OperationList); listOp != nil && listOp.ResponseType != nil && listOp.List != nil {
		resp := listOp.ResponseType
		for resp != nil && resp.Kind() == reflect.Pointer {
			resp = resp.Elem()
		}
		if resp != nil {
			if field, ok := resp.FieldByName(listOp.List.PageInfoField); ok {
				goTypes = append(goTypes, field.Type)
			}
		}
	}
	aliases := collectGoPackages(goTypes...)
	if listOp := p.Operation(schema.OperationList); listOp != nil && listOp.ResponseType != nil && listOp.List != nil {
		resp := listOp.ResponseType
		for resp != nil && resp.Kind() == reflect.Pointer {
			resp = resp.Elem()
		}
		if resp != nil {
			if field, ok := resp.FieldByName(listOp.List.PageInfoField); ok {
				v.PageInfoRef = aliases.typeRef(field.Type)
				pageType := derefType(field.Type)
				v.PageInfoPackage, v.PageInfoImportPath = typePackage(pageType)
				if alias := aliases.alias(pageType.PkgPath()); alias != "" {
					v.PageInfoPackage = alias
				}
			}
		}
	}
	v.ModelPackage, v.ModelImportPath = typePackage(derefType(p.ModelType))
	if alias := aliases.alias(derefType(p.ModelType).PkgPath()); alias != "" {
		v.ModelPackage = alias
	}
	v.ProtoPackage, v.ProtoImportPath = typePackage(derefType(p.EntityType))
	if alias := aliases.alias(derefType(p.EntityType).PkgPath()); alias != "" {
		v.ProtoPackage = alias
	}
	if v.ModelPackage != "" {
		v.ModelRef = v.ModelPackage + "." + derefType(p.ModelType).Name()
	} else {
		v.ModelRef = derefType(p.ModelType).Name()
	}
	for _, op := range p.Operations {
		responseRef := aliases.typeRef(op.ResponseType)
		v.TSRawOperations = append(v.TSRawOperations, tsOperationView{Method: lowerFirst(op.Method)})
		if op.Kind == schema.OperationCustom {
			v.TSCustomOperations = append(v.TSCustomOperations, tsOperationView{Method: lowerFirst(op.Method)})
		}
		v.Operations = append(v.Operations, operationView{
			Method:          op.Method,
			RequestRef:      aliases.typeRef(op.RequestType),
			ResponseRef:     responseRef,
			ResponseElemRef: responseRef,
			ItemField:       op.Binding.Item,
			IdentifierField: op.Binding.Identifier,
			IsCreate:        op.Kind == schema.OperationCreate,
			IsUpdate:        op.Kind == schema.OperationUpdate,
			IsDelete:        op.Kind == schema.OperationDelete,
			IsList:          op.Kind == schema.OperationList,
			IsGet:           op.Kind == schema.OperationGet,
			IsBatchDelete:   op.Kind == schema.OperationBatchDelete,
			IsCustom:        op.Kind == schema.OperationCustom,
			BatchIDsField:   op.Binding.BatchIDs,
		})
		if permission := operationPermission(p.Res.PermPrefix, op.Operation); permission != "" {
			v.Permissions = append(v.Permissions, permissionView{
				Procedure:  "/" + string(p.Res.Service.Descriptor.FullName()) + "/" + op.Method,
				Permission: permission,
			})
			assignTSPermission(&v.TSPermissions, p.Res.Name, op.Operation, permission)
		}
	}
	for _, f := range p.Fields {
		gf := goField{
			Identifier:     f.Decl.Identifier,
			ProtoGoName:    f.ProtoGoName,
			ModelField:     f.ModelField,
			ProtoName:      f.ProtoName,
			DBName:         f.DBName,
			ColumnConst:    p.Res.Name + "Column" + f.ProtoGoName,
			InCreate:       f.InCreate,
			CreateOptional: f.CreateOptional && f.ModelType.Kind() == reflect.Pointer,
			ModelNullable:  f.ModelType.Kind() == reflect.Pointer,
			InUpdate:       f.InUpdate,
			Label:          f.Decl.Label,
			TSName:         lowerFirst(f.ProtoGoName),
			TSType:         tsTypeOf(f),
			ValRequired:    f.Decl.Required,

			ValMaxLen:     f.Decl.MaxLen,
			IsString:      derefType(f.ModelType).Kind() == reflect.String,
			Required:      f.Decl.Required,
			Searchable:    f.Decl.Searchable,
			Filterable:    f.Decl.Filterable,
			Sortable:      f.Decl.Sortable,
			Listable:      f.Decl.Listable,
			MaxLen:        f.Decl.MaxLen,
			ServerManaged: f.Decl.Lifecycle.ServerManaged,
			Updatable:     f.Decl.Lifecycle.EffUpdatable() && !f.Decl.ReadOnlyFlag,
		}
		gf.CreateOnlyInUpdate = f.CreateOnlyInUpdate
		if f.Decl.Required && derefType(f.ModelType).Kind() == reflect.String {
			v.UsesStrings = true
		}
		if f.Decl.Relation != nil {
			gf.HasRelation = true
			gf.RelTarget = f.Decl.Relation.Target().Name
			// 使用 Build 解析的目标 entity 前端 JSON 名（camelCase）。
			gf.RelValueField = f.Decl.Relation.RefJSONName
			gf.RelLabelField = f.Decl.Relation.LabelJSONName
		}
		src := "m." + f.ModelField
		getter := "req.Get" + f.ProtoGoName + "()"
		if f.Decl.Identifier {
			v.UsesIdentity = true
			v.IdentityImport = strings.TrimSuffix(v.ResourceImport, "/resource") + "/identity"
			getter = "identity.NormalizeIdentifier(" + getter + ")"
		}

		// 三个方向各自按 ConversionPlan 生成视图。
		gf.ToEntity = toEntityView(f.ModelToEntity, src, &gf, v)
		gf.Create = toModelView(f.CreateToModel, getter, &gf, v)
		gf.Update = toModelView(f.UpdateToModel, getter, &gf, v)

		// enum 辅助函数名（任一方向为 enum 即生成；函数名按资源+字段统一），
		// 并替换视图表达式中的占位符为实际函数名。
		if gf.HasEnum {
			base := lowerFirst(p.Res.Name) + f.ProtoGoName
			gf.EnumToModel = base + "ToModel"
			gf.EnumFromModel = base + "FromModel"
			sub := func(e string) string {
				e = strings.ReplaceAll(e, "ENUM_TO_MODEL", gf.EnumToModel)
				e = strings.ReplaceAll(e, "ENUM_FROM_MODEL", gf.EnumFromModel)
				return e
			}
			gf.ToEntity.Expr = sub(gf.ToEntity.Expr)
			gf.Create.Expr = sub(gf.Create.Expr)
			gf.Update.Expr = sub(gf.Update.Expr)
			for k, val := range f.Decl.EnumMap {
				gf.EnumEntries = append(gf.EnumEntries, enumEntry{k, gf.EnumTypeName + "_" + k, val})
			}
			sort.Slice(gf.EnumEntries, func(i, j int) bool { return gf.EnumEntries[i].Name < gf.EnumEntries[j].Name })
		}
		if f.InUpdate && f.Decl.Lifecycle.EffUpdatable() && !f.Decl.ReadOnlyFlag {
			v.UpdatableColumnConsts = append(v.UpdatableColumnConsts, gf.ColumnConst)
		}
		v.Fields = append(v.Fields, gf)
	}
	for _, op := range p.Operations {
		if op.Kind != schema.OperationUpdate && !isPresenceAwareCustomUpdate(p, op) {
			continue
		}
		update := &tsUpdateView{
			Method:          lowerFirst(op.Method),
			IdentifierField: lowerFirst(op.Binding.Identifier),
			IdentifierType:  p.Key.TSType,
		}
		for _, field := range v.Fields {
			if field.InUpdate && field.Updatable {
				update.Fields = append(update.Fields, tsUpdateField{Name: field.TSName, Type: field.TSType})
			}
		}
		v.TSUpdate = update
		break
	}
	v.TSFacade = buildTSFacadeView(p, v)
	v.TSReadFacade = buildTSReadFacadeView(p, v)
	return v
}

// isPresenceAwareCustomUpdate identifies a deliberately retained custom
// profile-update workflow. The schema still owns the operation and its
// request, while the generated builder owns optional-field presence. This is
// intentionally limited to the resource's conventional Update<Name> method;
// unrelated CUSTOM actions (status/password/relations) never enter this path.
func isPresenceAwareCustomUpdate(p *schema.Plan, op *schema.OperationPlan) bool {
	return p.Res.Mode == schema.Partial && op != nil &&
		op.Kind == schema.OperationCustom && op.Method == "Update"+p.Res.Name
}

// buildTSFacadeView derives the unified typed operations facade IR from the
// validated six standard operations (需求 12.17). It returns nil when the
// Resource lacks a complete standard-six set or a standard paginated List.
func buildTSFacadeView(p *schema.Plan, v *goPlan) *tsFacadeView {
	list := p.Operation(schema.OperationList)
	create := p.Operation(schema.OperationCreate)
	get := p.Operation(schema.OperationGet)
	update := p.Operation(schema.OperationUpdate)
	del := p.Operation(schema.OperationDelete)
	batch := p.Operation(schema.OperationBatchDelete)
	if list == nil || list.List == nil || create == nil || get == nil || update == nil || del == nil || batch == nil {
		return nil
	}
	if v.TSServiceModule == "" {
		return nil
	}
	facade := &tsFacadeView{
		EntityModule:      v.TSServiceModule,
		EntityType:        p.EntityType.Name(),
		CreateType:        p.CreateType.Name(),
		KeyType:           p.Key.TSType,
		KeyJSONName:       p.Key.JSONName,
		OrderFieldType:    fmt.Sprintf("SortableFieldName<typeof %sSchema>", v.TSVar),
		UpdateValuesName:  p.Res.Name + "UpdateValues",
		UpdateTouchedName: p.Res.Name + "UpdateTouched",
		ListMethod:        lowerFirst(list.Method),
		CreateMethod:      lowerFirst(create.Method),
		GetMethod:         lowerFirst(get.Method),
		UpdateMethod:      lowerFirst(update.Method),
		DeleteMethod:      lowerFirst(del.Method),
		BatchDeleteMethod: lowerFirst(batch.Method),
		CreateItemField:   lowerFirst(create.Binding.Item),
		GetItemField:      lowerFirst(get.Binding.Item),
		UpdateItemField:   lowerFirst(update.Binding.Item),
		IdentifierField:   lowerFirst(update.Binding.Identifier),
		BatchIDsField:     lowerFirst(batch.Binding.BatchIDs),
		PageField:         lowerFirst(list.List.PageRequestField),
		KeywordField:      lowerFirst(list.List.KeywordField),
		SortField:         lowerFirst(list.List.SortField),
		ItemsField:        lowerFirst(list.List.ItemsField),
		PageInfoField:     lowerFirst(list.List.PageInfoField),
	}
	if list.List.KeywordField != "" {
		facade.HasKeyword = true
		facade.KeywordOptional = listKeywordFieldOptional(list)
	}
	if list.List.SortField != "" {
		facade.HasSort = true
	}
	if len(list.List.Filters) > 0 {
		facade.HasFilters = true
		facade.ListRequestType = list.RequestType.Elem().Name()
		names := make([]string, 0, len(list.List.Filters))
		for _, filter := range list.List.Filters {
			facade.Filters = append(facade.Filters, tsFacadeFilter{RequestJSONName: filter.RequestJSONName})
			names = append(names, fmt.Sprintf("%q", filter.RequestJSONName))
		}
		facade.FilterType = fmt.Sprintf("Pick<%s, %s>", facade.ListRequestType, strings.Join(names, " | "))
	} else {
		facade.FilterType = "Record<never, never>"
	}
	return facade
}

// buildTSReadFacadeView derives the partial read facade from the validated
// standard List operation. It is independent from the full six-operation
// facade so a Partial Resource cannot accidentally acquire fake CRUD methods.
func buildTSReadFacadeView(p *schema.Plan, v *goPlan) *tsReadFacadeView {
	list := p.Operation(schema.OperationList)
	if p.Res.Mode != schema.Partial || list == nil || list.List == nil || v.TSServiceModule == "" {
		return nil
	}
	facade := &tsReadFacadeView{
		EntityModule:   v.TSServiceModule,
		EntityType:     p.EntityType.Name(),
		KeyType:        p.Key.TSType,
		KeyJSONName:    p.Key.JSONName,
		OrderFieldType: fmt.Sprintf("SortableFieldName<typeof %sSchema>", v.TSVar),
		ListMethod:     lowerFirst(list.Method),
		PageField:      lowerFirst(list.List.PageRequestField),
		KeywordField:   lowerFirst(list.List.KeywordField),
		SortField:      lowerFirst(list.List.SortField),
		ItemsField:     lowerFirst(list.List.ItemsField),
		PageInfoField:  lowerFirst(list.List.PageInfoField),
		FilterType:     "Record<never, never>",
	}
	if list.List.KeywordField != "" {
		facade.HasKeyword = true
		facade.KeywordOptional = listKeywordFieldOptional(list)
	}
	if list.List.SortField != "" {
		facade.HasSort = true
	}
	if len(list.List.Filters) > 0 {
		facade.HasFilters = true
		facade.ListRequestType = list.RequestType.Elem().Name()
		names := make([]string, 0, len(list.List.Filters))
		for _, filter := range list.List.Filters {
			facade.Filters = append(facade.Filters, tsFacadeFilter{RequestJSONName: filter.RequestJSONName})
			names = append(names, fmt.Sprintf("%q", filter.RequestJSONName))
		}
		facade.FilterType = fmt.Sprintf("Pick<%s, %s>", facade.ListRequestType, strings.Join(names, " | "))
	}
	for _, op := range p.Operations {
		if op.Kind == schema.OperationCustom && op.Method == "Create"+p.Res.Name {
			facade.CreateMethod = lowerFirst(op.Method)
		}
		if !isPresenceAwareCustomUpdate(p, op) {
			continue
		}
		update := &tsUpdateView{
			Method:          lowerFirst(op.Method),
			IdentifierField: lowerFirst(op.Binding.Identifier),
			IdentifierType:  p.Key.TSType,
		}
		if update.IdentifierField == "" {
			// The custom User declaration intentionally follows the standard
			// identifier convention; keep this guard for malformed/legacy plans
			// instead of generating a request with an empty property name.
			update.IdentifierField = lowerFirst(p.Key.ProtoName)
		}
		for _, field := range v.Fields {
			if field.InUpdate && field.Updatable {
				update.Fields = append(update.Fields, tsUpdateField{Name: field.TSName, Type: field.TSType})
			}
		}
		facade.Update = update
		break
	}
	createNames := make([]string, 0)
	for _, field := range v.Fields {
		if field.InCreate {
			createNames = append(createNames, fmt.Sprintf("%q", field.TSName))
		}
	}
	facade.CreateFields = strings.Join(createNames, " | ")
	return facade
}

// listKeywordFieldOptional reports whether the standard List request's keyword
// field has protobuf presence (optional string, 需求 12.17 列表 presence)。
// Required keyword is normalized to "" in the facade (wire type demands the
// field); optional keyword is forwarded as-is so "unset" stays unset, matching
// the filters' !== undefined presence handling in the same template.
func listKeywordFieldOptional(list *schema.OperationPlan) bool {
	if list == nil || list.List == nil || list.List.KeywordField == "" {
		return false
	}
	_, fields, err := schema.InspectStruct(list.Request)
	if err != nil {
		return false
	}
	for _, field := range fields {
		if field.Name == list.List.KeywordField {
			return field.Optional
		}
	}
	return false
}

// operationPermission derives the fixed v1 mapping for standard CRUD
// operations and CUSTOM permissions from their validated action. The only
// compatibility exception is the closed LegacyPermission plan validated by
// Schema.Build (for example GetRole -> system:role:query).
func operationPermission(prefix string, operation schema.Operation) string {
	if operation.LegacyPermission == schema.LegacyPermissionGetQuery {
		return prefix + ":query"
	}
	switch operation.Kind {
	case schema.OperationList, schema.OperationGet:
		return prefix + ":list"
	case schema.OperationCreate:
		return prefix + ":add"
	case schema.OperationUpdate:
		return prefix + ":edit"
	case schema.OperationDelete, schema.OperationBatchDelete:
		return prefix + ":remove"
	case schema.OperationCustom:
		return prefix + ":" + operation.Action
	default:
		return ""
	}
}

func assignTSPermission(target *tsPermissionView, resourceName string, operation schema.Operation, permission string) {
	switch operation.Kind {
	case schema.OperationList:
		target.List = permission
	case schema.OperationGet:
		target.Get = permission
	case schema.OperationCreate:
		target.Create = permission
	case schema.OperationUpdate:
		target.Update = permission
	case schema.OperationDelete:
		target.Delete = permission
	case schema.OperationBatchDelete:
		target.BatchDelete = permission
	case schema.OperationCustom:
		switch operation.Method {
		case "Get" + resourceName:
			target.Get = permission
		case "Create" + resourceName:
			target.Create = permission
		case "Update" + resourceName:
			target.Update = permission
		case "Delete" + resourceName:
			target.Delete = permission
		case "BatchDelete" + resourceName + "s":
			target.BatchDelete = permission
		}
	}
}

// toEntityView 按 ModelToEntity ConversionPlan 生成 model→entity 视图。
func toEntityView(cp schema.ConversionPlan, src string, gf *goField, v *goPlan) conversionView {
	if cp.SourceType != nil && cp.SourceType.Kind() == reflect.Pointer && cp.SourceType.Elem() == cp.TargetType {
		return conversionView{Expr: "*" + src, Nullable: true}
	}
	switch cp.Kind {
	case schema.ConvTime:
		v.UsesTime = true
		gf.IsTime = true
		return conversionView{Expr: "timestamppb.New(" + src + ")"}
	case schema.ConvEnum:
		markEnum(gf, cp)
		return conversionView{Expr: "ENUM_FROM_MODEL(" + src + ")", IsEnum: true}
	case schema.ConvNarrow:
		return conversionView{Expr: "int32(" + src + ")"}
	default:
		return conversionView{Expr: src}
	}
}

// toModelView 按 Create/Update ConversionPlan 生成 request→model 视图。
func toModelView(cp schema.ConversionPlan, getter string, gf *goField, v *goPlan) conversionView {
	if cp.TargetType != nil && cp.TargetType.Kind() == reflect.Pointer && cp.TargetType.Elem() == cp.SourceType {
		return conversionView{Expr: "new(" + getter + ")", Nullable: true}
	}
	switch cp.Kind {
	case schema.ConvTime:
		v.UsesTime = true
		return conversionView{Expr: "resource.NormalizeTimestamp(" + getter + ".AsTime())"}
	case schema.ConvEnum:
		markEnum(gf, cp)
		return conversionView{Expr: "ENUM_TO_MODEL(" + getter + ")", IsEnum: true}
	case schema.ConvNarrow:
		cv := conversionView{Expr: cp.TargetType.Name() + "(" + getter + ")"}
		if cp.NeedsBounds {
			cv.NeedsBounds = true
			cv.NarrowCheck = getter + " < math.MinInt16 || " + getter + " > math.MaxInt16"
			v.UsesMath = true
		}
		return cv
	default:
		return conversionView{Expr: getter}
	}
}

// markEnum 登记字段级 enum 辅助函数元数据（函数名后续统一替换占位符）。
func markEnum(gf *goField, cp schema.ConversionPlan) {
	gf.HasEnum = true
	gf.EnumTypeName = cp.EnumTypeName
	gf.EnumModelType = cp.ModelTypeName
}

var (
	timeT = reflect.TypeOf(time.Time{})
	tsT   = reflect.TypeOf((*timestamppb.Timestamp)(nil))
)

func isTimeField(f *schema.FieldPlan) bool {
	return f.ModelType == timeT && f.EntityType == tsT
}

func tsTypeOf(f *schema.FieldPlan) string {
	switch derefType(f.ModelType).Kind() {
	case reflect.Int64, reflect.Uint64:
		return "bigint"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return "number"
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	}
	if isTimeField(f) {
		return "string"
	}
	return "unknown"
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// protoGoNameOf 返回 Plan 中指定 Model 字段的 ProtoGoName；未命中返回空。
func typePackage(t reflect.Type) (string, string) {
	name := t.String()
	for len(name) > 0 && name[0] == '*' {
		name = name[1:]
	}
	if dot := strings.IndexByte(name, '.'); dot > 0 {
		return name[:dot], t.PkgPath()
	}
	return "", t.PkgPath()
}

func protoGoNameOf(p *schema.Plan, modelField string) string {
	if f := p.Field(modelField); f != nil {
		return f.ProtoGoName
	}
	return ""
}

// requestFieldGoName 把 proto request 字段名（snake_case）转为 Go 导出字段名。
func requestFieldGoName(requestField string) string {
	parts := strings.Split(requestField, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

// convDeclaredConsts 返回已由 *_conv.gen.go 声明的 update 列常量名集合。
func convDeclaredConsts(p *schema.Plan) map[string]bool {
	declared := map[string]bool{}
	for _, f := range p.Fields {
		if f.InUpdate {
			declared[p.Res.Name+"Column"+f.ProtoGoName] = true
		}
	}
	return declared
}

// buildListView 由 ListPlan 生成标准 List 的模板视图；无 ListBinding 返回 nil。
func buildListView(p *schema.Plan) *listView {
	op := p.Operation(schema.OperationList)
	if op == nil || op.List == nil {
		return nil
	}
	declared := convDeclaredConsts(p)
	// Key is declared by the repository template, so list sort/search/filter
	// views must not emit a second constant for the same DBName.
	declared[p.Res.Name+"Column"+protoGoNameOf(p, p.Key.ModelField)] = true
	lp := op.List
	lv := &listView{
		QueryType:      p.Res.Name + "ListQuery",
		AdapterType:    p.Res.Name + "Repository",
		KeyColumnConst: p.Res.Name + "Column" + protoGoNameOf(p, p.Key.ModelField),
		PageField:      lp.PageRequestField, KeywordField: lp.KeywordField, SortField: lp.SortField, ItemsField: lp.ItemsField, PageInfoField: lp.PageInfoField,
	}
	for _, defaultOrder := range lp.DefaultOrders {
		lv.DefaultOrders = append(lv.DefaultOrders, listDefaultOrderView{
			ColumnConst: p.Res.Name + "Column" + protoGoNameOf(p, defaultOrder.SchemaField),
			Desc:        defaultOrder.Desc,
		})
	}
	if lp.KeywordField != "" && len(lp.SearchDBNames) > 0 {
		lv.SearchConst = lowerFirst(p.Res.Name) + "SearchColumns"
		for _, f := range p.Fields {
			if !f.Decl.Searchable {
				continue
			}
			const_ := p.Res.Name + "Column" + f.ProtoGoName
			declaredInConv := declared[const_]
			declared[const_] = true
			lv.SearchFields = append(lv.SearchFields, listColumnView{
				ColumnConst:    const_,
				DBName:         f.DBName,
				DeclaredInConv: declaredInConv,
			})
		}
	}
	if lp.SortField != "" && len(lp.SortableDBNames) > 0 {
		lv.SortConst = lowerFirst(p.Res.Name) + "SortableColumns"
		for _, f := range p.Fields {
			if !f.Decl.Sortable {
				continue
			}
			const_ := p.Res.Name + "Column" + f.ProtoGoName
			declaredInConv := declared[const_]
			declared[const_] = true
			lv.SortFields = append(lv.SortFields, listColumnView{
				ColumnConst:    const_,
				DBName:         f.DBName,
				JSONName:       f.JSONName,
				DeclaredInConv: declaredInConv,
			})
		}
	}
	for _, fp := range lp.Filters {
		const_ := p.Res.Name + "Column" + protoGoNameOf(p, fp.SchemaField)
		declaredInConv := declared[const_]
		declared[const_] = true
		fv := listFilterView{
			FieldName:      requestFieldGoName(fp.RequestField),
			ModelField:     fp.SchemaField,
			ColumnConst:    const_,
			DBName:         fp.DBName,
			DeclaredInConv: declaredInConv,
			GoType:         fp.ConversionPlan.TargetType.String(),
		}
		switch fp.ConversionPlan.Kind {
		case schema.ConvEnum:
			fv.NeedsEnum = true
			fv.ValueExpr = lowerFirst(p.Res.Name) + protoGoNameOf(p, fp.SchemaField) + "ToModel(v)"
		case schema.ConvNarrow:
			fv.NeedsBounds = true
			fv.ValueExpr = fp.ConversionPlan.TargetType.String() + "(v)"
		default:
			fv.ValueExpr = "v"
			for _, f := range p.Fields {
				if f.ModelField == fp.SchemaField && f.Decl.Identifier {
					fv.ValueExpr = "strings.TrimSpace(v)"
					lv.UsesIdentifier = true
				}
			}
		}
		lv.Filters = append(lv.Filters, fv)
	}
	return lv
}

var goConvTpl = template.Must(template.New("go").Parse(`{{ .Header }}
package {{ .GoPackage }}

import (
	"fmt"
{{- if .UsesMath }}
	"math"
{{- end }}
{{- if .UsesStrings }}
	"strings"
{{- end }}

{{- if .UsesIdentity }}
 "{{ .IdentityImport }}"
{{- end }}
	"{{ .ResourceImport }}"
	{{ .ProtoPackage }} "{{ .ProtoImportPath }}"
	{{ .ModelPackage }} "{{ .ModelImportPath }}"
{{- if .UsesTime }}
	"google.golang.org/protobuf/types/known/timestamppb"
{{- end }}
)
{{ range .Fields }}{{ if .InUpdate -}}
const {{ .ColumnConst }} resource.Column = {{ printf "%q" .DBName }}
{{ end }}{{ end }}
// {{ .Name }}ToEntity model → proto entity。枚举未知值 / Timestamp 越界时返回 error。
func {{ .Name }}ToEntity(m *{{ .ModelPackage }}.{{ .ModelType }}) (*{{ .ProtoPackage }}.{{ .EntityType }}, error) {
	if m == nil {
		return nil, nil
	}
	e := &{{ .ProtoPackage }}.{{ .EntityType }}{}
{{- range .Fields }}
{{- if .ToEntity.Nullable }}
 if m.{{ .ModelField }} != nil {
  e.{{ .ProtoGoName }} = {{ .ToEntity.Expr }}
 }
{{- else if .ToEntity.IsEnum }}
	if sv, err := {{ .ToEntity.Expr }}; err != nil {
		return nil, err
	} else {
		e.{{ .ProtoGoName }} = sv
	}
{{- else if .IsTime }}
	if ts := timestamppb.New({{ "m." }}{{ .ModelField }}); ts.CheckValid() != nil {
		return nil, fmt.Errorf("字段 {{ .ProtoName }} 时间 %v 超出 protobuf Timestamp 范围", {{ "m." }}{{ .ModelField }})
	} else {
		e.{{ .ProtoGoName }} = ts
	}
{{- else }}
	e.{{ .ProtoGoName }} = {{ .ToEntity.Expr }}
{{- end }}
{{- end }}
	return e, nil
}

// {{ .Name }}CreateToModel CreateRequest → model（仅 creatable 字段）。
// 收窄转换越界时返回 error，不静默截断。
func {{ .Name }}CreateToModel(req *{{ .ProtoPackage }}.{{ .CreateType }}) (*{{ .ModelPackage }}.{{ .ModelType }}, error) {
	m := &{{ .ModelPackage }}.{{ .ModelType }}{}
{{- range .Fields }}{{ if .InCreate }}
{{- if .Create.NeedsBounds }}
	if {{ .Create.NarrowCheck }} {
		return nil, fmt.Errorf("字段 {{ .ProtoName }} 值 %d 超出 int16 范围", {{ "req.Get" }}{{ .ProtoGoName }}())
	}
{{- end }}
{{- if .Create.IsEnum }}
	if ev, err := {{ .Create.Expr }}; err != nil {
		return nil, err
	} else {
		m.{{ .ModelField }} = ev
	}
{{- else if .CreateOptional }}
 if req.{{ .ProtoGoName }} != nil { m.{{ .ModelField }} = {{ .Create.Expr }} }
{{- else }}
	m.{{ .ModelField }} = {{ .Create.Expr }}
{{- end }}
{{- end }}{{ end }}
	return m, nil
}

// {{ .Name }}UpdateToModel UpdateRequest 应用到 oldM，返回变更列。
// 仅处理 presence 字段（req.X != nil），省略字段不覆盖旧值。
func {{ .Name }}UpdateToModel(oldM *{{ .ModelPackage }}.{{ .ModelType }}, req *{{ .ProtoPackage }}.{{ .UpdateType }}) (*{{ .ModelPackage }}.{{ .ModelType }}, resource.ColumnSet, error) {
	m := *oldM
	var columns []resource.Column
{{- range .Fields }}{{ if .InUpdate }}
	if req.{{ .ProtoGoName }} != nil {
{{- if .Update.NeedsBounds }}
		if {{ .Update.NarrowCheck }} {
			return nil, resource.ColumnSet{}, fmt.Errorf("字段 {{ .ProtoName }} 值 %d 超出 int16 范围", {{ "req.Get" }}{{ .ProtoGoName }}())
		}
{{- end }}
{{- if .Update.IsEnum }}
		ev, err := {{ .Update.Expr }}
		if err != nil {
			return nil, resource.ColumnSet{}, err
		}
		if ev != m.{{ .ModelField }} {
			m.{{ .ModelField }} = ev
			columns = append(columns, {{ .ColumnConst }})
		}
{{- else if .Update.Nullable }}
  if nv := {{ .Update.Expr }}; m.{{ .ModelField }} == nil || *nv != *m.{{ .ModelField }} {
   m.{{ .ModelField }} = nv
   columns = append(columns, {{ .ColumnConst }})
  }
{{- else }}
		if nv := {{ .Update.Expr }}; nv != m.{{ .ModelField }} {
			m.{{ .ModelField }} = nv
			columns = append(columns, {{ .ColumnConst }})
		}
{{- end }}
	}
{{- end }}{{ end }}
{{- range .Fields }}{{ if .CreateOnlyInUpdate }}
	// {{ .ProtoGoName }} 为 CreateOnly：复用 Update 转换（含溢出/枚举校验），与旧值不同则拒绝。
	// converter 返回普通 error，由 Service 按方向包装错误码。
	if req.{{ .ProtoGoName }} != nil {
{{- if .Update.NeedsBounds }}
		if {{ .Update.NarrowCheck }} {
			return nil, resource.ColumnSet{}, fmt.Errorf("字段 {{ .ProtoName }} 值 %d 超出 int16 范围", {{ "req.Get" }}{{ .ProtoGoName }}())
		}
{{- end }}
{{- if .Update.IsEnum }}
		cv, err := {{ .Update.Expr }}
		if err != nil {
			return nil, resource.ColumnSet{}, err
		}
		if cv != m.{{ .ModelField }} {
			return nil, resource.ColumnSet{}, fmt.Errorf("字段 {{ .ProtoName }} 创建后不可修改")
		}
{{- else if .Update.Nullable }}
  if nv := {{ .Update.Expr }}; m.{{ .ModelField }} == nil || *nv != *m.{{ .ModelField }} {
   return nil, resource.ColumnSet{}, fmt.Errorf("字段 {{ .ProtoName }} 创建后不可修改")
  }
{{- else }}
		if nv := {{ .Update.Expr }}; nv != m.{{ .ModelField }} {
			return nil, resource.ColumnSet{}, fmt.Errorf("字段 {{ .ProtoName }} 创建后不可修改")
		}
{{- end }}
	}
{{- end }}{{ end }}
	return &m, resource.NewColumnSet(columns...), nil
}

// {{ .Name }}ValidateCreate 按 Schema Required/MaxLen 校验 Create 请求字段。
// Create 字段非 optional，零值即提交值：Required 校验非空，MaxLen 校验长度。
func {{ .Name }}ValidateCreate(req *{{ .ProtoPackage }}.{{ .CreateType }}) error {
{{- range .Fields }}{{ if and .InCreate (or .ValRequired .ValMaxLen) .IsString }}
	{
		v := {{ if .Identifier }}identity.NormalizeIdentifier({{ end }}req.Get{{ .ProtoGoName }}(){{ if .Identifier }}){{ end }}
{{- if .ValRequired }}
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("字段 {{ .ProtoName }} 不能为空")
		}
{{- end }}
{{- if gt .ValMaxLen 0 }}
		if len([]rune(v)) > {{ .ValMaxLen }} {
			return fmt.Errorf("字段 {{ .ProtoName }} 长度不能超过 {{ .ValMaxLen }}")
		}
{{- end }}
	}
{{- end }}{{ end }}
	return nil
}

// {{ .Name }}ValidateUpdate 按 Schema Required/MaxLen 校验提交的 Update 字段（presence 感知）。
// 返回第一个违规字段的错误，无违规返回 nil。由 Service 在 converter 之前调用，
// 保证后端校验与 Go Schema 单一来源，不再手写复制。
// {{ .Name }}HasUpdateFields reports whether any patch field (including
// CreateOnly fields whose submitted value must be validated) was present.
// It is generated from FieldPlan presence metadata, so Schema additions cannot
// drift into a hand-maintained service field list.
func {{ .Name }}HasUpdateFields(req *{{ .ProtoPackage }}.{{ .UpdateType }}) bool {
{{- range .Fields }}{{ if or .InUpdate .CreateOnlyInUpdate }}
	if req.{{ .ProtoGoName }} != nil {
		return true
	}
{{- end }}{{ end }}
	return false
}

func {{ .Name }}ValidateUpdate(req *{{ .ProtoPackage }}.{{ .UpdateType }}) error {
{{- range .Fields }}{{ if and (or .InUpdate .CreateOnlyInUpdate) .Update.NeedsBounds }}
	if req.{{ .ProtoGoName }} != nil {
		if {{ .Update.NarrowCheck }} {
			return fmt.Errorf("字段 {{ .ProtoName }} 值 %d 超出 int16 范围", {{ "req.Get" }}{{ .ProtoGoName }}())
		}
	}
{{- end }}{{ end }}
{{- range .Fields }}{{ if and (or .InUpdate .CreateOnlyInUpdate) (or .ValRequired .ValMaxLen) .IsString }}
	if req.{{ .ProtoGoName }} != nil {
		v := {{ if .Identifier }}identity.NormalizeIdentifier({{ end }}*req.{{ .ProtoGoName }}{{ if .Identifier }}){{ end }}
{{- if .ValRequired }}
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("字段 {{ .ProtoName }} 不能为空")
		}
{{- end }}
{{- if gt .ValMaxLen 0 }}
		if len([]rune(v)) > {{ .ValMaxLen }} {
			return fmt.Errorf("字段 {{ .ProtoName }} 长度不能超过 {{ .ValMaxLen }}")
		}
{{- end }}
	}
{{- end }}{{ end }}
	return nil
}
{{- range .Fields }}{{ if .HasEnum }}

// {{ .EnumToModel }} proto 枚举 → model {{ .EnumModelType }}（未知值返回 error）。
func {{ .EnumToModel }}(v {{ $.ProtoPackage }}.{{ .EnumTypeName }}) ({{ .EnumModelType }}, error) {
	switch v {
{{- range .EnumEntries }}
	case {{ $.ProtoPackage }}.{{ .ConstRef }}:
		return {{ .Value }}, nil
{{- end }}
	default:
		return 0, fmt.Errorf("未知枚举值 %v", v)
	}
}

// {{ .EnumFromModel }} model {{ .EnumModelType }} → proto 枚举（未知值返回 error）。
func {{ .EnumFromModel }}(v {{ .EnumModelType }}) ({{ $.ProtoPackage }}.{{ .EnumTypeName }}, error) {
	switch v {
{{- range .EnumEntries }}
	case {{ .Value }}:
		return {{ $.ProtoPackage }}.{{ .ConstRef }}, nil
{{- end }}
	default:
		return 0, fmt.Errorf("未知 model 值 %d", v)
	}
}
{{- end }}{{ end }}
`))

var goRepoTpl = template.Must(template.New("repo").Funcs(template.FuncMap{"lowerFirst": lowerFirst}).Parse(`{{ .Header }}
package {{ .GoPackage }}

import (
	"context"
	"fmt"
	"strings"
{{- if not .ReadRepository }}
	"time"
{{- end }}

	"{{ .ModelImportPath }}"
	"{{ .ResourceImport }}"
	genquery "{{ .GenQueryImport }}"
{{- if not .ReadRepository }}
	"gorm.io/gen/field"
	"gorm.io/gorm/clause"
{{- end }}
)

// 列常量：Key/Searchable/Sortable/Filterable/UpdatedAt；已在 *_conv.gen.go 声明的 update 列不重复。
const {{ .KeyColumnConst }} resource.Column = {{ printf "%q" .KeyDBName }}
const {{ .UpdatedAtColumnConst }} resource.Column = {{ printf "%q" .UpdatedAtDBName }}
{{ range .List.SearchFields -}}
{{ if not .DeclaredInConv -}}
const {{ .ColumnConst }} resource.Column = {{ printf "%q" .DBName }}
{{ end -}}
{{ end }}
{{- range .List.SortFields }}
{{ if not .DeclaredInConv -}}
const {{ .ColumnConst }} resource.Column = {{ printf "%q" .DBName }}
{{ end -}}
{{ end }}
{{- range .List.Filters }}
{{ if not .DeclaredInConv -}}
const {{ .ColumnConst }} resource.Column = {{ printf "%q" .DBName }}
{{ end -}}
{{ end }}
// 编译期接口断言：{{ .List.AdapterType }} 必须实现当前 Resource 声明的只读/完整 Repository contract。
{{- if .ReadRepository }}
var _ resource.ReadRepository[{{ .ModelPackage }}.{{ .ModelType }}, {{ .List.QueryType }}] = (*{{ .List.AdapterType }})(nil)
{{- else }}
var _ resource.Repository[{{ .ModelPackage }}.{{ .ModelType }}, {{ .List.QueryType }}] = (*{{ .List.AdapterType }})(nil)
{{- end }}

// {{ .List.QueryType }} 是 {{ .Name }} 标准 List 的规范化 typed 查询。
// 由 Controller 按 ListBinding/FilterPlan 构造；adapter 不读取原始 proto Request。
type {{ .List.QueryType }} struct {
	PageNum  int
	PageSize int
	Keyword  string
{{- range .List.Filters }}
	{{ .FieldName }} *{{ .GoType }}
{{- end }}
	Orders []resource.Order // 白名单解析后的排序项，按请求顺序
}

// {{ .Name }}ListCacheKey is comparable and canonical for unrestricted cache entries.
type {{ .Name }}ListCacheKey struct {
	PageNum int
	PageSize int
	Keyword string
	Orders string
{{- range .List.Filters }}
	{{ .FieldName }}Present bool
	{{ .FieldName }} {{ .GoType }}
{{- end }}
}

func New{{ .Name }}ListCacheKey(q {{ .List.QueryType }}) {{ .Name }}ListCacheKey {
	if q.PageNum < 1 { q.PageNum = 1 }
	if q.PageSize <= 0 { q.PageSize = resource.DefaultPageSize }
	if q.PageSize > resource.MaxPageSize { q.PageSize = resource.MaxPageSize }
	key := {{ .Name }}ListCacheKey{PageNum: q.PageNum, PageSize: q.PageSize, Keyword: strings.TrimSpace(q.Keyword)}
	var orders strings.Builder
	for _, order := range q.Orders { fmt.Fprintf(&orders, "%q:%t;", order.Column, order.Desc) }
	key.Orders = orders.String()
{{- range .List.Filters }}
	if q.{{ .FieldName }} != nil { key.{{ .FieldName }}Present = true; key.{{ .FieldName }} = *q.{{ .FieldName }} }
{{- end }}
	return key
}
{{ if .List.SearchConst -}}
var {{ .List.SearchConst }} = []resource.Column{
{{- range .List.SearchFields }}
	{{ .ColumnConst }},
{{- end }}
}
{{ end -}}
{{ if .List.SortConst -}}
// {{ .List.SortConst }} 是 OrderBy.field（protobuf JSON name）→ DB 列的白名单。
var {{ .List.SortConst }} = map[string]resource.Column{
{{- range .List.SortFields }}
	{{ printf "%q" .JSONName }}: {{ .ColumnConst }},
{{- end }}
}
{{ end }}
// {{ .List.AdapterType }} 是 {{ .Name }} 的默认生成 Repository adapter。
type {{ .List.AdapterType }} struct{}

// New{{ .List.AdapterType }} 构造默认 adapter（无状态，仅满足构造一致性）。
func New{{ .List.AdapterType }}() *{{ .List.AdapterType }} { return &{{ .List.AdapterType }}{} }

{{- if not .ReadRepository }}
func (a *{{ .List.AdapterType }}) Create(ctx context.Context, r resource.Session, m *{{ .ModelPackage }}.{{ .ModelType }}) error {
	item := genquery.Use(r.DB().WithContext(ctx)).{{ .ModelType }}
	// DO.Create retains the same pointer; the caller's transaction owns readback.
 if err := resource.NormalizeDBError(item.WithContext(ctx).DO.Create(m)); err != nil { return err }
 saved, err := item.WithContext(ctx).Where(item.{{ .KeyModelField }}.Eq(m.{{ .KeyModelField }})).First()
 if err := resource.NormalizeDBError(err); err != nil { return err }
 *m = *saved
 return nil
}

{{- end }}

func (a *{{ .List.AdapterType }}) GetByKey(ctx context.Context, r resource.Session, key int64, access resource.AccessScope) (*{{ .ModelPackage }}.{{ .ModelType }}, error) {
	item := genquery.Use(access.Apply(r.DB().WithContext(ctx))).{{ .ModelType }}
	model, err := item.WithContext(ctx).Where(item.{{ .KeyModelField }}.Eq(key)).First()
	if err := resource.NormalizeDBError(err); err != nil {
		return nil, err
	}
	return model, nil
}

{{- if not .ReadRepository }}
func (a *{{ .List.AdapterType }}) GetByKeyForUpdate(ctx context.Context, r resource.Session, key int64, access resource.AccessScope) (*{{ .ModelPackage }}.{{ .ModelType }}, error) {
	item := genquery.Use(access.Apply(r.DB().WithContext(ctx))).{{ .ModelType }}
	model, err := item.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(item.{{ .KeyModelField }}.Eq(key)).First()
	if err := resource.NormalizeDBError(err); err != nil {
		return nil, err
	}
	return model, nil
}

// Update 在调用方提供的 repo handle（通常为 Controller 持有的 txRepo）上执行：
// scoped 锁定读取后做 selective update，并以单个 now 同时写 Model.UpdatedAt 与
// 独立 UpdatedAt 列。事务边界由调用方（Controller）决定，adapter 不自开事务。
func (a *{{ .List.AdapterType }}) Update(ctx context.Context, r resource.Session, key int64, m *{{ .ModelPackage }}.{{ .ModelType }}, columns resource.ColumnSet, access resource.AccessScope) error {
	if err := {{ .Name }}ValidateUpdateColumns(columns); err != nil {
		return err
	}
	// Use one normalized timestamp for this write and read back the stored result.
	now := resource.NormalizeTimestamp(time.Now())
	if prev := resource.NormalizeTimestamp(m.{{ .UpdatedAtModelField }}); !prev.IsZero() && !now.After(prev) {
		now = prev.Add(time.Microsecond)
	}
	m.{{ .UpdatedAtModelField }} = now
	if _, err := a.GetByKeyForUpdate(ctx, r, key, access); err != nil {
		return err
	}
	item := genquery.Use(access.Apply(r.DB().WithContext(ctx))).{{ .ModelType }}
	assignments := []field.AssignExpr{item.{{ .UpdatedAtModelField }}.Value(now)}
	for _, column := range columns.Values() {
		switch column {
{{- range .Fields }}
{{- if and .InUpdate .Updatable }}
		case {{ .ColumnConst }}:
			{{- if .ModelNullable }}
   if m.{{ .ModelField }} == nil { assignments = append(assignments,item.{{ .ModelField }}.Null()) } else { assignments = append(assignments,item.{{ .ModelField }}.Value(*m.{{ .ModelField }})) }
{{- else }}
   assignments = append(assignments, item.{{ .ModelField }}.Value(m.{{ .ModelField }}))
{{- end }}
{{- end }}
{{- end }}
		}
	}
 _, err := item.WithContext(ctx).Where(item.{{ .KeyModelField }}.Eq(key)).UpdateColumnSimple(assignments...)
 if err := resource.NormalizeDBError(err); err != nil { return err }
 saved, err := a.GetByKey(ctx, r, key, access)
 if err != nil { return err }
 *m = *saved
 return nil

}

// DeleteByKey 在调用方 repo handle 上 scoped 锁定读取后删除；事务边界由调用方决定。
func (a *{{ .List.AdapterType }}) DeleteByKey(ctx context.Context, r resource.Session, key int64, access resource.AccessScope) error {
	if _, err := a.GetByKeyForUpdate(ctx, r, key, access); err != nil {
		return err
	}
	item := genquery.Use(access.Apply(r.DB().WithContext(ctx))).{{ .ModelType }}
	result, err := item.WithContext(ctx).Where(item.{{ .KeyModelField }}.Eq(key)).Delete()
	if err := resource.NormalizeDBError(err); err != nil {
		return err
	}
	if result.RowsAffected == 0 { return resource.ErrNotFound }
 return nil
}

func (a *{{ .List.AdapterType }}) GetManyForUpdate(ctx context.Context, r resource.Session, keys []int64, access resource.AccessScope) ([]*{{ .ModelPackage }}.{{ .ModelType }}, error) {
	item := genquery.Use(access.Apply(r.DB().WithContext(ctx))).{{ .ModelType }}
	models, err := item.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(item.{{ .KeyModelField }}.In(keys...)).Find()
	if err := resource.NormalizeDBError(err); err != nil {
		return nil, err
	}
	return models, nil
}

// BatchDelete 保证全有或全无（需求 11.6/417）：拒绝空集合；先对 ID 去重，
// scoped SELECT ... FOR UPDATE 加载并校验数量等于 distinct ID 数，不等返回
// NotFound；删除后再校验 RowsAffected 等于 distinct ID 数，不等返回错误。
// 事务边界由调用方决定：数量不匹配时返回 error，调用方 rollback 即零删除。
func (a *{{ .List.AdapterType }}) BatchDelete(ctx context.Context, r resource.Session, keys []int64, access resource.AccessScope) (int64, error) {
	if len(keys) == 0 {
		return 0, fmt.Errorf("{{ .Name }}: BatchDelete ID 集合不能为空")
	}
	distinct := make([]int64, 0, len(keys))
	seen := make(map[int64]struct{}, len(keys))
	for _, key := range keys {
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		distinct = append(distinct, key)
	}
	models, err := a.GetManyForUpdate(ctx, r, distinct, access)
	if err != nil {
		return 0, err
	}
	if len(models) != len(distinct) {
		// 任一 ID 不存在或不在 scope，对外统一 NotFound，避免存在性泄漏。
		return 0, resource.ErrNotFound
	}
	item := genquery.Use(access.Apply(r.DB().WithContext(ctx))).{{ .ModelType }}
	result, err := item.WithContext(ctx).Where(item.{{ .KeyModelField }}.In(distinct...)).Delete()
	if err := resource.NormalizeDBError(err); err != nil {
		return 0, err
	}
	affected := result.RowsAffected
	if affected == 0 {
		return 0, resource.ErrNotFound
	}
	if affected != int64(len(distinct)) {
		return 0, fmt.Errorf("{{ .Name }}: BatchDelete 影响行数 %d 与 distinct ID 数 %d 不一致", affected, len(distinct))
	}
	return affected, nil
}

{{- end }}

func (a *{{ .List.AdapterType }}) List(ctx context.Context, r resource.Session, q {{ .List.QueryType }}, access resource.AccessScope) ([]*{{ .ModelPackage }}.{{ .ModelType }}, int64, error) {
	db := access.Apply(r.DB().WithContext(ctx))
{{- if .List.SearchConst }}
	db = resource.Search({{ .List.SearchConst }}, q.Keyword).Apply(db)
{{- end }}
	item := genquery.Use(db).{{ .ModelType }}
	query := item.WithContext(ctx)
{{- range .List.Filters }}
	if q.{{ .FieldName }} != nil {
		query = query.Where(item.{{ .ModelField }}.Eq(*q.{{ .FieldName }}))
	}
{{- end }}
	total, err := query.Count()
	if err != nil {
		return nil, 0, resource.NormalizeDBError(err)
	}
	orders := q.Orders
{{- if .List.DefaultOrders }}
	if len(orders) == 0 {
		orders = []resource.Order{
{{- range .List.DefaultOrders }}
			{Column: {{ .ColumnConst }}, Desc: {{ .Desc }}},
{{- end }}
		}
	}
{{- end }}
	for _, order := range resource.StableOrders(orders, {{ .List.KeyColumnConst }}) {
		column, _ := item.GetFieldByName(string(order.Column))
		if order.Desc {
			query = query.Order(column.Desc())
		} else {
			query = query.Order(column)
		}
	}
	page, size := resource.NormalizePage(q.PageNum, q.PageSize)
	rows, err := query.Offset((page - 1) * size).Limit(size).Find()
	if err != nil {
		return nil, 0, resource.NormalizeDBError(err)
	}
	return rows, total, nil

}

// {{ lowerFirst .Name }}UpdateColumnPolicy 仅含 EffUpdatable 字段 DBName（需求 10.10）。
var {{ lowerFirst .Name }}UpdateColumnPolicy = resource.NewColumnPolicy(
{{- range .UpdatableColumnConsts }}
	{{ . }},
{{- end }}
)

// {{ .Name }}ValidateUpdateColumns 校验 selective Update 的列集合：
// 非空、无重复、每列属于当前资源的 EffUpdatable 白名单；失败返回内部契约错误。
// Injected Repository/typed decorator 必须复用本函数或委托已验证的默认 adapter。
func {{ .Name }}ValidateUpdateColumns(columns resource.ColumnSet) error {
	if columns.Len() == 0 {
		return fmt.Errorf("{{ .Name }}: update ColumnSet 不能为空")
	}
	seen := map[resource.Column]struct{}{}
	for _, column := range columns.Values() {
		if _, dup := seen[column]; dup {
			return fmt.Errorf("{{ .Name }}: update ColumnSet 含重复列 %q", column)
		}
		seen[column] = struct{}{}
	}
	if err := {{ lowerFirst .Name }}UpdateColumnPolicy.Validate(columns); err != nil {
		return fmt.Errorf("{{ .Name }}: %w", err)
	}
	return nil
}

// {{ .Name }}ResolveOrder 把 proto OrderBy.field（JSON name）解析为白名单 resource.Order。
// 未知字段返回 InvalidArgument 语义错误；调用方（Controller）不得直接拼接客户端字段名。
func {{ .Name }}ResolveOrder(field string, desc bool) (resource.Order, error) {
{{- if .List.SortConst }}
	column, ok := {{ .List.SortConst }}[strings.TrimSpace(field)]
	if !ok {
		return resource.Order{}, fmt.Errorf("{{ .Name }}: 未知排序字段 %q", field)
	}
	return resource.Order{Column: column, Desc: desc}, nil
{{- else }}
	return resource.Order{}, fmt.Errorf("{{ .Name }}: 资源不支持排序字段 %q", field)
{{- end }}
}
`))

var goPermissionsTpl = template.Must(template.New("permissions").Parse(`{{ .Header }}
package {{ .GoPackage }}

import permission "{{ .PermissionImport }}"

// Register{{ .Name }}Permissions explicitly registers this Resource's declared
// standard RPC permissions during application bootstrap. It has no init side effect.
func Register{{ .Name }}Permissions(reg permission.Registrar) error {
	return reg.RegisterGenerated(
{{- range .Permissions }}
		permission.Entry{Procedure: {{ printf "%q" .Procedure }}, Permission: {{ printf "%q" .Permission }}},
{{- end }}
	)
}
`))

var goCustomControllerTpl = template.Must(template.New("custom-controller").Parse(`{{ .Header }}
package {{ .GoPackage }}

import (
  "context"
  "fmt"

  "{{ .ProtoImportPath }}"
)

// {{ .Name }}ControllerDeps contains only the declared CUSTOM override.
type {{ .Name }}ControllerDeps struct {
  Overrides {{ .Name }}Overrides
}

// {{ .Name }}Overrides contains exactly this Partial Resource's CUSTOM RPCs.
type {{ .Name }}Overrides interface {
{{- range .Operations }}
{{- if .IsCustom }}
  {{ .Method }}(ctx context.Context, req *{{ .RequestRef }}) (*{{ .ResponseRef }}, error)
{{- end }}
{{- end }}
}

type {{ .Name }}Controller struct {
  overrides {{ .Name }}Overrides
}

func New{{ .Name }}Controller(deps {{ .Name }}ControllerDeps) (*{{ .Name }}Controller, error) {
  if deps.Overrides == nil {
    return nil, fmt.Errorf("{{ .Name }}: Partial requires non-nil Overrides")
  }
  return &{{ .Name }}Controller{overrides: deps.Overrides}, nil
}

{{- range .Operations }}
{{- if .IsCustom }}
func (c *{{ $.Name }}Controller) {{ .Method }}(ctx context.Context, req *{{ .RequestRef }}) (*{{ .ResponseRef }}, error) {
  return c.overrides.{{ .Method }}(ctx, req)
}
{{- end }}
{{- end }}
`))

var goControllerTpl = template.Must(template.New("controller").Funcs(template.FuncMap{"lowerFirst": lowerFirst}).Parse(`{{ .Header }}
package {{ .GoPackage }}

import (
	"context"
{{- if and .List .List.UsesIdentifier }}
 "strings"
{{- end }}
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect/v2"

	"{{ .ModelImportPath }}"
	{{ .ProtoPackage }} "{{ .ProtoImportPath }}"
{{- if and .PageInfoImportPath (ne .PageInfoImportPath .ProtoImportPath) }}
	{{ .PageInfoPackage }} "{{ .PageInfoImportPath }}"
{{- end }}
	"{{ .ResourceImport }}"
)

// {{ .Name }}ControllerDeps 仅暴露 {{ .Name }} 实际需要的强类型依赖（需求 8.5）。
type {{ .Name }}ControllerDeps struct {
	Store *resource.Store
	Hooks    *resource.HookSet[{{ .ModelRef }}]
{{- if .RepositoryInjected }}
	// Repository 仅在 RepositoryInjected 时存在；nil 返回构造错误。
{{- if .ReadRepository }}
	Repository resource.ReadRepository[{{ .ModelRef }}, {{ .List.QueryType }}]
{{- else }}
	Repository resource.Repository[{{ .ModelRef }}, {{ .List.QueryType }}]
{{- end }}
{{- end }}
{{- if .Partial }}
	// Overrides 仅在 Partial 时存在；nil 返回构造错误。
	Overrides {{ .Name }}Overrides
{{- end }}
{{- if .DataScope }}
	// DataScopeProvider 仅在 data_scope=true 时存在；nil 返回构造错误。
	DataScopeProvider resource.DataScopeProvider
{{- end }}
{{- if .CreateScopeCheck }}
	// CreateScopeValidator 仅在需要 create scope check 时存在；nil 返回构造错误。
	CreateScopeValidator resource.CreateScopeValidator[{{ .ModelRef }}]
{{- end }}
}
{{- if .Partial }}

// {{ .Name }}Overrides contains exactly this Partial Resource's CUSTOM RPCs.
// Each method mirrors the Connect handler signature, so delegation needs no
// runtime type assertion or fallback panic.
type {{ .Name }}Overrides interface {
{{- range .Operations }}
{{- if .IsCustom }}
	{{ .Method }}(ctx context.Context, req *{{ .RequestRef }}) (*{{ .ResponseRef }}, error)
{{- end }}
{{- end }}
}
{{- end }}

// {{ .Name }}Controller 编排 {{ .Name }} 标准六 RPC：校验 → converter → HookSet → Repository → AfterCommit。
type {{ .Name }}Controller struct {
	store *resource.Store
	hooks    *resource.HookSet[{{ .ModelRef }}]
{{- if .ReadRepository }}
	repo     resource.ReadRepository[{{ .ModelRef }}, {{ .List.QueryType }}]
{{- else }}
	repo     resource.Repository[{{ .ModelRef }}, {{ .List.QueryType }}]
{{- end }}
{{- if .Partial }}
	overrides {{ .Name }}Overrides
{{- end }}
{{- if .DataScope }}
	dataScope resource.DataScopeProvider
{{- end }}
{{- if .CreateScopeCheck }}
	createScopeValidator resource.CreateScopeValidator[{{ .ModelRef }}]
{{- end }}
}

// {{ lowerFirst .Name }}UpdateInputError preserves InvalidArgument semantics
// for request→model conversion failures discovered after the scoped lock read.
type {{ lowerFirst .Name }}UpdateInputError struct{ err error }

func (e *{{ lowerFirst .Name }}UpdateInputError) Error() string { return e.err.Error() }
func (e *{{ lowerFirst .Name }}UpdateInputError) Unwrap() error { return e.err }

// {{ lowerFirst .Name }}RunAfterCommit isolates post-commit side effects from
// the already-successful transaction and response. A panic is recovered and
// logged structurally rather than changing the RPC result.
func {{ lowerFirst .Name }}RepositoryError(err error) error {
	// Hooks may intentionally return a typed Connect error (for example a
	// domain validation failure), so preserve its code before repository
	// normalization. This keeps generated controllers compatible with
	// specialized Resource boundaries without adding per-resource CRUD paths.
	if code := connect.CodeOf(err); code != connect.CodeUnknown {
		return err
	}
	if errors.Is(err, resource.ErrNotFound) {
		return connect.NewError(connect.CodeNotFound, resource.ErrNotFound.Error()).WithCause(resource.ErrNotFound)
	}
	if errors.Is(err, resource.ErrDuplicate) {
		return connect.NewError(connect.CodeAlreadyExists, resource.ErrDuplicate.Error()).WithCause(resource.ErrDuplicate)
	}
	if errors.Is(err, resource.ErrCycle) {
		return connect.NewError(connect.CodeInvalidArgument, resource.ErrCycle.Error()).WithCause(resource.ErrCycle)
	}
	if errors.Is(err, resource.ErrOutOfRange) {
		return connect.NewError(connect.CodeInvalidArgument, resource.ErrOutOfRange.Error()).WithCause(resource.ErrOutOfRange)
	}
	if errors.Is(err, resource.ErrHasChildren) {
		return connect.NewError(connect.CodeFailedPrecondition, resource.ErrHasChildren.Error()).WithCause(resource.ErrHasChildren)
	}
	// A database rule (trigger SIGNAL / RAISE EXCEPTION) carries a message the
	// application wrote for users.
	var rejected *resource.RejectedError
	if errors.As(err, &rejected) {
		return connect.NewError(connect.CodeFailedPrecondition, rejected.Message).WithCause(err)
	}
	if errors.Is(err, resource.ErrForeignKey) {
		return connect.NewError(connect.CodeFailedPrecondition, resource.ErrForeignKey.Error()).WithCause(err)
	}
	if errors.Is(err, resource.ErrCheckViolation) {
		return connect.NewError(connect.CodeInvalidArgument, resource.ErrCheckViolation.Error()).WithCause(err)
	}
	// Driver messages name databases, tables and constraints: log the cause
	// and return a generic message.
	slog.Error("resource request failed", "resource", "{{ .Name }}", "error", err)
	return connect.NewError(connect.CodeInternal, resource.ErrInternal.Error()).WithCause(err)
}

func {{ lowerFirst .Name }}RunAfterCommit(hook string, fn func()) {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("resource after-commit hook panicked", "resource", "{{ .Name }}", "hook", hook, "panic", recovered)
		}
	}()
	fn()
}

// {{ lowerFirst .Name }}BatchDeleteLimit is the frozen v1 default maximum after ID deduplication.
const {{ lowerFirst .Name }}BatchDeleteLimit = 1000

func (c *{{ .Name }}Controller) accessScope(ctx context.Context) resource.AccessScope {
{{- if .DataScope }}
	return c.dataScope.DataScope(ctx, "{{ .DeptDBName }}", "{{ .CreatorDBName }}")
{{- else }}
	return resource.UnrestrictedAccessScope()
{{- end }}
}

// New{{ .Name }}Controller 构造 {{ .Name }} Controller；任何必需依赖为 nil 均返回 error（不 panic）。
func New{{ .Name }}Controller(deps {{ .Name }}ControllerDeps) (*{{ .Name }}Controller, error) {
	if deps.Store == nil || deps.Store.DB() == nil {
		return nil, fmt.Errorf("{{ .Name }}: Store 不能为 nil")
	}
	hooks := deps.Hooks
	if hooks == nil {
		hooks = &resource.HookSet[{{ .ModelRef }}]{}
	}
	c := &{{ .Name }}Controller{store: deps.Store, hooks: hooks}
{{- if .RepositoryInjected }}
	if deps.Repository == nil {
		return nil, fmt.Errorf("{{ .Name }}: RepositoryInjected 要求非 nil Repository")
	}
	c.repo = deps.Repository
{{- else }}
	c.repo = New{{ .List.AdapterType }}()
{{- end }}
{{- if .Partial }}
	if deps.Overrides == nil {
		return nil, fmt.Errorf("{{ .Name }}: Partial 要求非 nil Overrides")
	}
	c.overrides = deps.Overrides
{{- end }}
{{- if .DataScope }}
	if deps.DataScopeProvider == nil {
		return nil, fmt.Errorf("{{ .Name }}: data_scope=true 要求非 nil DataScopeProvider")
	}
	c.dataScope = deps.DataScopeProvider
{{- end }}
{{- if .CreateScopeCheck }}
	if deps.CreateScopeValidator == nil {
		return nil, fmt.Errorf("{{ .Name }}: create scope check 要求非 nil CreateScopeValidator")
	}
	c.createScopeValidator = deps.CreateScopeValidator
{{- end }}
	return c, nil
}
{{ range .Operations }}
{{- if .IsCustom }}
// {{ .Method }} delegates the declared CUSTOM operation to the typed Partial override.
func (c *{{ $.Name }}Controller) {{ .Method }}(ctx context.Context, req *{{ .RequestRef }}) (*{{ .ResponseRef }}, error) {
	return c.overrides.{{ .Method }}(ctx, req)
}
{{- else if .IsList }}
func (c *{{ $.Name }}Controller) {{ .Method }}(ctx context.Context, req *{{ .RequestRef }}) (*{{ .ResponseRef }}, error) {
	q := {{ $.List.QueryType }}{PageNum: int(req.Get{{ $.List.PageField }}().GetPageNum()), PageSize: int(req.Get{{ $.List.PageField }}().GetPageSize()), Keyword: req.Get{{ $.List.KeywordField }}()}
{{- range $.List.Filters }}
	if raw := req.{{ .FieldName }}; raw != nil {
		v := *raw
{{- if .NeedsEnum }}
		value, err := {{ .ValueExpr }}
		if err != nil { return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err) }
		q.{{ .FieldName }} = &value
{{- else if .NeedsBounds }}
		if v < -32768 || v > 32767 { return nil, connect.Errorf(connect.CodeInvalidArgument, "{{ .FieldName }} out of int16 range") }
		value := {{ .ValueExpr }}
		q.{{ .FieldName }} = &value
{{- else }}
		value := {{ .ValueExpr }}
		q.{{ .FieldName }} = &value
{{- end }}
	}
{{- end }}
	if q.PageNum < 1 { q.PageNum = 1 }
	if q.PageSize < 0 { return nil, connect.Errorf(connect.CodeInvalidArgument, "page_size must not be negative") }
	if q.PageSize == 0 { q.PageSize = resource.DefaultPageSize }
	if q.PageSize > resource.MaxPageSize { q.PageSize = resource.MaxPageSize }
{{- if $.List.SortField }}
	for _, order := range req.Get{{ $.List.SortField }}() { resolved, err := {{ $.Name }}ResolveOrder(order.GetField(), order.GetDesc()); if err != nil { return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err) }; q.Orders = append(q.Orders, resolved) }
{{- end }}
	models, total, err := c.repo.List(ctx, c.store, q, c.accessScope(ctx))
	if err != nil { return nil, {{ lowerFirst $.Name }}RepositoryError(err) }
	items := make([]*{{ $.ProtoPackage }}.{{ $.EntityType }}, 0, len(models))
	for _, m := range models { entity, err := {{ $.Name }}ToEntity(m); if err != nil { return nil, {{ lowerFirst $.Name }}RepositoryError(err) }; items = append(items, entity) }
	return &{{ .ResponseElemRef }}{ {{ $.List.ItemsField }}: items, {{ $.List.PageInfoField }}: &{{ $.PageInfoRef }}{Total: total, PageNum: int32(q.PageNum), PageSize: int32(q.PageSize)} }, nil
}
{{- else if .IsGet }}
func (c *{{ $.Name }}Controller) {{ .Method }}(ctx context.Context, req *{{ .RequestRef }}) (*{{ .ResponseRef }}, error) {
	m, err := c.repo.GetByKey(ctx, c.store, req.Get{{ .IdentifierField }}(), c.accessScope(ctx))
	if err != nil { return nil, {{ lowerFirst $.Name }}RepositoryError(err) }
	entity, err := {{ $.Name }}ToEntity(m)
	if err != nil { return nil, {{ lowerFirst $.Name }}RepositoryError(err) }
	return &{{ .ResponseElemRef }}{ {{ .ItemField }}: entity }, nil
}
{{- else if .IsCreate }}
// {{ .Method }} 执行 Create 的固定事务时序：先校验/转换，再在同一事务内运行
// BeforeCreate → Repository.Create → AfterCreate → ToEntity，提交成功后才运行 AfterCommitCreate。
func (c *{{ $.Name }}Controller) {{ .Method }}(ctx context.Context, req *{{ .RequestRef }}) (*{{ .ResponseRef }}, error) {
	if err := {{ $.Name }}ValidateCreate(req); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
	m, err := {{ $.Name }}CreateToModel(req)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
	var item *{{ $.ProtoPackage }}.{{ $.EntityType }}
	var createSnapshot {{ $.ModelRef }}
	if err := c.store.InTransaction(ctx, func(txRepo resource.Tx) error {
		if c.hooks.BeforeCreate != nil {
			if err := c.hooks.BeforeCreate(ctx, txRepo, m); err != nil {
				return err
			}
		}
{{- if $.CreateScopeCheck }}
		if err := c.createScopeValidator(ctx, txRepo, m); err != nil {
			return err
		}
{{- end }}
		if err := c.repo.Create(ctx, txRepo, m); err != nil {
			return err
		}
		if c.hooks.AfterCreate != nil {
			if err := c.hooks.AfterCreate(ctx, txRepo, m); err != nil {
				return err
			}
		}
		entity, err := {{ $.Name }}ToEntity(m)
		if err != nil {
			return err
		}
		item = entity
		createSnapshot = *m
		return nil
	}); err != nil {
		if errors.Is(err, resource.ErrScopeDenied) {
			return nil, connect.NewError(connect.CodePermissionDenied, err.Error()).WithCause(err)
		}
		return nil, {{ lowerFirst $.Name }}RepositoryError(err)
	}
	if c.hooks.AfterCommitCreate != nil {
		{{ lowerFirst $.Name }}RunAfterCommit("create", func() {
			c.hooks.AfterCommitCreate(ctx, createSnapshot)
		})
	}
	return &{{ .ResponseElemRef }}{ {{ .ItemField }}: item }, nil
}
{{- else if .IsUpdate }}
// {{ .Method }} 先区分全省略 patch 与提交同值的 no-op。非空变更的 Hook/审计
// 写入时序由 8.2 补全；本分支已保证 no-op 在事务内完成 ToEntity 后提交。
func (c *{{ $.Name }}Controller) {{ .Method }}(ctx context.Context, req *{{ .RequestRef }}) (*{{ .ResponseRef }}, error) {
	if err := {{ $.Name }}ValidateUpdate(req); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
	}
	if !{{ $.Name }}HasUpdateFields(req) {
		return nil, connect.Errorf(connect.CodeInvalidArgument, "至少需提交一个待更新字段")
	}
	var item *{{ $.ProtoPackage }}.{{ $.EntityType }}
	var updateSnapshot {{ $.ModelRef }}
	updated := false
	if err := c.store.InTransaction(ctx, func(txRepo resource.Tx) error {
		old, err := c.repo.GetByKeyForUpdate(ctx, txRepo, req.Get{{ .IdentifierField }}(), c.accessScope(ctx))
		if err != nil {
			return err
		}
		m, columns, err := {{ $.Name }}UpdateToModel(old, req)
		if err != nil {
			return &{{ lowerFirst $.Name }}UpdateInputError{err: err}
		}
		if columns.Len() == 0 {
			entity, err := {{ $.Name }}ToEntity(m)
			if err != nil {
				return err
			}
			item = entity
			return nil
		}
		if c.hooks.BeforeUpdate != nil {
			if err := c.hooks.BeforeUpdate(ctx, txRepo, old, m, columns); err != nil {
				return err
			}
		}
		if err := c.repo.Update(ctx, txRepo, req.Get{{ .IdentifierField }}(), m, columns, c.accessScope(ctx)); err != nil {
			return err
		}
		if c.hooks.AfterUpdate != nil {
			if err := c.hooks.AfterUpdate(ctx, txRepo, old, m, columns); err != nil {
				return err
			}
		}
		entity, err := {{ $.Name }}ToEntity(m)
		if err != nil {
			return err
		}
		item = entity
		updateSnapshot = *m
		updated = true
		return nil
	}); err != nil {
		var inputErr *{{ lowerFirst $.Name }}UpdateInputError
		if errors.As(err, &inputErr) {
			return nil, connect.NewError(connect.CodeInvalidArgument, inputErr.Error()).WithCause(inputErr)
		}
		return nil, {{ lowerFirst $.Name }}RepositoryError(err)
	}
	if updated && c.hooks.AfterCommitUpdate != nil {
		{{ lowerFirst $.Name }}RunAfterCommit("update", func() {
			c.hooks.AfterCommitUpdate(ctx, updateSnapshot)
		})
	}
	return &{{ .ResponseElemRef }}{ {{ .ItemField }}: item }, nil
}
{{- else if .IsDelete }}
// {{ .Method }} 在 scoped lock 后复制持久化快照；Hook 对 old 的内存修改不能影响提交后副作用。
func (c *{{ $.Name }}Controller) {{ .Method }}(ctx context.Context, req *{{ .RequestRef }}) (*{{ .ResponseRef }}, error) {
	var deleteSnapshot {{ $.ModelRef }}
	if err := c.store.InTransaction(ctx, func(txRepo resource.Tx) error {
		old, err := c.repo.GetByKeyForUpdate(ctx, txRepo, req.Get{{ .IdentifierField }}(), c.accessScope(ctx))
		if err != nil {
			return err
		}
		deleteSnapshot = *old
		if c.hooks.BeforeDelete != nil {
			if err := c.hooks.BeforeDelete(ctx, txRepo, old); err != nil {
				return err
			}
		}
		if err := c.repo.DeleteByKey(ctx, txRepo, req.Get{{ .IdentifierField }}(), c.accessScope(ctx)); err != nil {
			return err
		}
		if c.hooks.AfterDelete != nil {
			if err := c.hooks.AfterDelete(ctx, txRepo, old); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, {{ lowerFirst $.Name }}RepositoryError(err)
	}
	if c.hooks.AfterCommitDelete != nil {
		{{ lowerFirst $.Name }}RunAfterCommit("delete", func() {
			c.hooks.AfterCommitDelete(ctx, deleteSnapshot)
		})
	}
	return &{{ .ResponseElemRef }}{}, nil
}
{{- else if .IsBatchDelete }}
// {{ .Method }} validates the raw IDs before beginning the all-or-nothing transaction in 8.7.
func (c *{{ $.Name }}Controller) {{ .Method }}(ctx context.Context, req *{{ .RequestRef }}) (*{{ .ResponseRef }}, error) {
	rawIDs := req.Get{{ .BatchIDsField }}()
	if len(rawIDs) == 0 {
		return nil, connect.Errorf(connect.CodeInvalidArgument, "BatchDelete ID 集合不能为空")
	}
	distinctIDs := make([]int64, 0, len(rawIDs))
	seen := make(map[int64]struct{}, len(rawIDs))
	for _, id := range rawIDs {
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		distinctIDs = append(distinctIDs, id)
	}
	if len(distinctIDs) > {{ lowerFirst $.Name }}BatchDeleteLimit {
		return nil, connect.Errorf(connect.CodeInvalidArgument, "BatchDelete ID 数量超过上限 %d", {{ lowerFirst $.Name }}BatchDeleteLimit)
	}
	var batchSnapshots []{{ $.ModelRef }}
	if err := c.store.InTransaction(ctx, func(txRepo resource.Tx) error {
		models, err := c.repo.GetManyForUpdate(ctx, txRepo, distinctIDs, c.accessScope(ctx))
		if err != nil {
			return err
		}
		if len(models) != len(distinctIDs) {
			return resource.ErrNotFound
		}
		batchSnapshots = make([]{{ $.ModelRef }}, len(models))
		for i, m := range models {
			batchSnapshots[i] = *m
		}
		if c.hooks.BeforeBatchDelete != nil {
			if err := c.hooks.BeforeBatchDelete(ctx, txRepo, models); err != nil {
				return err
			}
		}
		affected, err := c.repo.BatchDelete(ctx, txRepo, distinctIDs, c.accessScope(ctx))
		if err != nil {
			return err
		}
		if affected != int64(len(distinctIDs)) {
			return fmt.Errorf("{{ $.Name }}: BatchDelete 影响行数 %d 与 distinct ID 数 %d 不一致", affected, len(distinctIDs))
		}
		if c.hooks.AfterBatchDelete != nil {
			if err := c.hooks.AfterBatchDelete(ctx, txRepo, models); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, {{ lowerFirst $.Name }}RepositoryError(err)
	}
	if c.hooks.AfterCommitBatchDelete != nil {
		{{ lowerFirst $.Name }}RunAfterCommit("batch_delete", func() {
			c.hooks.AfterCommitBatchDelete(ctx, batchSnapshots)
		})
	}
	return &{{ .ResponseElemRef }}{}, nil
}
{{- else }}
// {{ .Method }} 是 {{ $.Name }} 标准 RPC 的编排骨架。
// 完整时序由 8.2/8.2a/8.3-8.8 任务填充；骨架先固定方法集与 Controller 结构。
func (c *{{ $.Name }}Controller) {{ .Method }}(ctx context.Context, req *{{ .RequestRef }}) (*{{ .ResponseRef }}, error) {
	return nil, connect.Errorf(connect.CodeUnimplemented, "{{ $.Name }}.{{ .Method }} 骨架未实现")
}
{{- end }}
{{ end -}}
`))

var tsRawOperationsTpl = template.Must(template.New("ts-raw-operations").Parse(`{{ .Header }}
import { createClient, type Transport } from "@connectrpc/connect";
import { {{ .TSServiceName }} } from "{{ .TSServiceModule }}";

export function create{{ .Name }}Raw(transport: Transport) {
  const client = createClient({{ .TSServiceName }}, transport);
  return {
{{- range .TSRawOperations }}
    {{ .Method }}(request: Parameters<typeof client.{{ .Method }}>[0]) {
      return client.{{ .Method }}(request);
    },
{{- end }}
  };
}
export type {{ .Name }}Raw = ReturnType<typeof create{{ .Name }}Raw>;
`))

var tsOperationsTpl = template.Must(template.New("ts-operations").Parse(`{{ .Header }}
import type { {{ .Name }}Raw } from "./{{ .TSFileBase }}.raw.gen";
{{- if .TSFacade }}
import { {{ .TSVar }}Schema } from "./{{ .TSFileBase }}.schema.gen";
import type { {{ .TSFacade.EntityType }}, {{ .TSFacade.CreateType }}{{ if .TSFacade.HasFilters }}, {{ .TSFacade.ListRequestType }}{{ end }} } from "{{ .TSFacade.EntityModule }}";
import type {
  ResourceListQuery,
  ResourceListResult,
  ResourceOperations,
  ResourceUpdateSubmission,
} from "{{ .TSOperationsImport }}";
import type { SortableFieldName } from "{{ .TSSchemaImport }}";
{{- end }}

export type {{ .Name }}UpdateValues = {
{{- range .TSUpdate.Fields }}
  {{ .Name }}: {{ .Type }};
{{- end }}
};

export type {{ .Name }}UpdateTouched = ReadonlySet<keyof {{ .Name }}UpdateValues>;

// {{ .TSVar }}BuildUpdateRequest sends the identifier plus exactly the touched,
// ordinary-updatable fields. Explicit empty strings, 0, and false are retained.
export function {{ .TSVar }}BuildUpdateRequest(
  identifier: {{ .TSUpdate.IdentifierType }},
  values: {{ .Name }}UpdateValues,
  touched: {{ .Name }}UpdateTouched,
): Parameters<{{ .Name }}Raw["{{ .TSUpdate.Method }}"]>[0] {
  const request: Parameters<{{ .Name }}Raw["{{ .TSUpdate.Method }}"]>[0] = {
    {{ .TSUpdate.IdentifierField }}: identifier,
  };
{{- range .TSUpdate.Fields }}
  if (touched.has("{{ .Name }}")) {
    request.{{ .Name }} = values.{{ .Name }};
  }
{{- end }}
  return request;
}
{{- if .TSFacade }}

// Create values are the protobuf initializer accepted by the raw operation,
// not the full protobuf Message type. This keeps generated UI forms honest:
// callers must provide business fields, but never fabricate $typeName or other
// runtime message metadata.
export type {{ .Name }}CreateValues = Parameters<{{ .Name }}Raw["{{ .TSFacade.CreateMethod }}"]>[0];

export type {{ .Name }}Filters = {{ .TSFacade.FilterType }};

export type {{ .Name }}OrderField = {{ .TSFacade.OrderFieldType }};

export function create{{ .Name }}Operations(raw: {{ .Name }}Raw) {
  async function {{ .TSVar }}Update(
    submission: ResourceUpdateSubmission<{{ .TSFacade.KeyType }}, {{ .TSFacade.UpdateValuesName }}>,
  ): Promise<{{ .TSFacade.EntityType }}>;
  async function {{ .TSVar }}Update(
    identifier: {{ .TSFacade.KeyType }},
    values: {{ .TSFacade.UpdateValuesName }},
    touched: {{ .TSFacade.UpdateTouchedName }},
  ): Promise<{{ .TSFacade.EntityType }}>;
  async function {{ .TSVar }}Update(
    submissionOrIdentifier: ResourceUpdateSubmission<{{ .TSFacade.KeyType }}, {{ .TSFacade.UpdateValuesName }}> | {{ .TSFacade.KeyType }},
    values?: {{ .TSFacade.UpdateValuesName }},
    touched?: {{ .TSFacade.UpdateTouchedName }},
  ): Promise<{{ .TSFacade.EntityType }}> {
    let submission: ResourceUpdateSubmission<{{ .TSFacade.KeyType }}, {{ .TSFacade.UpdateValuesName }}>;
    if (typeof submissionOrIdentifier === "bigint") {
      if (values === undefined || touched === undefined) {
        throw new TypeError("{{ .TSVar }}Update requires values and touched when called with a key");
      }
      submission = { key: submissionOrIdentifier, values, touched, dirty: touched };
    } else {
      submission = submissionOrIdentifier;
    }
    const response = await raw.{{ .TSFacade.UpdateMethod }}(
      {{ .TSVar }}BuildUpdateRequest(submission.key, submission.values, submission.touched),
    );
    if (response.{{ .TSFacade.UpdateItemField }} === undefined) {
      throw new Error("{{ .Name }} Update response is missing item");
    }
    return response.{{ .TSFacade.UpdateItemField }};
  }

  const {{ .TSVar }}OperationsBase = {
    raw,
{{- if .TSCustomOperations }}
    custom: {
{{- range .TSCustomOperations }}
      {{ .Method }}(request: Parameters<{{ $.Name }}Raw["{{ .Method }}"]>[0]) {
        return raw.{{ .Method }}(request);
      },
{{- end }}
    },
{{- end }}
    buildUpdateRequest: {{ .TSVar }}BuildUpdateRequest,
    keyOf(entity: Readonly<{{ .TSFacade.EntityType }}>): {{ .TSFacade.KeyType }} {
      return entity.{{ .TSFacade.KeyJSONName }};
    },
    async list(
      query: ResourceListQuery<{{ .Name }}Filters, {{ .Name }}OrderField>,
    ): Promise<ResourceListResult<{{ .TSFacade.EntityType }}>> {
      const request: Parameters<{{ .Name }}Raw["{{ .TSFacade.ListMethod }}"]>[0] = {
        {{ .TSFacade.PageField }}: { pageNum: query.pageNum, pageSize: query.pageSize },
{{- if .TSFacade.HasKeyword }}
{{- if .TSFacade.KeywordOptional }}
        {{ .TSFacade.KeywordField }}: query.keyword,
{{- else }}
        {{ .TSFacade.KeywordField }}: query.keyword ?? "",
{{- end }}
{{- end }}
{{- if .TSFacade.HasSort }}
        {{ .TSFacade.SortField }}: query.orders?.map((order) => ({ field: order.field, desc: order.desc })) ?? [],
{{- end }}
      };
{{- if .TSFacade.HasFilters }}
      const filters = query.filters;
{{- end }}
{{- range .TSFacade.Filters }}
      if (filters !== undefined && filters.{{ .RequestJSONName }} !== undefined) {
        request.{{ .RequestJSONName }} = filters.{{ .RequestJSONName }};
      }
{{- end }}
      const response = await raw.{{ .TSFacade.ListMethod }}(request);
      const page = response.{{ .TSFacade.PageInfoField }};
      if (page === undefined) {
        throw new Error("{{ .Name }} List response is missing PageInfo");
      }
      return {
        {{ .TSFacade.ItemsField }}: response.{{ .TSFacade.ItemsField }},
        total: page.total,
        pageNum: query.pageNum,
        pageSize: query.pageSize,
      };
    },
    async get(key: {{ .TSFacade.KeyType }}): Promise<{{ .TSFacade.EntityType }}> {
      const response = await raw.{{ .TSFacade.GetMethod }}({ {{ .TSFacade.IdentifierField }}: key });
      if (response.{{ .TSFacade.GetItemField }} === undefined) {
        throw new Error("{{ .Name }} Get response is missing item");
      }
      return response.{{ .TSFacade.GetItemField }};
    },
    async create(values: Readonly<{{ .Name }}CreateValues>): Promise<{{ .TSFacade.EntityType }}> {
      const response = await raw.{{ .TSFacade.CreateMethod }}(values);
      if (response.{{ .TSFacade.CreateItemField }} === undefined) {
        throw new Error("{{ .Name }} Create response is missing item");
      }
      return response.{{ .TSFacade.CreateItemField }};
    },
    update: {{ .TSVar }}Update,
    async delete(key: {{ .TSFacade.KeyType }}): Promise<void> {
      await raw.{{ .TSFacade.DeleteMethod }}({ {{ .TSFacade.IdentifierField }}: key });
    },
    async batchDelete(keys: readonly {{ .TSFacade.KeyType }}[]): Promise<void> {
      await raw.{{ .TSFacade.BatchDeleteMethod }}({ {{ .TSFacade.BatchIDsField }}: [...keys] });
    },
  } as const;

  return {{ .TSVar }}OperationsBase satisfies ResourceOperations<
    {{ .TSFacade.EntityType }},
    {{ .TSFacade.KeyType }},
    {{ .Name }}Filters,
    {{ .Name }}CreateValues,
    {{ .TSFacade.UpdateValuesName }},
    {{ .TSFacade.EntityType }},
    void,
    {{ .Name }}OrderField
  >;
}
export type {{ .Name }}Operations = ReturnType<typeof create{{ .Name }}Operations>;
{{- else }}

export function create{{ .Name }}LegacyOperations(raw: {{ .Name }}Raw) {
  return {
    raw,
    buildUpdateRequest: {{ .TSVar }}BuildUpdateRequest,
    update(
      identifier: {{ .TSUpdate.IdentifierType }},
      values: {{ .Name }}UpdateValues,
      touched: {{ .Name }}UpdateTouched,
    ) {
      return raw.{{ .TSUpdate.Method }}({{ .TSVar }}BuildUpdateRequest(identifier, values, touched));
    },
  } as const;
}
export type {{ .Name }}LegacyOperations = ReturnType<typeof create{{ .Name }}LegacyOperations>;
{{- end }}
`))

var tsReadOperationsTpl = template.Must(template.New("ts-read-operations").Parse(`{{ .Header }}
import type { {{ .Name }}Raw } from "./{{ .TSFileBase }}.raw.gen";
import { {{ .TSVar }}Schema } from "./{{ .TSFileBase }}.schema.gen";
import type { {{ .TSReadFacade.EntityType }}{{ if .TSReadFacade.HasFilters }}, {{ .TSReadFacade.ListRequestType }}{{ end }} } from "{{ .TSReadFacade.EntityModule }}";
import type {
  ResourceListQuery,
  ResourceListResult,
  ResourceReadOperations,
} from "{{ .TSOperationsImport }}";
import type { SortableFieldName } from "{{ .TSSchemaImport }}";

{{- if .TSReadFacade.CreateMethod }}
export type {{ .Name }}CreateValues = Pick<
  Parameters<{{ .Name }}Raw["{{ .TSReadFacade.CreateMethod }}"]>[0],
  {{ .TSReadFacade.CreateFields }}
>;
{{- end }}
{{- if .TSReadFacade.Update }}
export type {{ .Name }}UpdateValues = {
{{- range .TSReadFacade.Update.Fields }}
  {{ .Name }}: {{ .Type }};
{{- end }}
};

export type {{ .Name }}UpdateTouched = ReadonlySet<keyof {{ .Name }}UpdateValues>;

export function {{ .TSVar }}BuildUpdateRequest(
  identifier: {{ .TSReadFacade.Update.IdentifierType }},
  values: {{ .Name }}UpdateValues,
  touched: {{ .Name }}UpdateTouched,
): Parameters<{{ .Name }}Raw["{{ .TSReadFacade.Update.Method }}"]>[0] {
  const request: Parameters<{{ .Name }}Raw["{{ .TSReadFacade.Update.Method }}"]>[0] = {
    {{ .TSReadFacade.Update.IdentifierField }}: identifier,
  };
{{- range .TSReadFacade.Update.Fields }}
  if (touched.has("{{ .Name }}")) {
    request.{{ .Name }} = values.{{ .Name }};
  }
{{- end }}
  return request;
}
{{- end }}

export type {{ .Name }}Filters = {{ .TSReadFacade.FilterType }};
export type {{ .Name }}OrderField = {{ .TSReadFacade.OrderFieldType }};

export function create{{ .Name }}Operations(raw: {{ .Name }}Raw) {
  const {{ .TSVar }}OperationsBase = {
    raw,
{{- if .TSCustomOperations }}
    custom: {
{{- range .TSCustomOperations }}
      {{ .Method }}(request: Parameters<{{ $.Name }}Raw["{{ .Method }}"]>[0]) {
        return raw.{{ .Method }}(request);
      },
{{- end }}
    },
{{- end }}
{{- if .TSReadFacade.Update }}
    buildUpdateRequest: {{ .TSVar }}BuildUpdateRequest,
{{- end }}
    keyOf(entity: Readonly<{{ .TSReadFacade.EntityType }}>): {{ .TSReadFacade.KeyType }} {
      return entity.{{ .TSReadFacade.KeyJSONName }};
    },
    async list(
      query: ResourceListQuery<{{ .Name }}Filters, {{ .Name }}OrderField>,
    ): Promise<ResourceListResult<{{ .TSReadFacade.EntityType }}>> {
      const request: Parameters<{{ .Name }}Raw["{{ .TSReadFacade.ListMethod }}"]>[0] = {
        {{ .TSReadFacade.PageField }}: { pageNum: query.pageNum, pageSize: query.pageSize },
{{- if .TSReadFacade.HasKeyword }}
{{- if .TSReadFacade.KeywordOptional }}
        {{ .TSReadFacade.KeywordField }}: query.keyword,
{{- else }}
        {{ .TSReadFacade.KeywordField }}: query.keyword ?? "",
{{- end }}
{{- end }}
{{- if .TSReadFacade.HasSort }}
        {{ .TSReadFacade.SortField }}: query.orders?.map((order) => ({ field: order.field, desc: order.desc })) ?? [],
{{- end }}
      };
{{- if .TSReadFacade.HasFilters }}
      const filters = query.filters;
{{- end }}
{{- range .TSReadFacade.Filters }}
      if (filters !== undefined && filters.{{ .RequestJSONName }} !== undefined) {
        request.{{ .RequestJSONName }} = filters.{{ .RequestJSONName }};
      }
{{- end }}
      const response = await raw.{{ .TSReadFacade.ListMethod }}(request);
      const page = response.{{ .TSReadFacade.PageInfoField }};
      if (page === undefined) {
        throw new Error("{{ .Name }} List response is missing PageInfo");
      }
      return {
        {{ .TSReadFacade.ItemsField }}: response.{{ .TSReadFacade.ItemsField }},
        total: page.total,
        pageNum: query.pageNum,
        pageSize: query.pageSize,
      };
    },
  } as const;
  return {{ .TSVar }}OperationsBase satisfies ResourceReadOperations<
    {{ .TSReadFacade.EntityType }},
    {{ .TSReadFacade.KeyType }},
    {{ .Name }}Filters,
    {{ .Name }}OrderField
  >;
}
export type {{ .Name }}Operations = ReturnType<typeof create{{ .Name }}Operations>;
`))

var tsOperationsRegistryTpl = template.Must(template.New("ts-operations-registry").Parse(`{{ .Header }}
import type { Transport } from "@connectrpc/connect";
{{- range .Entries }}
import { create{{ .Name }}Raw } from "./{{ .TSFileBase }}.raw.gen";
{{- end }}
{{- range .Facades }}
import { create{{ .Name }}Operations } from "./{{ .TSFileBase }}.operations.gen";
{{- end }}

export function createResourceOperations(transport: Transport) {
  {{- range .Entries }}
  const {{ .TSVar }}Raw = create{{ .Name }}Raw(transport);
  {{- end }}
  return {
    raw: {
{{- range .Entries }}
      {{ .Name }}: {{ .TSVar }}Raw,
{{- end }}
    },
    operations: {
{{- range .Facades }}
      {{ .Name }}: create{{ .Name }}Operations({{ .TSVar }}Raw),
{{- end }}
    },
  };
}
export type ResourceOperationSet = ReturnType<typeof createResourceOperations>;
`))

var tsRelationsTpl = template.Must(template.New("ts-relations").Parse(`{{ .Header }}
import { createRelationOptionLoader, type RelationOptionLoader } from "{{ .TSRelationImport }}";
{{- range .TSRelations }}
import type { {{ .TargetResource }}Raw } from "./{{ .TargetTSFileBase }}.raw.gen";
{{- end }}

export function create{{ .Name }}RelationLoaders(targets: {
{{- range .TSRelations }}
  {{ .TargetResource }}: Pick<{{ .TargetResource }}Raw, "{{ .TargetListMethod }}">;
{{- end }}
}) {
  return {
{{- range .TSRelations }}
    {{ .FieldName }}: createRelationOptionLoader<{{ .ValueType }}>({
      remoteSearchEnabled: {{ .RemoteSearchEnabled }},
      async loadPage(input) {
        const response = await targets.{{ .TargetResource }}.{{ .TargetListMethod }}({
          {{ .PageField }}: { pageNum: input.pageNum, pageSize: input.pageSize },
{{- if .RemoteSearchEnabled }}
          {{ .KeywordField }}: input.keyword ?? "",
{{- end }}
        });
        const page = response.{{ .PageInfoField }};
        if (page === undefined) {
          throw new Error("{{ $.Name }}.{{ .FieldName }} relation List response is missing PageInfo");
        }
        const options = response.{{ .ItemsField }}.map((item) => ({
          value: item.{{ .ValueField }},
          label: item.{{ .LabelField }},
        }));
        const nextPage = BigInt(page.pageNum) * BigInt(page.pageSize) < page.total
          ? page.pageNum + 1
          : undefined;
        return nextPage === undefined ? { options } : { options, nextPage };
      },
    }) satisfies RelationOptionLoader<{{ .ValueType }}>,
{{- end }}
  } as const;
}
export type {{ .Name }}RelationLoaders = ReturnType<typeof create{{ .Name }}RelationLoaders>;
`))

var tsSchemaTpl = template.Must(template.New("ts").Parse(`{{ .Header }}
import type { ResourceSchema } from "{{ .TSSchemaImport }}";

/** {{ .Name }} 资源 schema（后端 resource.Schema 的 TS 投影） */
export const {{ .TSVar }}Schema = {
  name: {{ printf "%q" .Name }},
  permPrefix: {{ printf "%q" .PermPrefix }},
  table: {{ printf "%q" .Table }},
  permissions: {
{{ if .TSPermissions.List }}    list: {{ printf "%q" .TSPermissions.List }},
{{ end }}{{ if .TSPermissions.Get }}    get: {{ printf "%q" .TSPermissions.Get }},
{{ end }}{{ if .TSPermissions.Create }}    create: {{ printf "%q" .TSPermissions.Create }},
{{ end }}{{ if .TSPermissions.Update }}    update: {{ printf "%q" .TSPermissions.Update }},
{{ end }}{{ if .TSPermissions.Delete }}    delete: {{ printf "%q" .TSPermissions.Delete }},
{{ end }}{{ if .TSPermissions.BatchDelete }}    batchDelete: {{ printf "%q" .TSPermissions.BatchDelete }},
{{ end }}  },
  fields: [
{{- range .Fields }}
    {
      name: {{ printf "%q" .TSName }},
{{- if .Label }}
      label: {{ printf "%q" .Label }},
{{- end }}
      protoName: {{ printf "%q" .ProtoName }},
      tsType: {{ printf "%q" .TSType }},
      required: {{ .Required }},
      searchable: {{ .Searchable }},
      filterable: {{ .Filterable }},
      sortable: {{ .Sortable }},
      listable: {{ .Listable }},
      maxLen: {{ .MaxLen }},
      creatable: {{ .InCreate }},
      updatable: {{ .InUpdate }},
      serverManaged: {{ .ServerManaged }},
{{- if .CreateOptional }}
      optional: true,
{{- end }}
{{- if .HasRelation }}
      relation: {
        kind: "resource",
        target: {{ printf "%q" .RelTarget }},
        valueField: {{ printf "%q" .RelValueField }},
        labelField: {{ printf "%q" .RelLabelField }},
      },
{{- end }}
    },
{{- end }}
  ],
} as const satisfies ResourceSchema;

export type {{ .Name }}Schema = typeof {{ .TSVar }}Schema;
`))
