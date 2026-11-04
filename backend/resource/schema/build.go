package schema

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	gormschema "gorm.io/gorm/schema"
)

// ConversionKind is the closed v1 conversion matrix consumed by generators.
type ConversionKind uint8

const (
	ConvDirect ConversionKind = iota
	ConvNarrow
	ConvEnum
	ConvTime
)

// ConversionPlan is one independently-derived conversion direction.
type ConversionPlan struct {
	Kind          ConversionKind
	SourceType    reflect.Type
	TargetType    reflect.Type
	NeedsBounds   bool
	EnumTypeName  string
	ModelTypeName string
}

// FieldPlan is the unified, validated generation IR for one field.
type FieldPlan struct {
	ModelField  string
	ModelType   reflect.Type
	ProtoName   string
	ProtoGoName string
	JSONName    string
	DBName      string
	TSType      string
	EntityType  reflect.Type

	InCreate           bool
	InUpdate           bool
	CreateOnlyInUpdate bool
	UpdateOptional     bool
	CreateOptional     bool

	ModelToEntity ConversionPlan
	CreateToModel ConversionPlan
	UpdateToModel ConversionPlan

	Decl     Field
	Relation *RelationLoadPlan
}

// KeyPlan is the validated v1 single int64 primary key.
type KeyPlan struct {
	ModelField string
	ProtoName  string
	JSONName   string
	DBName     string
	TSType     string
}

// FilterPlan converts one presence-aware List filter before SQL is built.
type FilterPlan struct {
	SchemaField     string
	RequestField    string
	RequestJSONName string
	DBName          string
	ConversionPlan  ConversionPlan
}

// DefaultOrderPlan is a resolved Schema default order for generated List.
type DefaultOrderPlan struct {
	SchemaField string
	DBName      string
	Desc        bool
}

// ListPlan is the validated standard paginated List IR.
type ListPlan struct {
	PageRequestField string
	KeywordField     string
	SortField        string
	ItemsField       string
	PageInfoField    string
	SearchDBNames    []string
	SortableDBNames  map[string]string // protobuf JSON name -> DBName
	Filters          []FilterPlan
	DefaultOrders    []DefaultOrderPlan
}

// OperationPlan is one validated RPC operation.
type OperationPlan struct {
	Operation
	RequestType         reflect.Type
	ResponseType        reflect.Type
	RequestProtoModule  string
	ResponseProtoModule string
	List                *ListPlan
}

// RelationLoadPlan is populated by BuildRegistry after target List validation.
type RelationLoadPlan struct {
	TargetResource      string
	RefJSONName         string
	LabelJSONName       string
	RemoteSearchEnabled bool
}

// Plan is the complete validated generation IR for one Resource.
type Plan struct {
	Res        *Resource
	ModelType  reflect.Type
	EntityType reflect.Type
	CreateType reflect.Type
	UpdateType reflect.Type
	Fields     []*FieldPlan
	Operations []*OperationPlan
	Key        KeyPlan

	UpdatedAtModelField string
	UpdatedAtDBName     string
	DeptDBName          string
	CreatorDBName       string
}

func (p *Plan) Field(modelField string) *FieldPlan {
	for _, field := range p.Fields {
		if field.ModelField == modelField {
			return field
		}
	}
	return nil
}

func (p *Plan) Operation(kind OperationKind) *OperationPlan {
	for _, operation := range p.Operations {
		if operation.Kind == kind {
			return operation
		}
	}
	return nil
}

var (
	typeTime      = reflect.TypeOf(time.Time{})
	typeTimestamp = reflect.TypeOf((*timestamppb.Timestamp)(nil))
	typeContext   = reflect.TypeOf((*context.Context)(nil)).Elem()
	typeError     = reflect.TypeOf((*error)(nil)).Elem()
)

