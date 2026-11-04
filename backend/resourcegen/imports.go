package resourcegen

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

const wktTSModule = "@bufbuild/protobuf/wkt"

type pkgRef struct {
	path string
	name string
}

type aliasSet struct {
	aliasByPath map[string]string
	nameByPath  map[string]string
}

func (a aliasSet) alias(path string) string {
	return a.aliasByPath[path]
}

func (a aliasSet) typeRef(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	alias := a.alias(t.PkgPath())
	if alias == "" {
		return t.Name()
	}
	return alias + "." + t.Name()
}

func collectGoPackages(types ...reflect.Type) aliasSet {
	var refs []pkgRef
	seen := map[string]bool{}
	for _, t := range types {
		if t == nil {
			continue
		}
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.PkgPath() == "" {
			continue
		}
		if seen[t.PkgPath()] {
			continue
		}
		seen[t.PkgPath()] = true
		refs = append(refs, pkgRef{path: t.PkgPath(), name: defaultPackageName(t)})
	}
	return assignAliases(refs)
}

func defaultPackageName(t reflect.Type) string {
	name := t.String()
	for strings.HasPrefix(name, "*") {
		name = name[1:]
	}
	if dot := strings.IndexByte(name, '.'); dot > 0 {
		return name[:dot]
	}
	if path := t.PkgPath(); path != "" {
		if i := strings.LastIndex(path, "/"); i >= 0 {
			return path[i+1:]
		}
		return path
	}
	return t.Name()
}

func assignAliases(refs []pkgRef) aliasSet {
	byName := map[string][]string{}
	nameByPath := map[string]string{}
	for _, ref := range refs {
		nameByPath[ref.path] = ref.name
		if !containsString(byName[ref.name], ref.path) {
			byName[ref.name] = append(byName[ref.name], ref.path)
		}
	}
	aliasByPath := map[string]string{}
	for name, paths := range byName {
		sort.Strings(paths)
		if len(paths) == 1 {
			aliasByPath[paths[0]] = name
			continue
		}
		for i, path := range paths {
			if i == 0 {
				aliasByPath[path] = name
				continue
			}
			aliasByPath[path] = fmt.Sprintf("%s_%d", name, i+1)
		}
	}
	return aliasSet{aliasByPath: aliasByPath, nameByPath: nameByPath}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func protoFileOfType(t reflect.Type) (string, error) {
	if t == nil {
		return "", fmt.Errorf("resourcegen: missing proto type")
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	value := reflect.New(t).Interface()
	message, ok := value.(proto.Message)
	if !ok {
		return "", fmt.Errorf("resourcegen: %s is not a protobuf message", t)
	}
	return string(message.ProtoReflect().Descriptor().ParentFile().Path()), nil
}

func protoFileOfService(desc protoreflect.ServiceDescriptor) string {
	if desc == nil {
		return ""
	}
	return string(desc.ParentFile().Path())
}

func tsModuleForProto(cfg Config, protoFile string) (string, error) {
	if protoFile == "" {
		return "", fmt.Errorf("resourcegen: empty proto file path")
	}
	if isWKT(protoFile) {
		return wktTSModule, nil
	}
	module, ok := cfg.TSImports.ProtoModules[protoFile]
	if !ok || strings.TrimSpace(module) == "" {
		return "", fmt.Errorf("resourcegen: missing TS mapping for proto file %q", protoFile)
	}
	return module, nil
}

func isWKT(protoFile string) bool {
	return strings.HasPrefix(protoFile, "google/protobuf/")
}

func requiredProtoFiles(plan *schema.Plan) ([]string, error) {
	seen := map[string]bool{}
	var files []string
	add := func(path string) {
		if path == "" || seen[path] || isWKT(path) {
			return
		}
		seen[path] = true
		files = append(files, path)
	}
	if service := plan.Res.Service.Descriptor; service != nil {
		add(protoFileOfService(service))
	}
	for _, value := range []any{plan.Res.Entity, plan.Res.CreateReq, plan.Res.UpdateReq} {
		if value == nil {
			continue
		}
		path, err := protoFileOfType(reflect.TypeOf(value))
		if err != nil {
			return nil, err
		}
		add(path)
	}
	for _, operation := range plan.Operations {
		for _, typ := range []reflect.Type{operation.RequestType, operation.ResponseType} {
			if typ == nil {
				continue
			}
			path, err := protoFileOfType(typ)
			if err != nil {
				return nil, err
			}
			add(path)
		}
	}
	sort.Strings(files)
	return files, nil
}

func checkProtoMappings(cfg Config, plans []*schema.Plan) error {
	for _, plan := range plans {
		files, err := requiredProtoFiles(plan)
		if err != nil {
			return err
		}
		for _, file := range files {
			if _, err := tsModuleForProto(cfg, file); err != nil {
				return err
			}
		}
	}
	return nil
}

func derefType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}
