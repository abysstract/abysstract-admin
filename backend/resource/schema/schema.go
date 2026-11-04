// Package schema defines the static, AI-native Resource schema DSL.
//
// Responsibility is deliberately split:
//   - protobuf owns the API wire contract: RPCs, messages, field numbers,
//     presence, and enums;
//   - this package owns resource behavior: CRUD capabilities, permissions,
//     display, search, sorting, relations, and field lifecycle;
//   - Hook implementations are ordinary Go functions injected into generated
//     controllers. Hooks never live in Schema and the generator never executes
//     Hook initialization code.
//
// Resource and Field values are static metadata. They must not retain database
// handles, caches, loggers, request state, or user configuration.
package schema

import "google.golang.org/protobuf/reflect/protoreflect"

// Mode is the resource generation mode.
type Mode uint8

const (
	// Generated uses generated implementations for the standard CRUD operations.
	Generated Mode = iota
	// Partial adds custom RPCs implemented by a strongly typed Overrides interface.
	Partial
)

// RepositoryImplementation selects the complete repository implementation.
type RepositoryImplementation uint8

const (
	RepositoryGenerated RepositoryImplementation = iota
	RepositoryInjected
)

// RepositoryConfig is static repository behavior metadata.
type RepositoryConfig struct {
	Implementation RepositoryImplementation
}

// OperationImplementation selects an operation implementation. It is
// intentionally a different Go type from RepositoryImplementation.
type OperationImplementation uint8

const (
	OperationGenerated OperationImplementation = iota
	OperationOverride
)

// OperationKind identifies the six standard operations and custom RPCs.
type OperationKind uint8

const (
	OperationList OperationKind = iota
	OperationCreate
	OperationGet
	OperationUpdate
	OperationDelete
	OperationBatchDelete
	OperationCustom
)

// LegacyPermission identifies the small, closed set of compatibility
// exceptions for procedure permissions that predate Resource generation.
// Standard operation permissions remain fixed by OperationKind unless a
// registered legacy exception is selected and validated by Build.
type LegacyPermission uint8

const (
	LegacyPermissionNone LegacyPermission = iota
	LegacyPermissionGetQuery
)

// ScopePolicy is the authorization policy for a custom operation.
type ScopePolicy uint8

const (
	ScopePolicyUnspecified ScopePolicy = iota
	ScopePolicyExempt
)

// ServiceConfig references both the protobuf service descriptor and its
// generated Connect handler interface. Handler is conventionally a typed nil
// pointer such as (*adminv1connect.DictServiceHandler)(nil).
type ServiceConfig struct {
	Descriptor protoreflect.ServiceDescriptor
	Handler    any
}

// OperationBinding binds request/response fields to semantic roles. Field
// names are generated Go field names, not guessed SQL or JSON identifiers.
type OperationBinding struct {
	Identifier string
	BatchIDs   string
	Item       string
	List       ListBinding
}

// ListDefaultOrder is a validated resource-field default order used when a
// List request does not provide an explicit order. Field names use Schema/
// Model naming and are resolved to DB columns during Build.
type ListDefaultOrder struct {
	Field string
	Desc  bool
}

// ListBinding binds the fixed v1 standard-list roles.
type ListBinding struct {
	Page          string
	Keyword       string
	Sort          string
	Filters       map[string]string // Schema/Model field name -> List request field name.
	DefaultOrders []ListDefaultOrder
	Items         string
	PageInfo      string
}

// Operation is complete static RPC metadata.
type Operation struct {
	Kind           OperationKind
	Method         string
	Request        any
	Response       any
	Binding        OperationBinding
	Implementation OperationImplementation
	Action         string
	// LegacyPermission opts into a validated compatibility exception. It is
	// not a general permission override; Build accepts only the registered
	// legacy procedure mappings.
	LegacyPermission LegacyPermission
	ScopePolicy      ScopePolicy

	bindingDefaults operationBindingDefaults
}

type operationBindingDefaults struct {
	identifier bool
	batchIDs   bool
}