// Build validates Model/protobuf/Schema consistency and derives a Plan. It
// aggregates all independently discoverable errors and never writes files.
func (r *Resource) Build() (*Plan, error) {
	var errors []string
	fail := func(format string, args ...any) {
		errors = append(errors, fmt.Sprintf(format, args...))
	}

	if r == nil {
		return nil, &BuildError{Resource: "<nil>", Errors: []string{"Resource is nil"}}
	}
	if r.Name == "" {
		fail("Name is empty")
	}
	if r.PermPrefix == "" || !strings.Contains(r.PermPrefix, ":") {
		fail("PermPrefix %q must contain ':'", r.PermPrefix)
	}
	if r.Table == "" {
		fail("Table is empty")
	}

	modelType, modelFields, modelErr := InspectStruct(r.Model)
	if modelErr != nil {
		fail("Model: %v", modelErr)
	}
	entityType, entityFields, entityErr := InspectStruct(r.Entity)
	if entityErr != nil {
		fail("Entity: %v", entityErr)
	}
	createType, createFields, createErr := InspectStruct(r.CreateReq)
	if createErr != nil {
		fail("CreateReq: %v", createErr)
	}
	updateType, updateFields, updateErr := InspectStruct(r.UpdateReq)
	if updateErr != nil {
		fail("UpdateReq: %v", updateErr)
	}
	if modelErr != nil || entityErr != nil || createErr != nil || updateErr != nil {
		return nil, &BuildError{Resource: r.Name, Errors: errors}
	}

	plan := &Plan{Res: r, ModelType: modelType, EntityType: entityType, CreateType: createType, UpdateType: updateType}
	modelByName := indexByName(modelFields)
	entityByProto := indexByProtoName(entityFields)
	createByProto := indexByProtoName(createFields)
	updateByProto := indexByProtoName(updateFields)

	gormModel, err := gormschema.Parse(reflect.New(modelType).Interface(), &sync.Map{}, gormschema.NamingStrategy{})
	if err != nil {
		fail("Model GORM schema: %v", err)
	}
	if gormModel != nil && gormModel.Table != r.Table {
		fail("Table declaration %q does not match model.TableName() %q", r.Table, gormModel.Table)
	}

	seenModelField := map[string]bool{}
	seenProtoName := map[string]string{}
	seenProtoGoName := map[string]string{}
	seenJSONName := map[string]string{}
	seenDBName := map[string]string{}
	for i := range r.Fields {
		declaration := r.Fields[i]
		if seenModelField[declaration.Name] {
			fail("field %q is declared more than once", declaration.Name)
			continue
		}
		seenModelField[declaration.Name] = true

		modelField, ok := modelByName[declaration.Name]
		if !ok {
			fail("field %q does not exist in model %s", declaration.Name, modelType.Name())
			continue
		}
		protoName := declaration.ProtoName
		if protoName == "" {
			protoName = protoFieldName(declaration.Name)
		}
		entityField, ok := entityByProto[protoName]
		if !ok {
			fail("field %q proto name %q does not exist in entity %s", declaration.Name, protoName, entityType.Name())
			continue
		}
		if previous := seenProtoName[protoName]; previous != "" {
			fail("fields %q and %q resolve to duplicate ProtoName %q", previous, declaration.Name, protoName)
		} else {
			seenProtoName[protoName] = declaration.Name
		}
		if previous := seenProtoGoName[entityField.Name]; previous != "" {
			fail("fields %q and %q resolve to duplicate ProtoGoName %q", previous, declaration.Name, entityField.Name)
		} else {
			seenProtoGoName[entityField.Name] = declaration.Name
		}
		if previous := seenJSONName[entityField.JSONName]; previous != "" {
			fail("fields %q and %q resolve to duplicate JSONName %q", previous, declaration.Name, entityField.JSONName)
		} else {
			seenJSONName[entityField.JSONName] = declaration.Name
		}

		dbName := ""
		if gormModel != nil {
			if gormField := gormModel.FieldsByName[declaration.Name]; gormField != nil {
				dbName = gormField.DBName
			}
		}
		if dbName == "" {
			fail("field %q has no GORM DBName", declaration.Name)
		} else if previous, duplicate := seenDBName[dbName]; duplicate {
			fail("fields %q and %q resolve to duplicate DBName %q", previous, declaration.Name, dbName)
		} else {
			seenDBName[dbName] = declaration.Name
		}

		validateFieldMetadata(declaration, modelField.Type, fail)
		if err := checkConvertible(declaration.Name, modelField.Type, entityField.Type); err != nil {
			fail("%v", err)
		}

		createField, inCreate := createByProto[protoName]
		updateField, inUpdate := updateByProto[protoName]
		validateLifecycle(declaration, r.Key.ModelField, inCreate, inUpdate, updateField, fail)

		if inCreate {
			requestType := requestValueType(createField, true)
			if err := checkConvertible(declaration.Name, modelField.Type, requestType); err != nil {
				fail("CreateRequest field %q: %v", declaration.Name, err)
			}
		}
		if inUpdate {
			requestType := requestValueType(updateField, true)
			if err := checkConvertible(declaration.Name, modelField.Type, requestType); err != nil {
				fail("UpdateRequest field %q: %v", declaration.Name, err)
			}
		}
		if err := checkEnumConsistency(declaration.Name, declaration.EnumMap, entityField.Type, requestValueType(createField, inCreate), requestValueType(updateField, inUpdate)); err != nil {
			fail("%v", err)
		}
		validateEnumMap(declaration, entityField.Type, createField, inCreate, updateField, inUpdate, fail)

		effectiveCreate := inCreate && declaration.Lifecycle.EffCreatable() && !declaration.ReadOnlyFlag
		effectiveUpdate := inUpdate && declaration.Lifecycle.EffUpdatable() && !declaration.ReadOnlyFlag
		createOnlyUpdate := inUpdate && !declaration.Lifecycle.EffUpdatable() && !declaration.Lifecycle.ServerManaged && !declaration.ReadOnlyFlag
		fieldPlan := &FieldPlan{
			ModelField: declaration.Name, ModelType: modelField.Type,
			ProtoName: protoName, ProtoGoName: entityField.Name, JSONName: entityField.JSONName, DBName: dbName, TSType: tsScalarType(entityField.Type),
			EntityType: entityField.Type, InCreate: effectiveCreate, InUpdate: effectiveUpdate,
			CreateOnlyInUpdate: createOnlyUpdate, UpdateOptional: inUpdate && updateField.Optional, CreateOptional: inCreate && createField.Optional,
			ModelToEntity: planConversion(modelField.Type, entityField.Type, enumIdentity(entityField.Type, &declaration)),
			Decl:          declaration,
		}
		if inCreate {
			requestType := requestValueType(createField, true)
			fieldPlan.CreateToModel = planConversion(requestType, modelField.Type, enumIdentity(requestType, &declaration))
		}
		if inUpdate {
			requestType := requestValueType(updateField, true)
			fieldPlan.UpdateToModel = planConversion(requestType, modelField.Type, enumIdentity(requestType, &declaration))
		}
		plan.Fields = append(plan.Fields, fieldPlan)
	}

	validateRequestCoverage(r, createByProto, updateByProto, fail)
	validateEntityCoverage(r, entityByProto, fail)
	validateLocalRelations(plan, fail)
	buildKeyPlan(r, plan, gormModel, fail)
	buildAuditPlan(r, plan, gormModel, fail)
	buildAuthPlan(r, plan, gormModel, fail)
	buildOperationPlans(r, plan, fail)
	validateOperationKeyBindings(plan, fail)

	if len(errors) > 0 {
		return nil, &BuildError{Resource: r.Name, Errors: errors}
	}
	return plan, nil
}

func validateFieldMetadata(field Field, modelType reflect.Type, fail func(string, ...any)) {
	if field.Required && field.Lifecycle.ServerManaged {
		fail("field %q: Required and ServerManaged are mutually exclusive", field.Name)
	}
	if field.Required && !field.Lifecycle.EffCreatable() {
		fail("field %q: Required implies creatable", field.Name)
	}
	if !field.Lifecycle.EffCreatable() && !field.Lifecycle.EffUpdatable() && !field.Lifecycle.ServerManaged && !field.ReadOnlyFlag {
		fail("field %q is never client writable; use ReadOnly() or a valid lifecycle", field.Name)
	}
	if field.Identifier && modelType.Kind() != reflect.String {
		fail("field %q Identifier requires string", field.Name)
	}
	if modelType.Kind() == reflect.Pointer {
		modelType = modelType.Elem()
	}
	if field.MaxLenSet {
		if modelType.Kind() != reflect.String {
			fail("field %q is not string; MaxLen cannot be generated", field.Name)
		}
		if field.MaxLen <= 0 {
			fail("field %q MaxLen must be positive, got %d", field.Name, field.MaxLen)
		}
	}
	if field.Required && modelType.Kind() != reflect.String {
		fail("field %q is not string; Required is unsupported because zero-value semantics are undefined", field.Name)
	}
	if field.Searchable && modelType.Kind() != reflect.String {
		fail("field %q is not string; Searchable is unsupported", field.Name)
	}
}

func validateLifecycle(field Field, keyModelField string, inCreate, inUpdate bool, updateField StructInfo, fail func(string, ...any)) {
	if field.Lifecycle.ServerManaged {
		if inCreate {
			fail("field %q is ServerManaged and forbidden in CreateRequest", field.Name)
		}
		if inUpdate && field.Name != keyModelField {
			fail("field %q is ServerManaged; only Key.ModelField %q may appear in UpdateRequest as Identifier", field.Name, keyModelField)
		}
		return
	}
	if field.ReadOnlyFlag {
		if inCreate || inUpdate {
			fail("field %q is ReadOnly and forbidden in Create/Update requests", field.Name)
		}
		return
	}
	if field.Lifecycle.EffCreatable() && !inCreate {
		fail("field %q creatable=true but is absent from CreateRequest", field.Name)
	}
	if !field.Lifecycle.EffCreatable() && inCreate {
		fail("field %q creatable=false but appears in CreateRequest", field.Name)
	}
	if field.Lifecycle.EffUpdatable() && !inUpdate {
		fail("field %q updatable=true but is absent from UpdateRequest", field.Name)
	}
	if inUpdate && !updateField.Optional {
		fail("field %q in UpdateRequest lacks protobuf presence (optional)", field.Name)
	}
}

