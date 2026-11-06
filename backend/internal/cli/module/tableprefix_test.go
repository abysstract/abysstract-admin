package module

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setPrefix rewrites one module declaration's tablePrefix in a fixture.
func setPrefix(t *testing.T, root, path, prefix string) {
	t.Helper()
	d, err := readDescriptor(root, path)
	if err != nil {
		t.Fatal(err)
	}
	d.Generation.TablePrefix = prefix
	data, err := jsonBytes(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckRejectsOverlappingTablePrefixes(t *testing.T) {
	root := fixture(t)
	if _, err := execute(t, root, "sync"); err != nil {
		t.Fatal(err)
	}
	// product_ nested under system's namespace selects the other module's tables.
	setPrefix(t, root, "backend/system/module.json", "product")
	_, err := execute(t, root, "check")
	if err == nil || !strings.Contains(err.Error(), `module product tablePrefix "product_" overlaps module system tablePrefix "product"`) {
		t.Fatalf("overlapping installed prefixes accepted: %v", err)
	}
	setPrefix(t, root, "backend/system/module.json", "product_core_")
	if _, err := execute(t, root, "check"); err == nil || !strings.Contains(err.Error(), "overlaps") {
		t.Fatalf("nested installed prefixes accepted: %v", err)
	}
	setPrefix(t, root, "backend/system/module.json", "system")
	if _, err := execute(t, root, "sync"); err != nil {
		t.Fatalf("prefix without trailing underscore: %v", err)
	}
	if _, err := execute(t, root, "check"); err != nil {
		t.Fatalf("prefix without trailing underscore: %v", err)
	}
	setPrefix(t, root, "backend/system/module.json", "Bad-")
	if _, err := execute(t, root, "check"); err == nil || !strings.Contains(err.Error(), "snake_case") {
		t.Fatalf("invalid prefix accepted: %v", err)
	}
}

func TestNewRejectsTablePrefixOfExistingModule(t *testing.T) {
	root := fixture(t)
	setPrefix(t, root, "backend/system/module.json", "sys_")
	// A business module named sys would own sys_* tables of the system module.
	_, err := execute(t, root, "new", "sys")
	if err == nil || !strings.Contains(err.Error(), `module sys tablePrefix "sys_" overlaps module system tablePrefix "sys_"`) {
		t.Fatalf("new accepted the system namespace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "backend/internal/modules/sys")); !os.IsNotExist(err) {
		t.Fatalf("rejected new wrote files: %v", err)
	}
	// Uninstalled module declarations are checked too.
	if _, err := execute(t, root, "new", "shop"); err != nil {
		t.Fatal(err)
	}
	setPrefix(t, root, "backend/internal/modules/shop/module.json", "shop_order_")
	if _, err := execute(t, root, "new", "shoporder"); err != nil {
		t.Fatalf("disjoint default prefix rejected: %v", err)
	}
	// Without a trailing underscore "shop" still stops at the boundary and leaves shopx_* alone.
	setPrefix(t, root, "backend/internal/modules/shop/module.json", "shop")
	if _, err := execute(t, root, "new", "shopx"); err != nil {
		t.Fatalf("boundary-separated prefix rejected: %v", err)
	}
}

func TestGenerateRejectsTargetOverlappingInstalledModule(t *testing.T) {
	root := fixture(t)
	if _, err := execute(t, root, "new", "shop"); err != nil {
		t.Fatal(err)
	}
	// The explicit target is not installed yet; generation still checks it before reading the database.
	setPrefix(t, root, "backend/internal/modules/shop/module.json", "product_shop_")
	_, err := prepareDevelopment(root, "shop")
	if err == nil || !strings.Contains(err.Error(), `module product tablePrefix "product_" overlaps module shop tablePrefix "product_shop_"`) {
		t.Fatalf("generate accepted an overlapping target: %v", err)
	}
	if _, err := developmentGenerationSteps(root, "shop", t.TempDir()); err == nil || !strings.Contains(err.Error(), "overlaps") {
		t.Fatalf("generation inventory accepted an overlapping target: %v", err)
	}
}
