package schema

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

// StructInfo is the reflected representation of one generated protobuf or
// Model struct field.
type StructInfo struct {
	Name      string
	Type      reflect.Type
	ProtoName string
	JSONName  string
	Optional  bool
	Repeated  bool
}

// InspectStruct returns exported struct fields and protobuf tag metadata.
func InspectStruct(value any) (reflect.Type, []StructInfo, error) {
	t := reflect.TypeOf(value)
	if t == nil {
		return nil, nil, fmt.Errorf("nil type")
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("%v is not a struct", t)
	}
	out := make([]StructInfo, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		info := StructInfo{Name: field.Name, Type: field.Type, Repeated: field.Type.Kind() == reflect.Slice}
		if tag := field.Tag.Get("protobuf"); tag != "" {
			for _, part := range strings.Split(tag, ",") {
				switch {
				case strings.HasPrefix(part, "name="):
					info.ProtoName = strings.TrimPrefix(part, "name=")
				case strings.HasPrefix(part, "json="):
					info.JSONName = strings.TrimPrefix(part, "json=")
				case part == "oneof":
					info.Optional = true
				case part == "rep":
					info.Repeated = true
				}
			}
		}
		if info.ProtoName != "" && info.JSONName == "" {
			info.JSONName = lowerCamel(info.ProtoName)
		}
		out = append(out, info)
	}
	return t, out, nil
}

func protoFieldName(goName string) string {
	if goName == "" {
		return ""
	}
	runes := []rune(goName)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			previousLower := i > 0 && unicode.IsLower(runes[i-1])
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if i > 0 && (previousLower || nextLower && unicode.IsUpper(runes[i-1])) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func lowerCamel(snake string) string {
	parts := strings.Split(snake, "_")
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		runes := []rune(part)
		b.WriteRune(unicode.ToUpper(runes[0]))
		b.WriteString(string(runes[1:]))
	}
	return b.String()
}

func indexByName(fields []StructInfo) map[string]StructInfo {
	out := make(map[string]StructInfo, len(fields))
	for _, field := range fields {
		out[field.Name] = field
	}
	return out
}

func indexByProtoName(fields []StructInfo) map[string]StructInfo {
	out := make(map[string]StructInfo, len(fields))
	for _, field := range fields {
		if field.ProtoName != "" {
			out[field.ProtoName] = field
		}
	}
	return out
}