func validateRequestCoverage(r *Resource, create, update map[string]StructInfo, fail func(string, ...any)) {
	ignored := make(map[string]bool, len(r.Ignore))
	for _, name := range r.Ignore {
		ignored[name] = true
	}
	consumed := make(map[string]bool, len(r.Fields))
	for _, field := range r.Fields {
		name := field.ProtoName
		if name == "" {
			name = protoFieldName(field.Name)
		}
		consumed[name] = true
	}
	for name := range create {
		if !consumed[name] && !ignored[name] {
			fail("CreateRequest field %q is not consumed by Schema or Ignore", name)
		}
	}
	for name := range update {
		if !consumed[name] && !ignored[name] {
			fail("UpdateRequest field %q is not consumed by Schema or Ignore", name)
		}
	}
}

func validateEntityCoverage(r *Resource, entity map[string]StructInfo, fail func(string, ...any)) {
	ignored := make(map[string]bool, len(r.EntityIgnore))
	for _, name := range r.EntityIgnore {
		ignored[name] = true
	}
	consumed := make(map[string]bool, len(r.Fields))
	for _, field := range r.Fields {
		name := field.ProtoName
		if name == "" {
			name = protoFieldName(field.Name)
		}
		consumed[name] = true
	}
	for name := range entity {
		if !consumed[name] && !ignored[name] {
			fail("Entity field %q is not declared by Schema or EntityIgnore", name)
		}
	}
}

func validateLocalRelations(plan *Plan, fail func(string, ...any)) {
	for _, sourceField := range plan.Fields {
		relation := sourceField.Decl.Relation
		if relation == nil {
			continue
		}
		if relation.Target == nil {
			fail("field %q relation Target is nil", sourceField.ModelField)
			continue
		}
		target := relation.Target()
		if target == nil {
			fail("field %q relation Target resolves to nil", sourceField.ModelField)
			continue
		}
		_, targetModelFields, modelErr := InspectStruct(target.Model)
		_, targetEntityFields, entityErr := InspectStruct(target.Entity)
		if modelErr != nil || entityErr != nil {
			fail("field %q relation target %s types are invalid: model=%v entity=%v", sourceField.ModelField, target.Name, modelErr, entityErr)
			continue
		}
		modelByName := indexByName(targetModelFields)
		entityByProto := indexByProtoName(targetEntityFields)
		declarations := make(map[string]Field, len(target.Fields))
		for _, field := range target.Fields {
			declarations[field.Name] = field
		}
		refModel, refModelOK := modelByName[relation.RefKey]
		refDecl, refDeclOK := declarations[relation.RefKey]
		if !refModelOK || !refDeclOK {
			fail("field %q relation target %s has no declared RefKey %q", sourceField.ModelField, target.Name, relation.RefKey)
			continue
		}
		labelModel, labelModelOK := modelByName[relation.LabelField]
		labelDecl, labelDeclOK := declarations[relation.LabelField]
		if !labelModelOK || !labelDeclOK {
			fail("field %q relation target %s has no declared LabelField %q", sourceField.ModelField, target.Name, relation.LabelField)
			continue
		}
		refProtoName := refDecl.ProtoName
		if refProtoName == "" {
			refProtoName = protoFieldName(refDecl.Name)
		}
		labelProtoName := labelDecl.ProtoName
		if labelProtoName == "" {
			labelProtoName = protoFieldName(labelDecl.Name)
		}
		refEntity, refEntityOK := entityByProto[refProtoName]
		if !refEntityOK {
			fail("field %q relation RefKey %q has no target Entity field", sourceField.ModelField, relation.RefKey)
			continue
		}
		labelEntity, labelEntityOK := entityByProto[labelProtoName]
		if !labelEntityOK {
			fail("field %q relation LabelField %q has no target Entity field", sourceField.ModelField, relation.LabelField)
			continue
		}
		if !validRelationRefType(refEntity.Type) {
			fail("field %q relation RefKey %q has unsupported protobuf type %v", sourceField.ModelField, relation.RefKey, refEntity.Type)
		}
		if labelEntity.Type.Kind() != reflect.String || labelEntity.Type.PkgPath() != "" {
			fail("field %q relation LabelField %q must be protobuf string, got %v", sourceField.ModelField, relation.LabelField, labelEntity.Type)
		}
		if sourceField.ModelType != refModel.Type || !sourceField.ModelType.AssignableTo(refModel.Type) || !refModel.Type.AssignableTo(sourceField.ModelType) {
			fail("field %q type %v is incompatible with relation RefKey %s.%s type %v", sourceField.ModelField, sourceField.ModelType, target.Name, relation.RefKey, refModel.Type)
		}
		_ = labelModel
		relation.RefJSONName = refEntity.JSONName
		relation.LabelJSONName = labelEntity.JSONName
	}
}

func buildKeyPlan(r *Resource, plan *Plan, modelSchema *gormschema.Schema, fail func(string, ...any)) {
	if r.Key.ModelField == "" {
		fail("Key.ModelField is required")
		return
	}
	field := plan.Field(r.Key.ModelField)
	if field == nil {
		fail("Key.ModelField %q does not exist in Schema", r.Key.ModelField)
		return
	}
	if field.ModelType != reflect.TypeOf(int64(0)) {
		fail("Key.ModelField %q must have exact int64 type", r.Key.ModelField)
	}
	if !field.Decl.Lifecycle.ServerManaged {
		fail("Key.ModelField %q must be ServerManaged", r.Key.ModelField)
	}
	if modelSchema == nil || modelSchema.FieldsByName[r.Key.ModelField] == nil || !modelSchema.FieldsByName[r.Key.ModelField].PrimaryKey {
		fail("Key.ModelField %q must be a GORM primary key", r.Key.ModelField)
	}
	if modelSchema != nil && len(modelSchema.PrimaryFields) != 1 {
		fail("v1 requires exactly one primary key, found %d", len(modelSchema.PrimaryFields))
	}
	plan.Key = KeyPlan{ModelField: field.ModelField, ProtoName: field.ProtoName, JSONName: field.JSONName, DBName: field.DBName, TSType: field.TSType}
}

