package productdef

import (
	"github.com/abysstract/abysstract-admin/backend/resource"
	"github.com/abysstract/abysstract-admin/backend/resourcegen"
)

// GeneratorConfig owns the product Resource outputs and imports.
func GeneratorConfig(root string) resourcegen.Config {
	return resourcegen.Config{
		FormatVersion:  resource.GeneratorFormatV1,
		GenQueryImport: "github.com/abysstract/abysstract-admin/backend/internal/modules/product/query",
		RootDir:        root,
		GoOutputDir:    "backend/internal/modules/product/gen/resource",
		GoPackage:      "gen",
		TSOutputDir:    "frontend/apps/reference/src/modules/product/gen/resource",
		GoImports: resourcegen.GoImports{
			Resource:   "github.com/abysstract/abysstract-admin/backend/resource",
			Permission: "github.com/abysstract/abysstract-admin/backend/permission",
		},
		TSImports: resourcegen.TSImports{
			Schema:     "@abysstract/admin-core/resource/schema",
			Operations: "@abysstract/admin-core/resource/operations",
			Relation:   "@abysstract/admin-core/resource/relation",
			ProtoModules: map[string]string{
				"company/product/v1/product.proto": "../proto/company/product/v1/product_pb",
				"abysstract/admin/v1/common.proto": "@abysstract/admin-proto/abysstract/admin/v1/common_pb",
			},
		},
	}
}
