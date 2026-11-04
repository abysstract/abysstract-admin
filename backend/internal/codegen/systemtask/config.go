package systemtask

import (
	"github.com/abysstract/abysstract-admin/backend/internal/resource/coredefs"
	"github.com/abysstract/abysstract-admin/backend/resource"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
	"github.com/abysstract/abysstract-admin/backend/resourcegen"
)

func GeneratorConfig(root string) resourcegen.Config {
	return resourcegen.Config{
		FormatVersion:  resource.GeneratorFormatV1,
		GenQueryImport: "github.com/abysstract/abysstract-admin/backend/internal/dal/query",
		RootDir:        root,
		GoOutputDir:    "backend/internal/resource/gen",
		GoPackage:      "gen",
		TSOutputDir:    "frontend/packages/admin-core/src/gen/resource",
		GoImports: resourcegen.GoImports{
			Resource:   "github.com/abysstract/abysstract-admin/backend/resource",
			Permission: "github.com/abysstract/abysstract-admin/backend/permission",
		},
		TSImports: resourcegen.TSImports{
			Schema:       "@abysstract/admin-core/resource/schema",
			Operations:   "@abysstract/admin-core/resource/operations",
			Relation:     "@abysstract/admin-core/resource/relation",
			ProtoModules: resourcegen.SystemProtoModules(),
		},
	}

}

func Resources() []*schema.Resource { return coredefs.Resources() }