func buildAuditPlan(r *Resource, plan *Plan, modelSchema *gormschema.Schema, fail func(string, ...any)) {
	if r.Audit.UpdatedAtField == "" {
		fail("Audit.UpdatedAtField is required")
		return
	}
	field := plan.Field(r.Audit.UpdatedAtField)
	if field == nil {
		fail("Audit.UpdatedAtField %q does not exist in Model/Schema", r.Audit.UpdatedAtField)
		return
	}
	if field.ModelType != typeTime {
		fail("Audit.UpdatedAtField %q must have exact time.Time type", r.Audit.UpdatedAtField)
	}
	if !field.Decl.Lifecycle.ServerManaged || field.Decl.Lifecycle.EffCreatable() || field.Decl.Lifecycle.EffUpdatable() || field.CreateOnlyInUpdate {
		fail("Audit.UpdatedAtField %q must be ServerManaged and not client writable", r.Audit.UpdatedAtField)
	}
	if modelSchema == nil || modelSchema.FieldsByName[r.Audit.UpdatedAtField] == nil || modelSchema.FieldsByName[r.Audit.UpdatedAtField].DBName == "" {
		fail("Audit.UpdatedAtField %q has no GORM DBName", r.Audit.UpdatedAtField)
	}
	plan.UpdatedAtModelField = field.ModelField
	plan.UpdatedAtDBName = field.DBName
}

func buildAuthPlan(r *Resource, plan *Plan, modelSchema *gormschema.Schema, fail func(string, ...any)) {
	if !r.Auth.DataScope {
		if r.Auth.SkipCreateScopeCheck {
			fail("Auth.SkipCreateScopeCheck requires DataScope=true")
		}
		return
	}
	resolve := func(role, modelField string) string {
		if modelField == "" {
			fail("Auth.%s is required when DataScope=true", role)
			return ""
		}
		if modelSchema == nil || modelSchema.FieldsByName[modelField] == nil || modelSchema.FieldsByName[modelField].DBName == "" {
			fail("Auth.%s model field %q has no validated GORM DBName", role, modelField)
			return ""
		}
		return modelSchema.FieldsByName[modelField].DBName
	}
	plan.DeptDBName = resolve("DeptField", r.Auth.DeptField)
	plan.CreatorDBName = resolve("CreatorField", r.Auth.CreatorField)
}

var legacyPermissionOperations = map[string]map[string]LegacyPermission{
	"Role": {"GetRole": LegacyPermissionGetQuery},
	"Menu": {"GetMenu": LegacyPermissionGetQuery},
	"Dept": {"GetDept": LegacyPermissionGetQuery},
}

func validateLegacyPermission(resourceName string, operation Operation) error {
	switch operation.LegacyPermission {
	case LegacyPermissionNone:
		return nil
	case LegacyPermissionGetQuery:
		if operation.Kind != OperationGet {
			return fmt.Errorf("operation %q: LegacyPermissionGetQuery is only valid for standard Get", operation.Method)
		}
		allowed, ok := legacyPermissionOperations[resourceName][operation.Method]
		if !ok || allowed != LegacyPermissionGetQuery {
			return fmt.Errorf("operation %q: LegacyPermissionGetQuery is not registered for resource %q", operation.Method, resourceName)
		}
		return nil
	default:
		return fmt.Errorf("operation %q: unknown LegacyPermission value %d", operation.Method, operation.LegacyPermission)
	}
}

func buildOperationPlans(r *Resource, plan *Plan, fail func(string, ...any)) {
	if len(r.Operations) == 0 {
		return
	}
	service := r.Service.Descriptor
	if service == nil {
		fail("Service.Descriptor is required when Operations are declared")
	}
	handlerType, handlerErr := handlerInterfaceType(r.Service.Handler)
	if handlerErr != nil {
		fail("Service.Handler: %v", handlerErr)
	}

	if r.Mode != Generated && r.Mode != Partial {
		fail("Mode %d is unsupported", r.Mode)
	}
	seenMethod := map[string]bool{}
	seenKind := map[OperationKind]bool{}
	customCount := 0
	for _, operation := range r.Operations {
		if operation.Method == "" {
			fail("operation %v has empty Method", operation.Kind)
			continue
		}
		if seenMethod[operation.Method] {
			fail("service method %q is owned by multiple operations in Resource", operation.Method)
		}
		seenMethod[operation.Method] = true
		if operation.Kind != OperationCustom && seenKind[operation.Kind] {
			fail("standard operation kind %v is declared more than once", operation.Kind)
		}
		seenKind[operation.Kind] = true
		if err := validateLegacyPermission(r.Name, operation); err != nil {
			fail("%v", err)
		}
		switch operation.Kind {
		case OperationCustom:
			customCount++
			if r.Mode != Partial {
				fail("Generated Resource cannot declare CUSTOM operation %q", operation.Method)
			}
			if operation.Implementation != OperationOverride {
				fail("CUSTOM operation %q must use OperationOverride", operation.Method)
			}
			if strings.TrimSpace(operation.Action) == "" {
				fail("CUSTOM operation %q requires non-empty Action", operation.Method)
			}
			if operation.ScopePolicy != ScopePolicyExempt {
				fail("CUSTOM operation %q must use ScopePolicyExempt", operation.Method)
			}
		default:
			if operation.Implementation != OperationGenerated {
				fail("standard operation %q must use OperationGenerated", operation.Method)
			}
			if operation.Action != "" || operation.ScopePolicy != ScopePolicyUnspecified {
				fail("standard operation %q cannot declare Action or ScopePolicy", operation.Method)
			}
		}

		requestType, _, requestErr := InspectStruct(operation.Request)
		if requestErr != nil {
			fail("operation %s Request: %v", operation.Method, requestErr)
			continue
		}
		responseType, _, responseErr := InspectStruct(operation.Response)
		if responseErr != nil {
			fail("operation %s Response: %v", operation.Method, responseErr)
			continue
		}
		requestPtr := reflect.PointerTo(requestType)
		responsePtr := reflect.PointerTo(responseType)
		operationPlan := &OperationPlan{
			Operation: operation, RequestType: requestPtr, ResponseType: responsePtr,
			RequestProtoModule: protoESModule(operation.Request), ResponseProtoModule: protoESModule(operation.Response),
		}

		if service != nil {
			method := service.Methods().ByName(protoreflect.Name(operation.Method))
			if method == nil {
				fail("protobuf service %s has no method %q", service.FullName(), operation.Method)
			} else {
				if descriptor := protoMessageDescriptor(operation.Request); descriptor == nil || descriptor.FullName() != method.Input().FullName() {
					fail("operation %s Request does not match protobuf service method input %s", operation.Method, method.Input().FullName())
				}
				if descriptor := protoMessageDescriptor(operation.Response); descriptor == nil || descriptor.FullName() != method.Output().FullName() {
					fail("operation %s Response does not match protobuf service method output %s", operation.Method, method.Output().FullName())
				}
			}
		}
		if handlerErr == nil {
			validateHandlerMethod(handlerType, operation.Method, requestPtr, responsePtr, fail)
		}

		requestFields := structFieldsByGoName(operation.Request)
		responseFields := structFieldsByGoName(operation.Response)
		switch operation.Kind {
		case OperationList:
			operationPlan.List = buildListPlan(operation, plan, requestFields, responseFields, fail)
		case OperationCreate:
			validateItemBinding(operation, plan.EntityType, responseFields, fail)
		case OperationGet, OperationUpdate:
			validateIdentifierBinding(operation, requestFields, fail)
			validateItemBinding(operation, plan.EntityType, responseFields, fail)
		case OperationDelete:
			validateIdentifierBinding(operation, requestFields, fail)
		case OperationBatchDelete:
			validateBatchIDsBinding(operation, requestFields, fail)
		}
		plan.Operations = append(plan.Operations, operationPlan)
	}
	if r.Mode == Partial && customCount == 0 {
		fail("Partial Resource requires at least one CUSTOM operation")
	}
}

