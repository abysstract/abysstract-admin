package resourcegen

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/abysstract/abysstract-admin/backend/resource"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestRunBuildFailureDoesNotCreateOutputDirectories(t *testing.T) {
	root := t.TempDir()
	goRel, tsRel := "go-out", "ts-out"
	cfg := fixtureConfig()
	cfg.RootDir = root
	cfg.GoOutputDir = goRel
	cfg.TSOutputDir = tsRel
	if err := Generate(cfg, []*schema.Resource{{Name: "broken"}}); err == nil {
		t.Fatal("Generate must fail when BuildRegistry rejects the Registry")
	}
	for _, dir := range []string{filepath.Join(root, goRel), filepath.Join(root, tsRel)} {
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Build failure must not create output directory %q: err=%v", dir, err)
		}
	}
}

func TestRunRenderFailureDoesNotCreateOutputDirectories(t *testing.T) {
	root := t.TempDir()
	goRel, tsRel := "go-out", "ts-out"
	cfg := fixtureConfig()
	cfg.RootDir = root
	cfg.GoOutputDir = goRel
	cfg.TSOutputDir = tsRel
	plan := divergentResourcePlan(t)
	original := goConvTpl
	goConvTpl = template.Must(template.New("render-failure").Parse(`{{template "missing" .}}`))
	t.Cleanup(func() { goConvTpl = original })
	if err := Generate(cfg, []*schema.Resource{plan.Res}); err == nil {
		t.Fatal("Generate must return the injected Go render error")
	}
	for _, dir := range []string{filepath.Join(root, goRel), filepath.Join(root, tsRel)} {
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("render failure must not create output directory %q: err=%v", dir, err)
		}
	}
}

func TestUnsupportedFormatVersionFailsBeforeWrite(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	cfg.RootDir = root
	cfg.FormatVersion = resource.GeneratorFormatV1 + 1
	if err := Generate(cfg, nil); err == nil {
		t.Fatal("unsupported FormatVersion must fail")
	}
	if _, err := os.Stat(filepath.Join(root, cfg.GoOutputDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("unsupported format must not create output")
	}
}
