package adminapp

import (
	"fmt"

	"github.com/abysstract/abysstract-admin/backend/module"
)

func cloneDefinitions(in []module.Definition) []module.Definition {
	out := make([]module.Definition, len(in))
	for i, def := range in {
		out[i] = module.Definition{
			Name:        def.Name,
			Requires:    append([]string(nil), def.Requires...),
			Services:    append([]module.ConnectService(nil), def.Services...),
			Permissions: def.Permissions,
			HTTP:        cloneHTTP(def.HTTP),
			Create:      def.Create,
		}
	}
	return out
}

func cloneHTTP(in []module.HTTPRoute) []module.HTTPRoute {
	out := make([]module.HTTPRoute, len(in))
	copy(out, in)
	return out
}

func fieldError(moduleName, field, message string) error {
	if moduleName == "" {
		return fmt.Errorf("adminapp: %s: %s", field, message)
	}
	return fmt.Errorf("adminapp: module %q field %s: %s", moduleName, field, message)
}