func handlerInterfaceType(handler any) (reflect.Type, error) {
	t := reflect.TypeOf(handler)
	if t == nil {
		return nil, fmt.Errorf("typed nil pointer to Connect handler interface is required")
	}
	if t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Interface {
		t = t.Elem()
	}
	if t.Kind() != reflect.Interface {
		return nil, fmt.Errorf("%v is not a Connect handler interface reference", t)
	}
	return t, nil
}

func validateHandlerMethod(handler reflect.Type, methodName string, request, response reflect.Type, fail func(string, ...any)) {
	method, ok := handler.MethodByName(methodName)
	if !ok {
		fail("Connect handler interface %s has no method %q", handler.Name(), methodName)
		return
	}
	t := method.Type
	if t.NumIn() != 2 || !t.In(0).Implements(typeContext) {
		fail("Connect handler method %s has incompatible context/request signature", methodName)
		return
	}
	if got := t.In(1); got != request {
		fail("Connect handler method %s Request is %v, want %v", methodName, got, request)
	}
	if t.NumOut() != 2 || !t.Out(1).Implements(typeError) {
		fail("Connect handler method %s has incompatible response/error signature", methodName)
		return
	}
	if got := t.Out(0); got != response {
		fail("Connect handler method %s Response is %v, want %v", methodName, got, response)
	}
}

func protoESModule(value any) string {
	descriptor := protoMessageDescriptor(value)
	if descriptor == nil {
		return ""
	}
	path := descriptor.ParentFile().Path()
	return strings.TrimSuffix(path, ".proto") + "_pb"
}

func protoMessageDescriptor(value any) protoreflect.MessageDescriptor {
	t := reflect.TypeOf(value)
	if t == nil {
		return nil
	}
	for t.Kind() == reflect.Pointer {
		if t.Elem().Kind() != reflect.Pointer {
			break
		}
		t = t.Elem()
	}
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return nil
	}
	message, ok := reflect.New(t.Elem()).Interface().(proto.Message)
	if !ok {
		return nil
	}
	return message.ProtoReflect().Descriptor()
}

func structFieldsByGoName(value any) map[string]StructInfo {
	_, fields, err := InspectStruct(value)
	if err != nil {
		return nil
	}
	return indexByName(fields)
}

func buildListPlan(operation Operation, resourcePlan *Plan, request, response map[string]StructInfo, fail func(string, ...any)) *ListPlan {
	binding := operation.Binding.List
	list := &ListPlan{
		PageRequestField: binding.Page,
		KeywordField:     binding.Keyword,
		SortField:        binding.Sort,
		ItemsField:       binding.Items,
		PageInfoField:    binding.PageInfo,
		SortableDBNames:  map[string]string{},
	}
	if binding.Page == "" {
		fail("standard List %s requires Page binding", operation.Method)
	} else if field, ok := request[binding.Page]; !ok || protoMessageFullName(field.Type) != "abysstract.admin.v1.PageRequest" {
		fail("standard List %s Page binding %q must be PageRequest", operation.Method, binding.Page)
	}
	if binding.PageInfo == "" {
		fail("standard List %s requires PageInfo binding", operation.Method)
	} else if field, ok := response[binding.PageInfo]; !ok || protoMessageFullName(field.Type) != "abysstract.admin.v1.PageInfo" {
		fail("standard List %s PageInfo binding %q must be PageInfo", operation.Method, binding.PageInfo)
	}
	if binding.Items == "" {
		fail("standard List %s requires Items binding", operation.Method)
	} else if field, ok := response[binding.Items]; !ok || !isRepeatedEntity(field.Type, resourcePlan.EntityType) {
		fail("standard List %s Items binding %q must be repeated %s", operation.Method, binding.Items, resourcePlan.EntityType.Name())
	}

	var searchable, sortable, filterable []*FieldPlan
	for _, field := range resourcePlan.Fields {
		if field.Decl.Searchable {
			searchable = append(searchable, field)
			list.SearchDBNames = append(list.SearchDBNames, field.DBName)
		}
		if field.Decl.Sortable {
			sortable = append(sortable, field)
			list.SortableDBNames[field.JSONName] = field.DBName
		}
		if field.Decl.Filterable {
			filterable = append(filterable, field)
		}
	}
	if len(searchable) > 0 {
		if binding.Keyword == "" {
			fail("standard List %s has Searchable fields but no Keyword binding", operation.Method)
		} else if field, ok := request[binding.Keyword]; !ok || field.Type.Kind() != reflect.String {
			fail("standard List %s Keyword binding %q must be string", operation.Method, binding.Keyword)
		}
	} else if binding.Keyword != "" {
		if field, ok := request[binding.Keyword]; !ok || field.Type.Kind() != reflect.String {
			fail("standard List %s Keyword binding %q must be string", operation.Method, binding.Keyword)
		}
	}
	if len(sortable) > 0 {
		if binding.Sort == "" {
			fail("standard List %s has Sortable fields but no Sort binding", operation.Method)
		} else if field, ok := request[binding.Sort]; !ok || !isRepeatedMessage(field.Type, "abysstract.admin.v1.OrderBy") {
			fail("standard List %s Sort binding %q must be repeated OrderBy", operation.Method, binding.Sort)
		}
	} else if binding.Sort != "" {
		if field, ok := request[binding.Sort]; !ok || !isRepeatedMessage(field.Type, "abysstract.admin.v1.OrderBy") {
			fail("standard List %s Sort binding %q must be repeated OrderBy", operation.Method, binding.Sort)
		}
	}

	mappedRequest := map[string]string{}
	for _, field := range filterable {
		requestFieldName := binding.Filters[field.ModelField]
		if requestFieldName == "" {
			fail("standard List %s Filterable field %q has no Filters mapping", operation.Method, field.ModelField)
			continue
		}
		if previous := mappedRequest[requestFieldName]; previous != "" {
			fail("standard List %s filter request field %q is mapped by both %q and %q", operation.Method, requestFieldName, previous, field.ModelField)
		}
		mappedRequest[requestFieldName] = field.ModelField
		requestField, ok := request[requestFieldName]
		if !ok {
			fail("standard List %s filter request field %q does not exist", operation.Method, requestFieldName)
			continue
		}
		if !requestField.Optional {
			fail("standard List %s filter request field %q lacks protobuf presence", operation.Method, requestFieldName)
		}
		sourceType := requestValueType(requestField, true)
		conversion, err := planFilterConversion(sourceType, field.ModelType, &field.Decl)
		if err != nil {
			fail("standard List %s filter %q: %v", operation.Method, field.ModelField, err)
			continue
		}
		list.Filters = append(list.Filters, FilterPlan{
			SchemaField:     field.ModelField,
			RequestField:    requestFieldName,
			RequestJSONName: requestField.JSONName,
			DBName:          field.DBName,
			ConversionPlan:  conversion,
		})
	}
	for schemaField := range binding.Filters {
		field := resourcePlan.Field(schemaField)
		if field == nil || !field.Decl.Filterable {
			fail("standard List %s Filters maps unknown/non-Filterable field %q", operation.Method, schemaField)
		}
	}
	seenDefaultOrders := map[string]bool{}
	for _, defaultOrder := range binding.DefaultOrders {
		field := resourcePlan.Field(defaultOrder.Field)
		if field == nil {
			fail("standard List %s DefaultOrders references unknown field %q", operation.Method, defaultOrder.Field)
			continue
		}
		if seenDefaultOrders[field.ModelField] {
			fail("standard List %s DefaultOrders repeats field %q", operation.Method, field.ModelField)
			continue
		}
		seenDefaultOrders[field.ModelField] = true
		list.DefaultOrders = append(list.DefaultOrders, DefaultOrderPlan{
			SchemaField: field.ModelField,
			DBName:      field.DBName,
			Desc:        defaultOrder.Desc,
		})
	}
	return list
}