// StandardOperation contains the caller-supplied part of one standard RPC.
type StandardOperation struct {
	Method           string
	Request          any
	Response         any
	Binding          OperationBinding
	LegacyPermission LegacyPermission
}

// StandardOperationsConfig describes all six standard RPCs.
type StandardOperationsConfig struct {
	List        StandardOperation
	Create      StandardOperation
	Get         StandardOperation
	Update      StandardOperation
	Delete      StandardOperation
	BatchDelete StandardOperation
}

// StandardOperations expands the standard-six shorthand into complete
// Operation metadata with fixed generated implementation discriminants and
// conventional role bindings. Callers may override any conventional binding.
func StandardOperations(c StandardOperationsConfig) []Operation {
	defaults := []struct {
		kind    OperationKind
		spec    StandardOperation
		binding OperationBinding
	}{
		{OperationList, c.List, OperationBinding{List: ListBinding{Page: "Page", Items: "Items", PageInfo: "Page"}}},
		{OperationCreate, c.Create, OperationBinding{Item: "Item"}},
		{OperationGet, c.Get, OperationBinding{Identifier: "Id", Item: "Item"}},
		{OperationUpdate, c.Update, OperationBinding{Identifier: "Id", Item: "Item"}},
		{OperationDelete, c.Delete, OperationBinding{Identifier: "Id"}},
		{OperationBatchDelete, c.BatchDelete, OperationBinding{BatchIDs: "Ids"}},
	}
	out := make([]Operation, 0, len(defaults))
	for _, d := range defaults {
		binding := mergeBinding(d.binding, d.spec.Binding)
		out = append(out, Operation{
			Kind:             d.kind,
			Method:           d.spec.Method,
			Request:          d.spec.Request,
			Response:         d.spec.Response,
			Binding:          binding,
			Implementation:   OperationGenerated,
			LegacyPermission: d.spec.LegacyPermission,
			bindingDefaults: operationBindingDefaults{
				identifier: (d.kind == OperationGet || d.kind == OperationUpdate || d.kind == OperationDelete) && d.spec.Binding.Identifier == "",
				batchIDs:   d.kind == OperationBatchDelete && d.spec.Binding.BatchIDs == "",
			},
		})
	}
	return out
}

func mergeBinding(base, override OperationBinding) OperationBinding {
	if override.Identifier != "" {
		base.Identifier = override.Identifier
	}
	if override.BatchIDs != "" {
		base.BatchIDs = override.BatchIDs
	}
	if override.Item != "" {
		base.Item = override.Item
	}
	if override.List.Page != "" {
		base.List.Page = override.List.Page
	}
	if override.List.Keyword != "" {
		base.List.Keyword = override.List.Keyword
	}
	if override.List.Sort != "" {
		base.List.Sort = override.List.Sort
	}
	if override.List.DefaultOrders != nil {
		base.List.DefaultOrders = append([]ListDefaultOrder(nil), override.List.DefaultOrders...)
	}
	if override.List.Filters != nil {
		base.List.Filters = cloneStringMap(override.List.Filters)
	}
	if override.List.Items != "" {
		base.List.Items = override.List.Items
	}
	if override.List.PageInfo != "" {
		base.List.PageInfo = override.List.PageInfo
	}
	return base
}

// KeyConfig explicitly identifies the v1 int64 primary key by Model field.
type KeyConfig struct {
	ModelField string
}

// AuditConfig explicitly identifies server-managed audit fields.
type AuditConfig struct {
	UpdatedAtField string
}

// AuthConfig is static authorization metadata. DeptField and CreatorField are
// Model Go field names; Build resolves them to validated GORM DB names.
type AuthConfig struct {
	DataScope    bool
	DeptField    string
	CreatorField string
	// SkipCreateScopeCheck bypasses the in-transaction create authorization
	// check. It is security-sensitive, requires DataScope=true, and should be
	// used only where the caller explicitly accepts that creation risk.
	SkipCreateScopeCheck bool
}

