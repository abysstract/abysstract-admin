package adminapp

import (
	"fmt"

	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/module"
	"github.com/abysstract/abysstract-admin/backend/permission"
)

type ownedRegistrar struct {
	moduleName string
	allowed    map[string]struct{}
	inner      *interceptor.PermRegistry
}

func (r *ownedRegistrar) RegisterGenerated(entries ...permission.Entry) error {
	for _, entry := range entries {
		if _, ok := r.allowed[entry.Procedure]; !ok {
			return fieldError(r.moduleName, "Permissions", fmt.Sprintf("procedure %q is not owned by this module", entry.Procedure))
		}
	}
	return r.inner.RegisterGenerated(entries...)
}

func (r *ownedRegistrar) RegisterPublic(procedures ...string) error {
	for _, procedure := range procedures {
		if _, ok := r.allowed[procedure]; !ok {
			return fieldError(r.moduleName, "Permissions", fmt.Sprintf("procedure %q is not owned by this module", procedure))
		}
	}
	return r.inner.RegisterPublic(procedures...)
}

func collectPermissions(defs []module.Definition, procedures []procedureSpec) (*interceptor.PermRegistry, error) {
	registry := interceptor.NewPermRegistry()
	ownedByModule := make(map[string]map[string]struct{}, len(defs))
	for _, spec := range procedures {
		set, ok := ownedByModule[spec.Module]
		if !ok {
			set = make(map[string]struct{})
			ownedByModule[spec.Module] = set
		}
		set[spec.Procedure] = struct{}{}
	}
	for _, def := range defs {
		if def.Permissions == nil {
			continue
		}
		registrar := &ownedRegistrar{
			moduleName: def.Name,
			allowed:    ownedByModule[def.Name],
			inner:      registry,
		}
		if registrar.allowed == nil {
			registrar.allowed = map[string]struct{}{}
		}
		if err := def.Permissions(registrar); err != nil {
			return nil, err
		}
	}
	for _, spec := range procedures {
		public, required, _ := registry.Lookup(spec.Procedure)
		switch {
		case public && required != "":
			return nil, fieldError(spec.Module, "Permissions", fmt.Sprintf("%s is both public and protected", spec.Procedure))
		case public:
			continue
		case required != "":
			continue
		default:
			return nil, fieldError(spec.Module, "Permissions", fmt.Sprintf("missing declaration for %s", spec.Procedure))
		}
	}
	if err := registry.Freeze(); err != nil {
		return nil, err
	}
	return registry, nil
}