func validateIdentifierBinding(operation Operation, request map[string]StructInfo, fail func(string, ...any)) {
	name := operation.Binding.Identifier
	if name == "" {
		fail("operation %s requires Identifier binding", operation.Method)
		return
	}
	if _, ok := request[name]; !ok {
		fail("operation %s Identifier field %q does not exist", operation.Method, name)
	}
}

func validateBatchIDsBinding(operation Operation, request map[string]StructInfo, fail func(string, ...any)) {
	name := operation.Binding.BatchIDs
	if name == "" {
		fail("operation %s requires BatchIDs binding", operation.Method)
		return
	}
	field, ok := request[name]
	if !ok || field.Type.Kind() != reflect.Slice {
		fail("operation %s BatchIDs field %q must be repeated", operation.Method, name)
	}
}

func validateItemBinding(operation Operation, entityType reflect.Type, response map[string]StructInfo, fail func(string, ...any)) {
	name := operation.Binding.Item
	if name == "" {
		fail("operation %s requires Item binding", operation.Method)
		return
	}
	field, ok := response[name]
	if !ok || field.Type != reflect.PointerTo(entityType) {
		fail("operation %s Item field %q must be *%s", operation.Method, name, entityType.Name())
	}
}

func validateOperationKeyBindings(plan *Plan, fail func(string, ...any)) {
	if plan.Key.ModelField == "" {
		return
	}
	for _, operation := range plan.Operations {
		request := structFieldsByGoName(operation.Request)
		switch operation.Kind {
		case OperationGet, OperationUpdate, OperationDelete:
			if operation.bindingDefaults.identifier {
				candidates := identifierBindingCandidates(request)
				if len(candidates) > 1 {
					fail("operation %s has multiple Key-compatible request fields %v; explicit Identifier binding is required", operation.Method, candidates)
				}
			}
			field, ok := request[operation.Binding.Identifier]
			if !ok {
				continue
			}
			valueType := requestValueType(field, true)
			if !isKeyWireType(valueType) {
				fail("operation %s Identifier field %q must have exact builtin int64 wire type, got %v", operation.Method, operation.Binding.Identifier, valueType)
			}
		case OperationBatchDelete:
			if operation.bindingDefaults.batchIDs {
				candidates := batchIDsBindingCandidates(request)
				if len(candidates) > 1 {
					fail("operation %s has multiple repeated Key-compatible request fields %v; explicit BatchIDs binding is required", operation.Method, candidates)
				}
			}
			field, ok := request[operation.Binding.BatchIDs]
			if !ok || field.Type.Kind() != reflect.Slice {
				continue
			}
			if !isKeyWireType(field.Type.Elem()) {
				fail("operation %s BatchIDs field %q must have exact repeated builtin int64 wire type, got %v", operation.Method, operation.Binding.BatchIDs, field.Type)
			}
		}
	}
}

func isKeyWireType(t reflect.Type) bool {
	return t == reflect.TypeOf(int64(0))
}

func identifierBindingCandidates(fields map[string]StructInfo) []string {
	var candidates []string
	for name, field := range fields {
		if isKeyWireType(requestValueType(field, true)) {
			candidates = append(candidates, name)
		}
	}
	sort.Strings(candidates)
	return candidates
}

func batchIDsBindingCandidates(fields map[string]StructInfo) []string {
	var candidates []string
	for name, field := range fields {
		if field.Type.Kind() == reflect.Slice && isKeyWireType(field.Type.Elem()) {
			candidates = append(candidates, name)
		}
	}
	sort.Strings(candidates)
	return candidates
}

func protoMessageFullName(t reflect.Type) string {
	if t == nil {
		return ""
	}
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return ""
	}
	message, ok := reflect.New(t.Elem()).Interface().(proto.Message)
	if !ok {
		return ""
	}
	return string(message.ProtoReflect().Descriptor().FullName())
}

func isRepeatedEntity(t, entity reflect.Type) bool {
	return t.Kind() == reflect.Slice && t.Elem() == reflect.PointerTo(entity)
}

func isRepeatedMessage(t reflect.Type, fullName string) bool {
	return t.Kind() == reflect.Slice && protoMessageFullName(t.Elem()) == fullName
}

func tsScalarType(t reflect.Type) string {
	if t == nil {
		return "unknown"
	}
	if t == typeTimestamp {
		return "string"
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Int64, reflect.Uint64:
		return "bigint"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Float32, reflect.Float64:
		return "number"
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	default:
		return "unknown"
	}
}

