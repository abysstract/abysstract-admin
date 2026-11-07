package module

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// appFixture is a separate application: its own module path, the framework
// system module, a frontend at frontend/src and MySQL migrations only.
func appFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string][]byte{
		"go.mod":                          []byte("module example.com/shop\n\ngo 1.26.0\n"),
		"admin.modules.json":              []byte(`{"frontendRoot": "frontend/src", "modules": ["system"]}` + "\n"),
		"backend/migrations/mysql/doc.go": []byte("// Package mysql holds MySQL Go migrations.\npackage mysql\n"),
		"backend/migrations/mysql/20261001090001_core_init.sql": []byte("-- +goose Up\n"),
	}
	if _, err := writeFiles(root, files, false); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLayoutDerivesApplicationImport(t *testing.T) {
	for _, tc := range []struct{ file, module, want string }{
		{"go.mod", "example.com/shop", "example.com/shop/backend/"},
		{"backend/go.mod", "example.com/shop/backend", "example.com/shop/backend/"},
		{"go.mod", "github.com/abysstract/abysstract-admin", frameworkImport},
	} {
		root := t.TempDir()
		if _, err := writeFiles(root, map[string][]byte{tc.file: []byte("module " + tc.module + "\n")}, false); err != nil {
			t.Fatal(err)
		}
		layout, err := loadLayout(root)
		if err != nil {
			t.Fatal(err)
		}
		if layout.Import != tc.want || layout.FrontendRoot != defaultFrontendRoot || layout.Framework {
			t.Fatalf("%s %s: %+v", tc.file, tc.module, layout)
		}
	}
	root := appFixture(t)
	if err := os.WriteFile(filepath.Join(root, "admin.modules.json"), []byte(`{"frontendRoot": "../web", "modules": ["system"]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadLayout(root); err == nil {
		t.Fatal("frontendRoot outside the repository accepted")
	}
}

func TestApplicationModuleUsesApplicationPaths(t *testing.T) {
	root := appFixture(t)
	if _, err := execute(t, root, "sync"); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, root, "new", "notes"); err != nil {
		t.Fatal(err)
	}
	base := "backend/internal/modules/notes"
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if config := read(base + "/definition/codegen.go"); !strings.Contains(config, `GenQueryImport: "example.com/shop/backend/internal/modules/notes/query"`) || !strings.Contains(config, `TSOutputDir:    "frontend/src/modules/notes/gen/resource"`) {
		t.Fatalf("codegen config: %s", config)
	}
	d, err := readDescriptor(root, base+"/module.json")
	if err != nil {
		t.Fatal(err)
	}
	if d.Frontend.Entry != "frontend/src/modules/notes/index.tsx" || d.Generation.TSOutput != "frontend/src/modules/notes/gen/proto" {
		t.Fatalf("declaration: %+v %+v", d.Frontend, d.Generation)
	}

	files, _, err := prepare(root, "add", "notes")
	if err != nil {
		t.Fatal(err)
	}
	loader := string(files["backend/internal/app/modules.gen.go"])
	for _, want := range []string{`"example.com/shop/backend/internal/modules/notes"`, `"github.com/abysstract/abysstract-admin/backend/system"`} {
		if !strings.Contains(loader, want) {
			t.Fatalf("loader lacks %s:\n%s", want, loader)
		}
	}
	pages := string(files["frontend/src/modules.gen.ts"])
	if !strings.Contains(pages, `import { coreSystemModule as m0 } from "@abysstract/admin-core/system";`) || !strings.Contains(pages, `from "./modules/notes/index";`) {
		t.Fatalf("frontend modules:\n%s", pages)
	}
	registry := string(files["backend/migrations/go_registry.gen.go"])
	if !strings.Contains(registry, `"example.com/shop/backend/migrations/mysql"`) || strings.Contains(registry, "postgres") || !strings.Contains(registry, "mysql.ModuleNotes") {
		t.Fatalf("migration registry:\n%s", registry)
	}
	for path := range files {
		if strings.HasPrefix(path, "backend/migrations/") && strings.HasSuffix(path, ".go") && !strings.HasPrefix(path, "backend/migrations/mysql/") && path != "backend/migrations/go_registry.gen.go" {
			t.Fatalf("module migration for an unused dialect: %s", path)
		}
	}
	for _, framework := range []string{"buf.yaml", "backend/internal/codegen/modules.json", "backend/internal/codegen/resources/tasks.gen.go"} {
		if _, exists := files[framework]; exists {
			t.Fatalf("framework inventory written to an application: %s", framework)
		}
	}
	if _, err := writeFiles(root, files, false); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, root, "check"); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateHonoursDeclaredPaths(t *testing.T) {
	root := appFixture(t)
	if _, err := execute(t, root, "new", "notes"); err != nil {
		t.Fatal(err)
	}
	base := "backend/internal/modules/notes"
	d, err := readDescriptor(root, base+"/module.json")
	if err != nil {
		t.Fatal(err)
	}
	// Move the frontend module and its protocol output to another layout.
	if err := os.MkdirAll(filepath.Join(root, "web/notes"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, d.Frontend.Entry), filepath.Join(root, "web/notes/index.tsx")); err != nil {
		t.Fatal(err)
	}
	d.Frontend.Entry = "web/notes/index.tsx"
	d.Generation.TSOutput = "web/protocol"
	d.Generation.GoOutput = base + "/api"
	data, err := jsonBytes(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, base, "module.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	dsn := filepath.Join(t.TempDir(), "app.sqlite")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	t.Setenv("ADMIN_DB_DRIVER", "sqlite")
	t.Setenv("ADMIN_DB_DSN", "file:"+dsn)
	if err := db.Exec("CREATE TABLE notes_note (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, root, "resource", "notes", "notes_note"); err != nil {
		t.Fatal(err)
	}
	files, err := prepareDevelopment(root, "notes")
	if err != nil {
		t.Fatal(err)
	}
	page := string(files["web/notes/pages/NoteList.tsx"])
	if !strings.Contains(page, `from "../../protocol/company/notes/v1/note_pb"`) {
		t.Fatalf("page does not follow the declared paths:\n%s", page)
	}
	for _, want := range []string{`api "example.com/shop/backend/internal/modules/notes/api/company/notes/v1"`, `"example.com/shop/backend/internal/modules/notes/model"`} {
		if !strings.Contains(string(files[base+"/definition/note.gen.go"]), want) {
			t.Fatalf("resource lacks %s:\n%s", want, files[base+"/definition/note.gen.go"])
		}
	}
	config := string(files[base+"/definition/codegen.go"])
	if !strings.Contains(config, `TSOutputDir:    "web/notes/gen/resource"`) || !strings.Contains(config, `"../../../protocol/company/notes/v1/note_pb"`) {
		t.Fatalf("codegen config:\n%s", config)
	}
	for path := range files {
		if strings.HasPrefix(path, "frontend/") {
			t.Fatalf("generated into the default frontend: %s", path)
		}
	}
}

func TestRewriteProtoImports(t *testing.T) {
	files := map[string]string{
		"company/notes/v1/note_pb.ts": strings.Join([]string{
			`import { file_abysstract_admin_v1_common } from "../../../abysstract/admin/v1/common_pb";`,
			`import { file_company_notes_v1_tag } from "./tag_pb";`,
			`import { file_company_shop_v1_item } from "../../shop/v1/item_pb.js";`,
			`import type { Timestamp } from "@bufbuild/protobuf/wkt";`,
		}, "\n"),
		"company/notes/v1/tag_pb.ts": `export {};`,
	}
	out, err := rewriteProtoImports(files,
		map[string]bool{"abysstract/admin/v1/common_pb.ts": true},
		map[string]string{"company/shop/v1/item_pb.ts": "web/shop/proto/company/shop/v1/item_pb.ts"},
		"web/notes/proto")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		`import { file_abysstract_admin_v1_common } from "@abysstract/admin-proto/abysstract/admin/v1/common_pb";`,
		`import { file_company_notes_v1_tag } from "./tag_pb.js";`,
		`import { file_company_shop_v1_item } from "../../../../../shop/proto/company/shop/v1/item_pb.js";`,
		`import type { Timestamp } from "@bufbuild/protobuf/wkt";`,
	}, "\n")
	if got := string(out["company/notes/v1/note_pb.ts"]); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if _, err := rewriteProtoImports(map[string]string{"company/notes/v1/a_pb.ts": `import { x } from "./missing_pb";`}, nil, nil, "out"); err == nil {
		t.Fatal("unknown dependency accepted")
	}
}

func TestProtoPluginsMatchFrameworkTemplate(t *testing.T) {
	data, err := os.ReadFile("../../../../proto/buf.gen.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var remotes []string
	for _, line := range strings.Split(string(data), "\n") {
		if remote, ok := strings.CutPrefix(strings.TrimSpace(line), "- remote: "); ok {
			remotes = append(remotes, remote)
		}
	}
	if len(remotes) != len(protoPlugins) {
		t.Fatalf("proto/buf.gen.yaml plugins %v differ from %v", remotes, protoPlugins)
	}
	for i, p := range protoPlugins {
		if remotes[i] != p.remote {
			t.Fatalf("plugin %d: proto/buf.gen.yaml uses %s, module generate uses %s", i, remotes[i], p.remote)
		}
	}
}
