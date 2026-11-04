package gen

import (
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// Task 6.1: converters must be ordinary, strongly typed Go functions that
// generated code users can call directly without reflection or any assertions.
func TestGeneratedConvertersExposeStronglyTypedFunctions(t *testing.T) {
	dictType, err := DictTypeCreateToModel(&adminv1.CreateDictTypeRequest{
		DictName: "名称",
		DictType: "category",
	})
	if err != nil {
		t.Fatalf("DictTypeCreateToModel() error = %v", err)
	}
	dictType.Status = 1
	if _, err := DictTypeToEntity(dictType); err != nil {
		t.Fatalf("DictTypeToEntity() error = %v", err)
	}
	if _, _, err := DictTypeUpdateToModel(dictType, &adminv1.UpdateDictTypeRequest{
		Id:       dictType.ID,
		DictName: proto.String("新名称"),
	}); err != nil {
		t.Fatalf("DictTypeUpdateToModel() error = %v", err)
	}

	dictData, err := DictDataCreateToModel(&adminv1.CreateDictDataRequest{
		DictType:  "category",
		DictLabel: "标签",
		DictValue: "value",
	})
	if err != nil {
		t.Fatalf("DictDataCreateToModel() error = %v", err)
	}
	dictData.Status = 1
	if _, err := DictDataToEntity(dictData); err != nil {
		t.Fatalf("DictDataToEntity() error = %v", err)
	}
	if _, _, err := DictDataUpdateToModel(dictData, &adminv1.UpdateDictDataRequest{
		Id:        dictData.ID,
		DictLabel: proto.String("新标签"),
	}); err != nil {
		t.Fatalf("DictDataUpdateToModel() error = %v", err)
	}

	// Compile-time assignments prove the functions use concrete types rather
	// than reflection dispatch or any-returning adapters.
	var _ *model.SysDictType = dictType
	var _ *model.SysDictDatum = dictData
}

// Task 6.2: the generated narrowing conversion must reject values outside the
// model int16 range instead of silently truncating the int32 protobuf value.
func TestGeneratedConvertersRejectNarrowingOverflow(t *testing.T) {
	for _, value := range []int32{32768, -32769} {
		t.Run("create", func(t *testing.T) {
			_, err := DictDataCreateToModel(&adminv1.CreateDictDataRequest{
				DictType:  "category",
				DictLabel: "标签",
				DictValue: "value",
				IsDefault: value,
			})
			if err == nil {
				t.Fatalf("CreateToModel(is_default=%d) error = nil, want overflow error", value)
			}
		})
		t.Run("update", func(t *testing.T) {
			old := &model.SysDictDatum{IsDefault: 1}
			_, columns, err := DictDataUpdateToModel(old, &adminv1.UpdateDictDataRequest{
				IsDefault: proto.Int32(value),
			})
			if err == nil {
				t.Fatalf("UpdateToModel(is_default=%d) error = nil, want overflow error", value)
			}
			if columns.Len() != 0 {
				t.Fatalf("UpdateToModel(is_default=%d) columns = %v, want empty on error", value, columns)
			}
		})
	}

	for _, value := range []int32{-32768, 32767} {
		m, err := DictDataCreateToModel(&adminv1.CreateDictDataRequest{
			DictType:  "category",
			DictLabel: "标签",
			DictValue: "value",
			IsDefault: value,
		})
		if err != nil {
			t.Fatalf("CreateToModel(is_default=%d) error = %v, want nil", value, err)
		}
		if m.IsDefault != int16(value) {
			t.Fatalf("CreateToModel(is_default=%d) model value = %d", value, m.IsDefault)
		}
	}
}

// Task 6.3: UpdateToModel is presence-aware. It changes only submitted
// optional fields, preserves omitted fields, and records only changed columns.
// Empty ColumnSet is the converter result for an all-omitted patch; the later
// Controller contract rejects that request before it reaches any SQL path.
func TestGeneratedUpdateUsesPresenceForSelectiveColumns(t *testing.T) {
	old := &model.SysDictDatum{
		DictSort:  3,
		DictLabel: "旧标签",
		DictValue: "old",
		IsDefault: 1,
		Remark:    "旧备注",
	}

	t.Run("omitted field is preserved", func(t *testing.T) {
		got, columns, err := DictDataUpdateToModel(old, &adminv1.UpdateDictDataRequest{
			DictLabel: proto.String("新标签"),
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.DictSort != old.DictSort {
			t.Fatalf("omitted DictSort = %d, want %d", got.DictSort, old.DictSort)
		}
		if got.DictLabel != "新标签" {
			t.Fatalf("DictLabel = %q, want new value", got.DictLabel)
		}
		if columns.Len() != 1 || !columns.Contains(DictDataColumnDictLabel) {
			t.Fatalf("columns = %v, want only %q", columns.Values(), DictDataColumnDictLabel)
		}
	})

	t.Run("explicit zero is applied", func(t *testing.T) {
		got, columns, err := DictDataUpdateToModel(old, &adminv1.UpdateDictDataRequest{
			DictSort: proto.Int32(0),
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.DictSort != 0 {
			t.Fatalf("explicit zero DictSort = %d, want 0", got.DictSort)
		}
		if columns.Len() != 1 || !columns.Contains(DictDataColumnDictSort) {
			t.Fatalf("columns = %v, want only %q", columns.Values(), DictDataColumnDictSort)
		}
	})

	t.Run("all omitted fields return no changes", func(t *testing.T) {
		got, columns, err := DictDataUpdateToModel(old, &adminv1.UpdateDictDataRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if *got != *old {
			t.Fatalf("all-omitted patch changed model: got=%+v want=%+v", got, old)
		}
		if columns.Len() != 0 {
			t.Fatalf("all-omitted patch columns = %v, want empty", columns.Values())
		}
	})
}

// Task 6.4: CreateOnly fields are accepted when omitted or submitted with the
// existing value, but a changed submitted value returns a converter error.
func TestGeneratedUpdateRejectsCreateOnlyChanges(t *testing.T) {
	old := &model.SysDictType{DictName: "名称", DictType: "original"}

	t.Run("same submitted value is allowed", func(t *testing.T) {
		got, columns, err := DictTypeUpdateToModel(old, &adminv1.UpdateDictTypeRequest{
			DictType: proto.String("original"),
		})
		if err != nil {
			t.Fatalf("same CreateOnly value error = %v", err)
		}
		if got.DictType != old.DictType || columns.Len() != 0 {
			t.Fatalf("same CreateOnly value = model=%+v columns=%v, want no change", got, columns.Values())
		}
	})

	t.Run("changed submitted value is rejected", func(t *testing.T) {
		got, columns, err := DictTypeUpdateToModel(old, &adminv1.UpdateDictTypeRequest{
			DictType: proto.String("changed"),
		})
		if err == nil {
			t.Fatal("changed CreateOnly value error = nil, want error")
		}
		if got != nil || columns.Len() != 0 {
			t.Fatalf("changed CreateOnly value = model=%v columns=%v, want nil/empty on error", got, columns.Values())
		}
	})

	t.Run("omitted value is allowed", func(t *testing.T) {
		got, columns, err := DictTypeUpdateToModel(old, &adminv1.UpdateDictTypeRequest{})
		if err != nil {
			t.Fatalf("omitted CreateOnly value error = %v", err)
		}
		if got.DictType != old.DictType || columns.Len() != 0 {
			t.Fatalf("omitted CreateOnly value = model=%+v columns=%v, want no change", got, columns.Values())
		}
	})
}

// Task 6.6: generated enum mappings reject values that are not in Schema's
// EnumMap in both directions instead of silently coercing them to zero.
func TestGeneratedEnumMappingsRejectUnknownValues(t *testing.T) {
	if _, err := DictDataToEntity(&model.SysDictDatum{Status: 99}); err == nil {
		t.Fatal("unknown model enum value error = nil, want error")
	}
	if _, err := dictDataStatusFromModel(99); err == nil {
		t.Fatal("unknown model enum helper error = nil, want error")
	}
	if _, err := dictDataStatusToModel(adminv1.Status(99)); err == nil {
		t.Fatal("unknown protobuf enum helper error = nil, want error")
	}

	entity, err := DictDataToEntity(&model.SysDictDatum{Status: 1})
	if err != nil {
		t.Fatalf("known model enum error = %v", err)
	}
	if entity.Status != adminv1.Status_STATUS_ENABLED {
		t.Fatalf("known model enum = %v, want STATUS_ENABLED", entity.Status)
	}
	modelStatus, err := dictDataStatusToModel(adminv1.Status_STATUS_DISABLED)
	if err != nil {
		t.Fatalf("known protobuf enum error = %v", err)
	}
	if modelStatus != 2 {
		t.Fatalf("known protobuf enum = %d, want 2", modelStatus)
	}
}

// Task 6.7: model timestamps outside protobuf Timestamp's range must fail
// conversion rather than emitting an invalid response message.
func TestGeneratedToEntityRejectsOutOfRangeTimestamp(t *testing.T) {
	outOfRange := time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)

	if _, err := DictTypeToEntity(&model.SysDictType{
		Status:     1,
		CreateTime: outOfRange,
	}); err == nil {
		t.Fatal("DictTypeToEntity(out-of-range create_time) error = nil, want error")
	}
	if _, err := DictDataToEntity(&model.SysDictDatum{
		Status:     1,
		CreateTime: outOfRange,
	}); err == nil {
		t.Fatal("DictDataToEntity(out-of-range create_time) error = nil, want error")
	}
}

// Task 7.1: the formal generated package exports Schema-derived validation
// functions for both create and update request shapes.
func TestGeneratedValidatorsAreDirectlyCallable(t *testing.T) {
	if err := DictTypeValidateCreate(&adminv1.CreateDictTypeRequest{
		DictName: "名称",
		DictType: "category",
	}); err != nil {
		t.Fatalf("DictTypeValidateCreate() error = %v", err)
	}
	if err := DictTypeValidateUpdate(&adminv1.UpdateDictTypeRequest{
		DictName: proto.String("名称"),
	}); err != nil {
		t.Fatalf("DictTypeValidateUpdate() error = %v", err)
	}
	if err := DictDataValidateCreate(&adminv1.CreateDictDataRequest{
		DictType:  "category",
		DictLabel: "标签",
		DictValue: "value",
	}); err != nil {
		t.Fatalf("DictDataValidateCreate() error = %v", err)
	}
	if err := DictDataValidateUpdate(&adminv1.UpdateDictDataRequest{
		DictLabel: proto.String("标签"),
	}); err != nil {
		t.Fatalf("DictDataValidateUpdate() error = %v", err)
	}
}

// Task 7.2: Required string validation treats whitespace-only input as empty
// while preserving non-empty Unicode content.
func TestGeneratedRequiredValidationUsesTrimSpace(t *testing.T) {
	for _, value := range []string{"", " ", "\t", "\n", " \t\n "} {
		err := DictDataValidateCreate(&adminv1.CreateDictDataRequest{
			DictType:  "category",
			DictLabel: value,
			DictValue: "value",
		})
		if err == nil {
			t.Fatalf("Create required whitespace %q error = nil, want error", value)
		}
	}
	if err := DictDataValidateCreate(&adminv1.CreateDictDataRequest{
		DictType:  "category",
		DictLabel: " 前后空白仍有内容 ",
		DictValue: "value",
	}); err != nil {
		t.Fatalf("Create non-empty Unicode content error = %v", err)
	}

	if err := DictDataValidateUpdate(&adminv1.UpdateDictDataRequest{
		DictLabel: proto.String(" \t "),
	}); err == nil {
		t.Fatal("Update required whitespace error = nil, want error")
	}
}

// Task 7.3: Update validation checks only fields with protobuf presence, and
// it includes CreateOnly fields when they are explicitly submitted.
func TestGeneratedUpdateValidationIsPresenceAwareIncludingCreateOnly(t *testing.T) {
	if err := DictDataValidateUpdate(&adminv1.UpdateDictDataRequest{}); err != nil {
		t.Fatalf("omitted required update fields error = %v, want nil", err)
	}
	if err := DictDataValidateUpdate(&adminv1.UpdateDictDataRequest{
		DictValue: proto.String(""),
	}); err == nil {
		t.Fatal("submitted empty required update field error = nil, want error")
	}
	if err := DictTypeValidateUpdate(&adminv1.UpdateDictTypeRequest{
		DictType: proto.String(""),
	}); err == nil {
		t.Fatal("submitted empty CreateOnly field error = nil, want error")
	}
	if err := DictTypeValidateUpdate(&adminv1.UpdateDictTypeRequest{}); err != nil {
		t.Fatalf("omitted CreateOnly update field error = %v, want nil", err)
	}
}

// Narrowed optional update values are request-shape validation, not a
// post-lock conversion concern: callers must receive InvalidArgument before
// any GetByKeyForUpdate path can observe the identifier.
func TestGeneratedUpdateValidationRejectsNarrowedValues(t *testing.T) {
	if err := DictDataValidateUpdate(&adminv1.UpdateDictDataRequest{IsDefault: proto.Int32(65536)}); err == nil {
		t.Fatal("overflowing is_default update error = nil, want validation error")
	}
}

// Task 7.4: MaxLen is measured in Unicode code points rather than UTF-8 byte
// length, and applies to both create and submitted update values.
func TestGeneratedMaxLenCountsUnicodeRunes(t *testing.T) {
	atLimit := strings.Repeat("界", 100)
	overLimit := strings.Repeat("界", 101)

	if err := DictDataValidateCreate(&adminv1.CreateDictDataRequest{
		DictType:  "category",
		DictLabel: atLimit,
		DictValue: "value",
	}); err != nil {
		t.Fatalf("100-rune Create label error = %v, want nil", err)
	}
	if err := DictDataValidateCreate(&adminv1.CreateDictDataRequest{
		DictType:  "category",
		DictLabel: overLimit,
		DictValue: "value",
	}); err == nil {
		t.Fatal("101-rune Create label error = nil, want error")
	}
	if err := DictDataValidateUpdate(&adminv1.UpdateDictDataRequest{
		DictLabel: proto.String(overLimit),
	}); err == nil {
		t.Fatal("101-rune Update label error = nil, want error")
	}
}
