package resourcegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/coredefs"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestGenerateWriteFailureReportsFileAndAllowsRerun(t *testing.T) {
	cfg, root := testRootConfig(t)
	resources := []*schema.Resource{coredefs.ConfigResource}
	fsys := &faultFS{base: osFS{}, failWrite: 2}
	if err := generate(cfg, resources, fsys); err == nil || !strings.Contains(err.Error(), "write") {
		t.Fatalf("write failure = %v", err)
	}
	if err := generate(cfg, resources, osFS{}); err != nil {
		t.Fatalf("rerun after write failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, cfg.GoOutputDir, "config_conv.gen.go")); err != nil {
		t.Fatal("rerun must restore generated files")
	}
}

func TestGenerateCleanupFailureReportsFile(t *testing.T) {
	cfg, root := testRootConfig(t)
	if err := generate(cfg, []*schema.Resource{coredefs.ConfigResource}, osFS{}); err != nil {
		t.Fatal(err)
	}
	goDir := filepath.Join(root, cfg.GoOutputDir)
	orphan := filepath.Join(goDir, "orphan.gen.go")
	if err := os.WriteFile(orphan, []byte(generatedHeader+"\npackage gen\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(goDir, ".adminresource-go.manifest")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, append(data, []byte("orphan.gen.go\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	fsys := &faultFS{base: osFS{}, failRemove: 1}
	if err := generate(cfg, []*schema.Resource{coredefs.ConfigResource}, fsys); err == nil || !strings.Contains(err.Error(), "remove") {
		t.Fatalf("cleanup failure = %v", err)
	}
	if err := generate(cfg, []*schema.Resource{coredefs.ConfigResource}, osFS{}); err != nil {
		t.Fatalf("rerun after cleanup failure: %v", err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("rerun must clean orphan")
	}
}

func TestMissingTSMappingFailsBeforeWrite(t *testing.T) {
	cfg, root := testRootConfig(t)
	cfg.TSImports.ProtoModules = map[string]string{}
	if err := Generate(cfg, []*schema.Resource{coredefs.ConfigResource}); err == nil || !strings.Contains(err.Error(), "missing TS mapping") {
		t.Fatalf("mapping error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, cfg.GoOutputDir)); !os.IsNotExist(err) {
		t.Fatal("mapping failure must not write")
	}
}

func TestInternalPermissionImportRejected(t *testing.T) {
	cfg, _ := testRootConfig(t)
	cfg.GoImports.Permission = "github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	if err := Generate(cfg, nil); err == nil || !strings.Contains(err.Error(), "public import path") {
		t.Fatalf("internal import error = %v", err)
	}
}

func TestMissingGenQueryImportFailsBeforeWrite(t *testing.T) {
	cfg, root := testRootConfig(t)
	cfg.GenQueryImport = " "
	if err := Generate(cfg, []*schema.Resource{coredefs.ConfigResource}); err == nil || !strings.Contains(err.Error(), "GenQueryImport is required") {
		t.Fatalf("query import error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, cfg.GoOutputDir)); !os.IsNotExist(err) {
		t.Fatal("invalid config must fail before writing")
	}
}
