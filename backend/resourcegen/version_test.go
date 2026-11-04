package resourcegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/coredefs"
	"github.com/abysstract/abysstract-admin/backend/resource"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestSupportedFormatVersionGeneratesAndCompiles(t *testing.T) {
	cfg, root := testRootConfig(t)
	cfg.GeneratorVersion = "dev+tool.1"
	if err := Generate(cfg, []*schema.Resource{coredefs.ConfigResource}); err != nil {
		t.Fatal(err)
	}
	goFile := filepath.Join(root, cfg.GoOutputDir, "format.gen.go")
	got, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "var _ uint32 = resource.GeneratorFormatV1") {
		t.Fatalf("generated format file missing runtime symbol:\n%s", text)
	}
	if !strings.Contains(text, "// FormatVersion: 1") {
		t.Fatalf("generated format file missing format header:\n%s", text)
	}
}

func TestUnknownFormatVersionFailsBeforeWrite(t *testing.T) {
	cfg, root := testRootConfig(t)
	cfg.FormatVersion = 99
	if err := Generate(cfg, []*schema.Resource{coredefs.ConfigResource}); err == nil || !strings.Contains(err.Error(), "unsupported FormatVersion") {
		t.Fatalf("unknown format error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, cfg.GoOutputDir)); !os.IsNotExist(err) {
		t.Fatal("unknown format must not write")
	}
}

func TestGeneratedHeadersRecordTraceableIdentity(t *testing.T) {
	cfg, root := testRootConfig(t)
	cfg.GeneratorVersion = "0.0.0-dev.0"
	cfg.SourceIdentity = "fixed-source:9d15c373864d949fec223ea6978cf910fb377c87a57f763a6e74978586c02cfa"
	if err := Generate(cfg, []*schema.Resource{coredefs.ConfigResource}); err != nil {
		t.Fatal(err)
	}
	files := []string{
		filepath.Join(root, cfg.GoOutputDir, "format.gen.go"),
		filepath.Join(root, cfg.TSOutputDir, "generator-format.gen.ts"),
		filepath.Join(root, cfg.GoOutputDir, "config_conv.gen.go"),
		filepath.Join(root, cfg.TSOutputDir, "config.schema.gen.ts"),
	}
	for _, file := range files {
		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(got)
		for _, want := range []string{
			"// GeneratorVersion: 0.0.0-dev.0",
			"// FormatVersion: 1",
			"// SourceIdentity: fixed-source:9d15c373864d949fec223ea6978cf910fb377c87a57f763a6e74978586c02cfa",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q\n%s", file, want, text)
			}
		}
	}
}

func TestSameSourceGenerationIsDeterministic(t *testing.T) {
	cfg, root := testRootConfig(t)
	cfg.GeneratorVersion = "dev+tool.1"
	cfg.SourceIdentity = "source-a"
	resources := []*schema.Resource{coredefs.ConfigResource}
	if err := Generate(cfg, resources); err != nil {
		t.Fatal(err)
	}
	firstGO, err := os.ReadFile(filepath.Join(root, cfg.GoOutputDir, "config_conv.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	firstTS, err := os.ReadFile(filepath.Join(root, cfg.TSOutputDir, "config.schema.gen.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Generate(cfg, resources); err != nil {
		t.Fatal(err)
	}
	secondGO, err := os.ReadFile(filepath.Join(root, cfg.GoOutputDir, "config_conv.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	secondTS, err := os.ReadFile(filepath.Join(root, cfg.TSOutputDir, "config.schema.gen.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if string(firstGO) != string(secondGO) {
		t.Fatal("repeated Go generation changed content")
	}
	if string(firstTS) != string(secondTS) {
		t.Fatal("repeated TS generation changed content")
	}
}

func TestToolVersionChangeKeepsFormatCompatible(t *testing.T) {
	cfg, root := testRootConfig(t)
	resources := []*schema.Resource{coredefs.ConfigResource}
	cfg.GeneratorVersion = "dev+tool.1"
	if err := Generate(cfg, resources); err != nil {
		t.Fatal(err)
	}
	cfg.GeneratorVersion = "dev+tool.2"
	if err := Generate(cfg, resources); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, cfg.GoOutputDir, "format.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "// GeneratorVersion: dev+tool.2") {
		t.Fatalf("tool version was not updated:\n%s", text)
	}
	if !strings.Contains(text, "// FormatVersion: 1") {
		t.Fatalf("format version must stay 1:\n%s", text)
	}
	if !strings.Contains(text, "resource.GeneratorFormatV1") {
		t.Fatal("runtime format symbol must remain")
	}
}

func TestOfficialVersionWithoutSourceIsMarkedDevelopment(t *testing.T) {
	cfg, root := testRootConfig(t)
	cfg.GeneratorVersion = "1.2.3"
	cfg.SourceIdentity = ""
	t.Setenv(generatorVersionEnv, "")
	t.Setenv(generatorSourceEnv, "")
	if err := Generate(cfg, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, cfg.GoOutputDir, "format.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "// GeneratorVersion: dev") {
		t.Fatalf("missing source must mark development:\n%s", text)
	}
	if strings.Contains(text, "// GeneratorVersion: 1.2.3") {
		t.Fatal("official version must not be written without source identity")
	}
	if strings.Contains(text, "SourceIdentity:") {
		t.Fatal("development header must omit empty source identity")
	}
}

func TestResolveGeneratorIdentityPrefersConfigThenEnv(t *testing.T) {
	t.Setenv(generatorVersionEnv, "env-version")
	t.Setenv(generatorSourceEnv, "env-source")
	version, source := resolveGeneratorIdentity(Config{GeneratorVersion: "cfg-version", SourceIdentity: "cfg-source"})
	if version != "cfg-version" || source != "cfg-source" {
		t.Fatalf("config identity = %q %q", version, source)
	}
	version, source = resolveGeneratorIdentity(Config{})
	if version != "env-version" || source != "env-source" {
		t.Fatalf("env identity = %q %q", version, source)
	}
}

func TestIsDevelopmentVersion(t *testing.T) {
	for _, version := range []string{"dev", "DEV", "vdev", "dev+fix.1", "0.0.0-dev.0", "v0.0.0-dev.1"} {
		if !isDevelopmentVersion(version) {
			t.Fatalf("%q should be development", version)
		}
	}
	for _, version := range []string{"1.0.0", "v1.2.3", "0.1.0"} {
		if isDevelopmentVersion(version) {
			t.Fatalf("%q should be official", version)
		}
	}
}

func TestFormatVersionNotConfusedWithToolVersion(t *testing.T) {
	if resource.GeneratorFormatV1 != 1 {
		t.Fatalf("GeneratorFormatV1 = %d", resource.GeneratorFormatV1)
	}
	if isDevelopmentVersion("1.0.0") {
		t.Fatal("runtime library version 1.0.0 is not a format incompatibility")
	}
}
