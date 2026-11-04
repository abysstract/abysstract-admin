package adminapp

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/abysstract/abysstract-admin/backend/module"
)

var moduleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

type procedureSpec struct {
	Module     string
	Service    string
	Method     string
	Procedure  string
	Prefix     string
	Descriptor protoreflect.MethodDescriptor
}

func sortAndValidateModules(defs []module.Definition) ([]module.Definition, error) {
	if len(defs) == 0 {
		return nil, ErrMissingSystemModule
	}
	index := make(map[string]int, len(defs))
	for i, def := range defs {
		if !moduleNamePattern.MatchString(def.Name) {
			return nil, fieldError(def.Name, "Name", "must be 1-64 characters starting with a lowercase letter")
		}
		if _, exists := index[def.Name]; exists {
			return nil, fieldError(def.Name, "Name", "duplicated")
		}
		if def.Create == nil {
			return nil, fieldError(def.Name, "Create", "function is required")
		}
		seenReq := make(map[string]struct{}, len(def.Requires))
		for _, req := range def.Requires {
			if req == def.Name {
				return nil, fieldError(def.Name, "Requires", "self dependency is not allowed")
			}
			if strings.TrimSpace(req) == "" {
				return nil, fieldError(def.Name, "Requires", "contains an empty module name")
			}
			if _, dup := seenReq[req]; dup {
				return nil, fieldError(def.Name, "Requires", fmt.Sprintf("duplicate dependency %q", req))
			}
			seenReq[req] = struct{}{}
		}
		index[def.Name] = i
	}
	if _, ok := index["system"]; !ok {
		return nil, ErrMissingSystemModule
	}
	for _, def := range defs {
		for _, req := range def.Requires {
			if _, ok := index[req]; !ok {
				return nil, fieldError(def.Name, "Requires", fmt.Sprintf("missing module %q", req))
			}
		}
	}
	ordered, err := topologicalSort(defs)
	if err != nil {
		return nil, err
	}
	return ordered, nil
}

func topologicalSort(defs []module.Definition) ([]module.Definition, error) {
	index := make(map[string]module.Definition, len(defs))
	incoming := make(map[string]int, len(defs))
	edges := make(map[string][]string, len(defs))
	for _, def := range defs {
		index[def.Name] = def
		incoming[def.Name] = 0
	}
	for _, def := range defs {
		for _, req := range def.Requires {
			edges[req] = append(edges[req], def.Name)
			incoming[def.Name]++
		}
	}
	ready := make([]string, 0, len(defs))
	for name, count := range incoming {
		if count == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)
	ordered := make([]module.Definition, 0, len(defs))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		ordered = append(ordered, index[name])
		next := append([]string(nil), edges[name]...)
		sort.Strings(next)
		for _, dep := range next {
			incoming[dep]--
			if incoming[dep] == 0 {
				ready = append(ready, dep)
				sort.Strings(ready)
			}
		}
	}
	if len(ordered) != len(defs) {
		return nil, fieldError("", "Requires", "dependency cycle detected")
	}
	return ordered, nil
}

func collectProcedures(defs []module.Definition) ([]procedureSpec, map[string]string, error) {
	out := make([]procedureSpec, 0)
	owner := make(map[string]string)
	services := make(map[string]string)
	for _, def := range defs {
		seenLocal := make(map[string]struct{})
		for i, svc := range def.Services {
			if svc.Descriptor == nil {
				return nil, nil, fieldError(def.Name, "Services", fmt.Sprintf("index %d is missing a descriptor", i))
			}
			fullName := string(svc.Descriptor.FullName())
			if _, ok := seenLocal[fullName]; ok {
				return nil, nil, fieldError(def.Name, "Services", fmt.Sprintf("duplicate service %s", fullName))
			}
			if prev, ok := services[fullName]; ok {
				return nil, nil, fieldError(def.Name, "Services", fmt.Sprintf("service %s already owned by module %q", fullName, prev))
			}
			seenLocal[fullName] = struct{}{}
			services[fullName] = def.Name
			methods := svc.Descriptor.Methods()
			for j := 0; j < methods.Len(); j++ {
				method := methods.Get(j)
				if method.IsStreamingClient() || method.IsStreamingServer() {
					return nil, nil, fieldError(def.Name, "Services", fmt.Sprintf("%s/%s is streaming", fullName, method.Name()))
				}
				procedure := "/" + fullName + "/" + string(method.Name())
				owner[procedure] = def.Name
				out = append(out, procedureSpec{
					Module:     def.Name,
					Service:    fullName,
					Method:     string(method.Name()),
					Procedure:  procedure,
					Prefix:     "/" + fullName + "/",
					Descriptor: method,
				})
			}
		}
	}
	return out, owner, nil
}
