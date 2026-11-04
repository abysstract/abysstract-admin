package module

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/moduleseed"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string][]byte{
		"backend/go.mod": []byte("module " + strings.TrimSuffix(backendImport, "/") + "\n"),
		"buf.base.yaml":  []byte("version: v2\nlint:\n  use: [STANDARD]\n"),
	}
	selected := selection{Modules: []string{"backend/system/module.json", "backend/internal/modules/product/module.json"}}
	var err error
	files["admin.modules.json"], err = jsonBytes(selected)
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"system", "product"} {
		backend := "backend/internal/modules/" + name
		if name == "system" {
			backend = "backend/system"
		}
		d := descriptor{Name: name, Backend: entry{Package: backend, Constructor: "NewModule"}, Frontend: frontend{Entry: "frontend/apps/reference/src/modules/" + name + "/index.tsx", Export: name + "Module"}, Generation: &generation{Name: name, TablePrefix: name + "_", ModelOutput: backend, Prefix: "company/" + name + "/v1/", GoOutput: backend + "/gen/proto", TSOutput: "frontend/apps/reference/src/modules/" + name + "/gen/proto", ProtoSource: backend + "/proto"}, Resource: &resourceSpec{Package: backend + "/definition", Config: "GeneratorConfig", Resources: "Resources"}}
		if name == "product" {
			d.Requires = []string{"system"}
		}
		for _, dir := range []string{backend, backend + "/proto", backend + "/definition"} {
			if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
				t.Fatal(err)
			}
		}
		files[selected.Modules[i]], err = jsonBytes(d)
		if err != nil {
			t.Fatal(err)
		}
		files[d.Frontend.Entry] = []byte("export {};\n")
	}
	snapshot := seedSnapshot{Action: "install", Seed: moduleseed.Spec{Module: "product", Menus: []moduleseed.Menu{{Key: "product.root", Name: "Product", Type: "M"}}}}
	files["backend/migrations/modules/20261102090001_product_install.json"], err = jsonBytes(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, driver := range []string{"postgres", "mysql", "mariadb", "sqlite"} {
		files["backend/migrations/"+driver+"/20261020090002_seed.go"] = []byte("package " + driver + "\nimport \"github.com/pressly/goose/v3\"\nfunc SeedAdminMigration() *goose.Migration { return nil }\n")
	}
	if _, err := writeFiles(root, files, false); err != nil {
		t.Fatal(err)
	}
	return root
}

func execute(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	cmd := NewCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs(append([]string{"--root", root}, args...))
	err := cmd.Execute()
	return output.String(), err
}

func TestModuleLifecycle(t *testing.T) {
	root := fixture(t)
	if _, err := execute(t, root, "sync"); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, root, "check"); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, root, "new", "orders"); err != nil {
		t.Fatal(err)
	}
	d, err := readDescriptor(root, "backend/internal/modules/orders/module.json")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "orders" || d.Frontend.Export != "ordersModule" || d.Generation == nil || d.Resource == nil || d.Seed == "" {
		t.Fatalf("unexpected declaration: %+v", d)
	}
	if _, err := executePreparation(t, root, "add", "orders", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "admin.modules.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "orders") {
		t.Fatal("dry-run changed selection")
	}
	if _, err := executePreparation(t, root, "add", "orders"); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, root, "check"); err != nil {
		t.Fatal(err)
	}
	// An explicitly declared dependent prevents removal before any files change.
	d.Requires = []string{"system", "product"}
	data, err = jsonBytes(d)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "backend/internal/modules/orders/module.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := executePreparation(t, root, "remove", "product"); err == nil || !strings.Contains(err.Error(), "requires product") {
		t.Fatalf("expected dependency rejection, got %v", err)
	}
	if _, err := executePreparation(t, root, "remove", "orders"); err != nil {
		t.Fatal(err)
	}
	// Cleanup factories embed snapshots and survive removal of module source.
	if _, err := executePreparation(t, root, "remove", "product"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "backend/internal/modules/product")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "frontend/apps/reference/src/modules/product")); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, root, "sync"); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, root, "check"); err != nil {
		t.Fatal(err)
	}
	backend, err := os.ReadFile(filepath.Join(root, "backend/internal/app/modules.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(backend), "product") {
		t.Fatal("removed backend import remains")
	}
	files, err := filepath.Glob(filepath.Join(root, "backend/migrations/postgres/*_module_product_remove.go"))
	if err != nil || len(files) != 1 {
		t.Fatalf("cleanup migration missing: %v %v", files, err)
	}
	cleanup, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cleanup), "internal/modules/product") {
		t.Fatal("history depends on removed source")
	}
}

func TestCheckDetectsDriftAndSyncRepairs(t *testing.T) {
	root := fixture(t)
	if _, err := execute(t, root, "sync"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "frontend/apps/reference/src/modules.gen.ts")
	if err := os.WriteFile(path, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	output, err := execute(t, root, "check")
	if err == nil || !strings.Contains(output, "modules.gen.ts") {
		t.Fatalf("expected drift: %s %v", output, err)
	}
	if _, err := execute(t, root, "sync"); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, root, "check"); err != nil {
		t.Fatal(err)
	}
}

func TestRejectInvalidNameAndExistingScaffold(t *testing.T) {
	root := fixture(t)
	if _, err := execute(t, root, "new", "../escape"); err == nil {
		t.Fatal("accepted invalid name")
	}
	if _, err := execute(t, root, "new", "product"); err == nil {
		t.Fatal("overwrote existing module")
	}
}

// Preparation tests exercise file planning independently of migration execution.
func executePreparation(t *testing.T, root, action, name string, flags ...string) (string, error) {
	t.Helper()
	files, notes, err := prepare(root, action, name)
	if err != nil {
		return "", err
	}
	dry := len(flags) == 1 && flags[0] == "--dry-run"
	paths, err := writeFiles(root, files, dry)
	return strings.Join(append(paths, notes...), "\n"), err
}

func TestUpgradeReusesSnapshotWithEmptyOptionalRoles(t *testing.T) {
	root := fixture(t)
	if _, err := execute(t, root, "new", "notes"); err != nil {
		t.Fatal(err)
	}
	seed := filepath.Join(root, "backend/internal/modules/notes/seed.json")
	if err := os.WriteFile(seed, []byte(`{"module":"notes","menus":[],"roles":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := executePreparation(t, root, "add", "notes"); err != nil {
		t.Fatal(err)
	}
	d, err := readDescriptor(root, "backend/internal/modules/notes/module.json")
	if err != nil {
		t.Fatal(err)
	}
	files, err := prepareSeed(root, "upgrade", d)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatal("unchanged declaration created another migration")
	}
}
