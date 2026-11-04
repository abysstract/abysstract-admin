package adminapp

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/abysstract/abysstract-admin/backend/module"
)

func matchInstance(def module.Definition, inst module.Instance) error {
	if err := matchServices(def.Name, def.Services, inst.Services); err != nil {
		return err
	}
	return matchHTTP(def.Name, def.HTTP, inst.HTTP)
}

func matchServices(moduleName string, declared, actual []module.ConnectService) error {
	if len(declared) != len(actual) {
		return fieldError(moduleName, "Services", fmt.Sprintf("instance has %d services, declaration has %d", len(actual), len(declared)))
	}
	used := make([]bool, len(actual))
	for _, want := range declared {
		wantName := string(want.Descriptor.FullName())
		found := -1
		for i, got := range actual {
			if used[i] || got.Descriptor == nil {
				continue
			}
			if string(got.Descriptor.FullName()) == wantName {
				found = i
				break
			}
		}
		if found < 0 {
			return fieldError(moduleName, "Services", fmt.Sprintf("missing instance service %s", wantName))
		}
		used[found] = true
		got := actual[found]
		if got.Register == nil {
			return fieldError(moduleName, "Services", fmt.Sprintf("%s Register is nil", wantName))
		}
		if err := matchServiceDescriptors(moduleName, want.Descriptor, got.Descriptor); err != nil {
			return err
		}
	}
	return nil
}

func matchServiceDescriptors(moduleName string, want, got protoreflect.ServiceDescriptor) error {
	if want.Methods().Len() != got.Methods().Len() {
		return fieldError(moduleName, "Services", fmt.Sprintf("%s method count mismatch", want.FullName()))
	}
	methods := want.Methods()
	for i := 0; i < methods.Len(); i++ {
		wm := methods.Get(i)
		gm := got.Methods().ByName(wm.Name())
		if gm == nil {
			return fieldError(moduleName, "Services", fmt.Sprintf("%s missing method %s", want.FullName(), wm.Name()))
		}
		if wm.Input().FullName() != gm.Input().FullName() || wm.Output().FullName() != gm.Output().FullName() {
			return fieldError(moduleName, "Services", fmt.Sprintf("%s/%s input or output type mismatch", want.FullName(), wm.Name()))
		}
		if wm.IsStreamingClient() != gm.IsStreamingClient() || wm.IsStreamingServer() != gm.IsStreamingServer() {
			return fieldError(moduleName, "Services", fmt.Sprintf("%s/%s streaming mismatch", want.FullName(), wm.Name()))
		}
	}
	return nil
}

func matchHTTP(moduleName string, declared []module.HTTPRoute, actual []module.HTTPHandler) error {
	if len(declared) != len(actual) {
		return fieldError(moduleName, "HTTP", fmt.Sprintf("instance has %d handlers, declaration has %d", len(actual), len(declared)))
	}
	used := make(map[string]struct{}, len(actual))
	index := make(map[string]module.HTTPHandler, len(actual))
	for _, handler := range actual {
		if handler.Name == "" {
			return fieldError(moduleName, "HTTP", "instance handler is missing Name")
		}
		if handler.Handler == nil {
			return fieldError(moduleName, "HTTP", fmt.Sprintf("handler %q is nil", handler.Name))
		}
		if _, ok := used[handler.Name]; ok {
			return fieldError(moduleName, "HTTP", fmt.Sprintf("duplicate handler %q", handler.Name))
		}
		used[handler.Name] = struct{}{}
		index[handler.Name] = handler
	}
	for _, route := range declared {
		if _, ok := index[route.Name]; !ok {
			return fieldError(moduleName, "HTTP", fmt.Sprintf("missing handler %q", route.Name))
		}
	}
	return nil
}