// Lifecycle is the three-flag client field lifecycle.
type Lifecycle struct {
	Creatable     *bool
	Updatable     *bool
	ServerManaged bool
}

func (l Lifecycle) EffCreatable() bool {
	if l.ServerManaged {
		return false
	}
	return l.Creatable == nil || *l.Creatable
}

func (l Lifecycle) EffUpdatable() bool {
	if l.ServerManaged {
		return false
	}
	return l.Updatable == nil || *l.Updatable
}

// Relation is a lazy static reference, allowing cyclic Go declarations.
type Relation struct {
	Target     func() *Resource
	RefKey     string
	LabelField string

	// Resolved by BuildRegistry for generator consumption.
	RefJSONName         string
	LabelJSONName       string
	RemoteSearchEnabled bool
}

// Field describes one Model field's resource behavior.
type Field struct {
	Name      string
	Label     string
	ProtoName string

	Required   bool
	Searchable bool
	Filterable bool
	Sortable   bool
	Listable   bool
	MaxLen     int
	MaxLenSet  bool
	Identifier bool

	EnumMap map[string]int16

	Lifecycle    Lifecycle
	Relation     *Relation
	ReadOnlyFlag bool
}

// Resource is a complete static resource declaration.
type Resource struct {
	Name       string
	PermPrefix string
	Mode       Mode
	Table      string

	Model     any
	Entity    any
	CreateReq any
	UpdateReq any

	Fields       []Field
	Ignore       []string
	EntityIgnore []string

	Service    ServiceConfig
	Operations []Operation
	Repository RepositoryConfig
	Auth       AuthConfig
	Key        KeyConfig
	Audit      AuditConfig
}

// FieldOpt configures resource field behavior and shared presentation metadata.
type FieldOpt func(*Field)

func Label(label string) FieldOpt { return func(f *Field) { f.Label = label } }

func Required() FieldOpt          { return func(f *Field) { f.Required = true } }
func MaxLen(n int) FieldOpt       { return func(f *Field) { f.MaxLen, f.MaxLenSet = n, true } }
func Searchable() FieldOpt        { return func(f *Field) { f.Searchable = true } }
func Filterable() FieldOpt        { return func(f *Field) { f.Filterable = true } }
func SortableOpt() FieldOpt       { return func(f *Field) { f.Sortable = true } }
func NotListable() FieldOpt       { return func(f *Field) { f.Listable = false } }
func ProtoName(n string) FieldOpt { return func(f *Field) { f.ProtoName = n } }
func ServerManaged() FieldOpt {
	return func(f *Field) {
		no := false
		f.Lifecycle.Creatable = &no
		f.Lifecycle.Updatable = &no
		f.Lifecycle.ServerManaged = true
	}
}
func CreateOnly() FieldOpt {
	return func(f *Field) {
		no := false
		f.Lifecycle.Updatable = &no
	}
}
func NotCreatable() FieldOpt {
	return func(f *Field) {
		no := false
		f.Lifecycle.Creatable = &no
	}
}
func ReadOnly() FieldOpt {
	return func(f *Field) {
		no := false
		f.Lifecycle.Creatable = &no
		f.Lifecycle.Updatable = &no
		f.ReadOnlyFlag = true
	}
}
func EnumMap(m map[string]int16) FieldOpt {
	return func(f *Field) {
		f.EnumMap = make(map[string]int16, len(m))
		for k, v := range m {
			f.EnumMap[k] = v
		}
	}
}
func RelationTo(target func() *Resource, refKey, labelField string) FieldOpt {
	return func(f *Field) {
		f.Relation = &Relation{Target: target, RefKey: refKey, LabelField: labelField}
	}
}

// F constructs a Field. Listable defaults to true.
func F(name string, opts ...FieldOpt) Field {
	f := Field{Name: name, Listable: true}
	for _, opt := range opts {
		opt(&f)
	}
	return f
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Identifier marks an exact program key whose input has surrounding whitespace removed.
func Identifier() FieldOpt { return func(f *Field) { f.Identifier = true } }
