package resourcegen

import "github.com/abysstract/abysstract-admin/backend/resource"

func fixtureConfig() Config {
	return Config{
		FormatVersion:  resource.GeneratorFormatV1,
		GenQueryImport: "github.com/abysstract/abysstract-admin/backend/internal/resource/schematest/query",
		RootDir:        "/tmp/admin-resourcegen-fixture-root",
		GoOutputDir:    "go",
		GoPackage:      "gen",
		TSOutputDir:    "ts",
		GoImports: GoImports{
			Resource:   "github.com/abysstract/abysstract-admin/backend/resource",
			Permission: "github.com/abysstract/abysstract-admin/backend/permission",
		},
		TSImports: TSImports{
			Schema:       "@abysstract/admin-core/resource/schema",
			Operations:   "@abysstract/admin-core/resource/operations",
			Relation:     "@abysstract/admin-core/resource/relation",
			ProtoModules: defaultProtoModules(),
		},
	}
}

func defaultProtoModules() map[string]string {
	names := []string{
		"auth", "common", "common_export", "config", "dashboard", "dept", "dict",
		"log", "menu", "notice", "profile", "role", "upload", "user",
	}
	modules := make(map[string]string, len(names)+4)
	for _, name := range names {
		file := "abysstract/admin/v1/" + name + ".proto"
		modules[file] = "@abysstract/admin-proto/abysstract/admin/v1/" + name + "_pb"
	}
	modules["testpb/test.proto"] = "../testpb/test_pb"
	modules["testenum/a/status.proto"] = "../testenum/a/status_pb"
	modules["testenum/b/status.proto"] = "../testenum/b/status_pb"
	modules["company/contract/v1/contract.proto"] = "../company/contract/v1/contract_pb"
	return modules
}

func SystemProtoModules() map[string]string {
	modules := defaultProtoModules()
	delete(modules, "testpb/test.proto")
	delete(modules, "testenum/a/status.proto")
	delete(modules, "testenum/b/status.proto")
	delete(modules, "company/contract/v1/contract.proto")
	return modules
}
