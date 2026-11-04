package schema

import (
	"reflect"
	"strings"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"

	enuma "github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testenum/a"

	enumb "github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testenum/b"
)

func TestRequestValueTypeIsTheSingleOptionalNormalizationPoint(t *testing.T) {
	info := StructInfo{Type: reflect.TypeOf((*adminv1.Status)(nil)), Optional: true}
	got := requestValueType(info, true)
	if got != reflect.TypeOf(adminv1.Status_STATUS_ENABLED) {
		t.Fatalf("optional enum normalized to %v", got)
	}
	if requestValueType(info, false) != nil {
		t.Fatal("absent request field must normalize to nil")
	}
}

func TestEnumConsistencyCoversBothDirectionsAndOptionalEnums(t *testing.T) {
	status := reflect.TypeOf(adminv1.Status_STATUS_ENABLED)
	plain := reflect.TypeOf(int32(0))
	if err := checkEnumConsistency("Status", nil, plain, status); err == nil {
		t.Fatal("request enum without EnumMap must fail even when Entity is plain int32")
	}
	if err := checkEnumConsistency("Status", map[string]int16{"STATUS_ENABLED": 1}, status, plain); err != nil {
		t.Fatalf("entity enum + plain request must be supported: %v", err)
	}
	type OtherStatus int32
	if err := checkEnumConsistency("Status", map[string]int16{"STATUS_ENABLED": 1}, status, reflect.TypeOf(OtherStatus(0))); err == nil {
		t.Fatal("different enum types across directions must fail")
	}
}

func TestV1ConversionMatrixRejectsUnsupportedAndNamedTypes(t *testing.T) {
	type ID int64
	type Code string
	tests := []struct {
		model any
		proto any
	}{
		{int8(0), int64(0)},
		{ID(0), int64(0)},
		{Code(""), ""},
	}
	for _, tt := range tests {
		if err := checkConvertible("X", reflect.TypeOf(tt.model), reflect.TypeOf(tt.proto)); err == nil {
			t.Fatalf("unsupported conversion %T <-> %T was accepted", tt.model, tt.proto)
		}
	}
	if err := checkConvertible("X", reflect.TypeOf(int16(0)), reflect.TypeOf(int32(0))); err != nil {
		t.Fatalf("int16 <-> int32 must remain supported: %v", err)
	}
}

func TestRelationRefKeyV1ScalarWhitelist(t *testing.T) {
	allowed := []any{"", int32(0), int64(0), uint32(0), uint64(0)}
	for _, value := range allowed {
		if !validRelationRefType(reflect.TypeOf(value)) {
			t.Errorf("%T should be an allowed relation RefKey", value)
		}
	}
	rejected := []any{false, int8(0), int16(0), uint8(0), uint16(0), float32(0), float64(0), []byte(nil), adminv1.Status_STATUS_ENABLED, (*adminv1.PageRequest)(nil)}
	for _, value := range rejected {
		if validRelationRefType(reflect.TypeOf(value)) {
			t.Errorf("%T should be rejected as a relation RefKey", value)
		}
	}
}

func TestBuildAggregatesIndependentErrors(t *testing.T) {
	r := renamedColumnsResource()
	r.Name = ""
	r.PermPrefix = ""
	r.Key.ModelField = ""
	_, err := r.Build()
	buildErr, ok := err.(*BuildError)
	if !ok || len(buildErr.Errors) < 3 {
		t.Fatalf("Build must aggregate independent errors, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "Name") || !strings.Contains(err.Error(), "PermPrefix") || !strings.Contains(err.Error(), "Key.ModelField") {
		t.Fatalf("aggregate error omitted expected causes: %v", err)
	}
}

func TestEnumConsistencyRejectsSameShortNameFromDifferentProtoPackages(t *testing.T) {
	typeA := reflect.TypeOf(enuma.Status_STATUS_ENABLED)
	typeB := reflect.TypeOf(enumb.Status_STATUS_ENABLED)
	if typeA.Name() != typeB.Name() {
		t.Fatalf("fixture must use the same short enum name: %s vs %s", typeA.Name(), typeB.Name())
	}
	if err := checkEnumConsistency("Status", map[string]int16{"STATUS_ENABLED": 1}, typeA, typeB); err == nil {
		t.Fatal("same short enum name from different protobuf packages must fail consistency validation")
	}
}

func TestBuildRejectsEnumMapDuplicateModelValues(t *testing.T) {
	resource := validDictTypeResource()
	for i := range resource.Fields {
		if resource.Fields[i].Name == "Status" {
			resource.Fields[i] = F("Status", EnumMap(map[string]int16{
				"STATUS_ENABLED":  1,
				"STATUS_DISABLED": 1,
			}), ReadOnly())
		}
	}
	_, err := resource.Build()
	if err == nil || !strings.Contains(err.Error(), "duplicate model value 1") {
		t.Fatalf("non-bijective EnumMap must fail Build: %v", err)
	}
}

func TestNullableScalarEntityKeepsPresence(t *testing.T) {
	for _, value := range []any{(*bool)(nil), (*string)(nil), (*int32)(nil), (*int64)(nil)} {
		typ := reflect.TypeOf(value)
		if err := checkConvertible("Value", typ, typ); err != nil {
			t.Fatal(err)
		}
		if tsScalarType(typ) == "unknown" {
			t.Fatalf("missing frontend scalar for %v", typ)
		}
	}
	if err := checkConvertible("Value", reflect.TypeOf((*bool)(nil)), reflect.TypeOf((*string)(nil))); err == nil {
		t.Fatal("accepted incompatible nullable scalar")
	}
}