func checkConvertible(field string, modelType, protoType reflect.Type) error {
	if modelType == nil || protoType == nil {
		return fmt.Errorf("field %q has nil conversion type", field)
	}
	if modelType.Kind() == reflect.Pointer && protoType.Kind() == reflect.Pointer && modelType == protoType {
		return checkConvertible(field, modelType.Elem(), protoType.Elem())
	}
	if modelType.Kind() == reflect.Pointer && modelType.Elem() == protoType {
		switch protoType.Kind() {
		case reflect.Int64, reflect.Int32, reflect.Int16, reflect.String, reflect.Bool:
			return nil
		}
	}
	modelKind, protoKind := modelType.Kind(), protoType.Kind()
	isSigned := func(kind reflect.Kind) bool { return kind >= reflect.Int && kind <= reflect.Int64 }
	switch {
	case modelType == typeTime && protoType == typeTimestamp:
		return nil
	case protoKind == reflect.Int32 && protoType.PkgPath() != "" && isSigned(modelKind):
		if modelType.PkgPath() != "" {
			return fmt.Errorf("field %q model named integer %v is unsupported for enum conversion", field, modelType)
		}
		return nil
	case modelKind == reflect.Int16 && protoKind == reflect.Int32 && protoType.PkgPath() == "":
		return nil
	case modelKind == protoKind && isSigned(modelKind):
		if !modelType.AssignableTo(protoType) || !protoType.AssignableTo(modelType) {
			return fmt.Errorf("field %q types %v and %v are not directly assignable (named types unsupported)", field, modelType, protoType)
		}
		return nil
	case modelKind == reflect.String && protoKind == reflect.String:
		if !modelType.AssignableTo(protoType) || !protoType.AssignableTo(modelType) {
			return fmt.Errorf("field %q types %v and %v are not directly assignable (named strings unsupported)", field, modelType, protoType)
		}
		return nil
	case modelKind == reflect.Bool && protoKind == reflect.Bool:
		if !modelType.AssignableTo(protoType) || !protoType.AssignableTo(modelType) {
			return fmt.Errorf("field %q types %v and %v are not directly assignable (named bools unsupported)", field, modelType, protoType)
		}
		return nil
	}
	return fmt.Errorf("field %q types %v and %v are outside the v1 conversion matrix", field, modelType, protoType)
}

func requestValueType(info StructInfo, exists bool) reflect.Type {
	if !exists {
		return nil
	}
	t := info.Type
	if info.Optional && t.Kind() == reflect.Pointer {
		return t.Elem()
	}
	return t
}

func protoEnumName(t reflect.Type) string {
	if t != nil && t.Kind() == reflect.Int32 && t.PkgPath() != "" {
		return t.Name()
	}
	return ""
}

func protoEnumIdentity(t reflect.Type) string {
	if protoEnumName(t) == "" {
		return ""
	}
	if enum, ok := reflect.Zero(t).Interface().(protoreflect.Enum); ok {
		return string(enum.Descriptor().FullName())
	}
	return t.PkgPath() + "." + t.Name()
}

func enumIdentity(t reflect.Type, field *Field) string {
	if field == nil || len(field.EnumMap) == 0 {
		return ""
	}
	return protoEnumName(t)
}

func checkEnumConsistency(field string, enumMap map[string]int16, types ...reflect.Type) error {
	seen := map[string]bool{}
	var names []string
	for _, t := range types {
		if identity := protoEnumIdentity(t); identity != "" && !seen[identity] {
			seen[identity] = true
			names = append(names, identity)
		}
	}
	if len(names) > 0 && len(enumMap) == 0 {
		return fmt.Errorf("field %q uses a protobuf enum and must declare EnumMap", field)
	}
	if len(names) > 1 {
		return fmt.Errorf("field %q uses different enum types %v across directions", field, names)
	}
	return nil
}

func planConversion(source, target reflect.Type, enumName string) ConversionPlan {
	plan := ConversionPlan{SourceType: source, TargetType: target}
	switch {
	case source == typeTime && target == typeTimestamp || source == typeTimestamp && target == typeTime:
		plan.Kind = ConvTime
	case enumName != "":
		plan.Kind = ConvEnum
		plan.EnumTypeName = enumName
		if target != nil && target.PkgPath() == "" && target.Kind() >= reflect.Int && target.Kind() <= reflect.Int64 {
			plan.ModelTypeName = target.Name()
		} else if source != nil && source.PkgPath() == "" && source.Kind() >= reflect.Int && source.Kind() <= reflect.Int64 {
			plan.ModelTypeName = source.Name()
		}
	case source != nil && target != nil && ((source.Kind() == reflect.Int16 && target.Kind() == reflect.Int32) || (source.Kind() == reflect.Int32 && target.Kind() == reflect.Int16)):
		plan.Kind = ConvNarrow
		plan.NeedsBounds = true
	default:
		plan.Kind = ConvDirect
	}
	return plan
}

func planFilterConversion(source, target reflect.Type, field *Field) (ConversionPlan, error) {
	if target != nil && target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	if source == nil || target == nil {
		return ConversionPlan{}, fmt.Errorf("nil filter conversion type")
	}
	if err := checkConvertible(field.Name, target, source); err != nil {
		return ConversionPlan{}, err
	}
	return planConversion(source, target, enumIdentity(source, field)), nil
}

func validateEnumMap(field Field, entityType reflect.Type, create StructInfo, inCreate bool, update StructInfo, inUpdate bool, fail func(string, ...any)) {
	if len(field.EnumMap) == 0 {
		return
	}
	enumType := entityType
	if protoEnumName(enumType) == "" && inCreate {
		enumType = requestValueType(create, true)
	}
	if protoEnumName(enumType) == "" && inUpdate {
		enumType = requestValueType(update, true)
	}
	keys := make([]string, 0, len(field.EnumMap))
	for key := range field.EnumMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	seenValues := make(map[int16]string, len(keys))
	for _, key := range keys {
		value := field.EnumMap[key]
		if previous := seenValues[value]; previous != "" {
			fail("field %q EnumMap keys %q and %q map to duplicate model value %d", field.Name, previous, key, value)
		} else {
			seenValues[value] = key
		}
	}
	valid := enumValueNames(enumType)
	if valid == nil {
		return
	}
	for _, key := range keys {
		if !valid[key] {
			fail("field %q EnumMap key %q is not a valid %s value", field.Name, key, enumType.Name())
		}
	}
}

func enumValueNames(t reflect.Type) map[string]bool {
	if protoEnumName(t) == "" {
		return nil
	}
	value := reflect.New(t).Interface()
	typed, ok := value.(interface{ Type() protoreflect.EnumType })
	if !ok {
		return nil
	}
	values := typed.Type().Descriptor().Values()
	out := make(map[string]bool, values.Len())
	for i := 0; i < values.Len(); i++ {
		out[string(values.Get(i).Name())] = true
	}
	return out
}

// BuildError aggregates all errors discoverable for one Resource or Registry.
type BuildError struct {
	Resource string
	Errors   []string
}

func (e *BuildError) Error() string {
	return fmt.Sprintf("resource %s: %d build errors:\n  - %s", e.Resource, len(e.Errors), strings.Join(e.Errors, "\n  - "))
}

// BuildRegistry performs cross-resource uniqueness, RPC ownership, relation
// identity, and relation-option loading validation. Any failure returns no Plan.
func BuildRegistry(resources []*Resource) ([]*Plan, error) {
	var errors []string
	fail := func(format string, args ...any) {
		errors = append(errors, fmt.Sprintf(format, args...))
	}

	byName := map[string]*Resource{}
	seenTable := map[string]string{}
	seenModel := map[reflect.Type]string{}
	seenEntity := map[reflect.Type]string{}
	methodOwner := map[string]string{}
	for _, resource := range resources {
		if resource == nil {
			fail("Registry contains nil Resource")
			continue
		}
		if previous, duplicate := byName[resource.Name]; duplicate {
			fail("Resource name %q is duplicated by different declarations (%p, %p)", resource.Name, previous, resource)
		} else {
			byName[resource.Name] = resource
		}
		if resource.Table != "" {
			if previous := seenTable[resource.Table]; previous != "" {
				fail("table %q is declared by both %s and %s", resource.Table, previous, resource.Name)
			} else {
				seenTable[resource.Table] = resource.Name
			}
		}
		if modelType := reflect.TypeOf(resource.Model); modelType != nil {
			if previous := seenModel[modelType]; previous != "" {
				fail("model %v is declared by both %s and %s", modelType, previous, resource.Name)
			} else {
				seenModel[modelType] = resource.Name
			}
		}
		if entityType := reflect.TypeOf(resource.Entity); entityType != nil {
			if previous := seenEntity[entityType]; previous != "" {
				fail("entity %v is declared by both %s and %s", entityType, previous, resource.Name)
			} else {
				seenEntity[entityType] = resource.Name
			}
		}
		for _, operation := range resource.Operations {
			if operation.Method == "" {
				continue
			}
			serviceName := "<nil>"
			if resource.Service.Descriptor != nil {
				serviceName = string(resource.Service.Descriptor.FullName())
			}
			key := serviceName + "/" + operation.Method
			if previous := methodOwner[key]; previous != "" {
				fail("service method %s is owned by both %s and %s", key, previous, resource.Name)
			} else {
				methodOwner[key] = resource.Name
			}
		}
	}

	for _, resource := range resources {
		if resource == nil {
			continue
		}
		for _, field := range resource.Fields {
			if field.Relation == nil {
				continue
			}
			if field.Relation.Target == nil {
				fail("%s.%s relation Target is nil", resource.Name, field.Name)
				continue
			}
			target := field.Relation.Target()
			if target == nil {
				fail("%s.%s relation target resolves to nil", resource.Name, field.Name)
				continue
			}
			registered := byName[target.Name]
			if registered == nil {
				fail("%s.%s relation target %q is not registered", resource.Name, field.Name, target.Name)
			} else if registered != target {
				fail("%s.%s relation target %q is not the same instance as Registry", resource.Name, field.Name, target.Name)
			}
		}
	}

	plans := make([]*Plan, 0, len(resources))
	plansByResource := map[*Resource]*Plan{}
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		plan, err := resource.Build()
		if err != nil {
			fail("%v", err)
			continue
		}
		plans = append(plans, plan)
		plansByResource[resource] = plan
	}

	for _, sourcePlan := range plans {
		for _, sourceField := range sourcePlan.Fields {
			relation := sourceField.Decl.Relation
			if relation == nil || relation.Target == nil {
				continue
			}
			target := relation.Target()
			targetPlan := plansByResource[target]
			if targetPlan == nil {
				continue
			}
			buildRelationLoadPlan(sourcePlan, sourceField, targetPlan, fail)
		}
	}

	if len(errors) > 0 {
		return nil, &BuildError{Resource: "<registry>", Errors: errors}
	}
	return plans, nil
}

func buildRelationLoadPlan(sourcePlan *Plan, sourceField *FieldPlan, targetPlan *Plan, fail func(string, ...any)) {
	relation := sourceField.Decl.Relation
	list := targetPlan.Operation(OperationList)
	if list == nil || list.List == nil || list.List.PageRequestField == "" || list.List.PageInfoField == "" || list.List.ItemsField == "" {
		fail("%s.%s relation target %s has no standard paginated List", sourcePlan.Res.Name, sourceField.ModelField, targetPlan.Res.Name)
		return
	}
	ref := targetPlan.Field(relation.RefKey)
	if ref == nil {
		fail("%s.%s relation target %s has no RefKey %q", sourcePlan.Res.Name, sourceField.ModelField, targetPlan.Res.Name, relation.RefKey)
		return
	}
	label := targetPlan.Field(relation.LabelField)
	if label == nil {
		fail("%s.%s relation target %s has no LabelField %q", sourcePlan.Res.Name, sourceField.ModelField, targetPlan.Res.Name, relation.LabelField)
		return
	}
	if label.EntityType.Kind() != reflect.String || label.EntityType.PkgPath() != "" {
		fail("%s.%s relation LabelField %q must be protobuf string", sourcePlan.Res.Name, sourceField.ModelField, relation.LabelField)
	}
	if !label.Decl.Listable {
		fail("%s.%s relation LabelField %q must be listable", sourcePlan.Res.Name, sourceField.ModelField, relation.LabelField)
	}
	if !validRelationRefType(ref.EntityType) {
		fail("%s.%s relation RefKey %q has unsupported protobuf type %v", sourcePlan.Res.Name, sourceField.ModelField, relation.RefKey, ref.EntityType)
	}
	if sourceField.ModelType != ref.ModelType || !sourceField.ModelType.AssignableTo(ref.ModelType) || !ref.ModelType.AssignableTo(sourceField.ModelType) {
		fail("%s.%s model type %v is incompatible with relation RefKey %s.%s type %v", sourcePlan.Res.Name, sourceField.ModelField, sourceField.ModelType, targetPlan.Res.Name, relation.RefKey, ref.ModelType)
	}
	remoteSearch := label.Decl.Searchable && list.List.KeywordField != ""
	resolved := &RelationLoadPlan{
		TargetResource: targetPlan.Res.Name, RefJSONName: ref.JSONName, LabelJSONName: label.JSONName,
		RemoteSearchEnabled: remoteSearch,
	}
	sourceField.Relation = resolved
	relation.RefJSONName = ref.JSONName
	relation.LabelJSONName = label.JSONName
	relation.RemoteSearchEnabled = remoteSearch
}

func validRelationRefType(t reflect.Type) bool {
	if t == nil || t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		return false
	}
	if protoEnumName(t) != "" {
		return false
	}
	switch t.Kind() {
	case reflect.String, reflect.Int32, reflect.Int64, reflect.Uint32, reflect.Uint64:
		return t.PkgPath() == ""
	default:
		return false
	}
}
