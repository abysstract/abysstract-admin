package coremigrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestSyncSources(t *testing.T) {
	dir := t.TempDir()
	first := fstest.MapFS{
		"20261001090001_tables.sql":   &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT 1;\n")},
		"20261001090002_seed.go.tmpl": &fstest.MapFile{Data: []byte("package migrations\n")},
	}
	files, err := syncSources(dir, first)
	if err != nil || len(files) != 2 {
		t.Fatalf("first sync: files=%v err=%v", files, err)
	}
	if !strings.HasSuffix(files[0], "_core_20261001090001_tables.sql") || !strings.HasSuffix(files[1], "_core_20261001090002_seed.go") || filepath.Base(files[0])[:14] >= filepath.Base(files[1])[:14] {
		t.Fatalf("source order or extensions: %v", files)
	}
	if err := os.WriteFile(files[0], []byte("project customization"), 0644); err != nil {
		t.Fatal(err)
	}
	customized := files[0]
	files, err = syncSources(dir, first)
	if err != nil || len(files) != 0 {
		t.Fatalf("repeat sync: files=%v err=%v", files, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "20261021000000_product_tables.sql"), []byte("business"), 0644); err != nil {
		t.Fatal(err)
	}
	upgrade := fstest.MapFS{
		"20261001090001_tables.sql":   first["20261001090001_tables.sql"],
		"20261001090002_seed.go.tmpl": first["20261001090002_seed.go.tmpl"],
		"20261001090003_change.sql":   &fstest.MapFile{Data: []byte("third")},
		"20261001090004_next.sql":     &fstest.MapFile{Data: []byte("fourth")},
	}
	files, err = syncSources(dir, upgrade)
	if err != nil || len(files) != 2 {
		t.Fatalf("skip-version sync: files=%v err=%v", files, err)
	}
	if filepath.Base(files[0])[:14] <= "20261021000000" || filepath.Base(files[0])[:14] >= filepath.Base(files[1])[:14] {
		t.Fatalf("upgrade versions: %v", files)
	}
	content, err := os.ReadFile(filepath.Join(dir, filepath.Base(files[0])))
	if err != nil || string(content) != "third" {
		t.Fatalf("new source content: %q %v", content, err)
	}
	content, err = os.ReadFile(filepath.Join(dir, "20261021000000_product_tables.sql"))
	if err != nil || string(content) != "business" {
		t.Fatalf("business migration changed: %q %v", content, err)
	}
	content, err = os.ReadFile(customized)
	if err != nil || string(content) != "project customization" {
		t.Fatalf("customization changed: %q %v", content, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 5 {
		t.Fatalf("directory: %v %v", entries, err)
	}
}

func TestSyncSourcesWithHyphenatedLocalNames(t *testing.T) {
	dir := t.TempDir()
	businessVersion := time.Now().UTC().Add(-time.Minute).Format("20060102150405")
	business := filepath.Join(dir, businessVersion+"_create-product.sql")
	if err := os.WriteFile(business, []byte("business"), 0644); err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(dir, "20261020090001_core_20261001090001_local-custom.sql")
	if err := os.WriteFile(renamed, []byte("custom"), 0644); err != nil {
		t.Fatal(err)
	}
	sources := fstest.MapFS{
		"20261001090001_tables.sql": &fstest.MapFile{Data: []byte("original")},
		"20261001090002_next.sql":   &fstest.MapFile{Data: []byte("next")},
	}
	files, err := syncSources(dir, sources)
	if err != nil || len(files) != 1 {
		t.Fatalf("sync: files=%v err=%v", files, err)
	}
	if !strings.HasSuffix(files[0], "_core_20261001090002_next.sql") || filepath.Base(files[0])[:14] <= businessVersion {
		t.Fatalf("migration must follow hyphenated business file: %v", files)
	}
	for path, want := range map[string]string{business: "business", renamed: "custom"} {
		content, err := os.ReadFile(path)
		if err != nil || string(content) != want {
			t.Fatalf("existing file %s changed: %q %v", path, content, err)
		}
	}
	files, err = syncSources(dir, sources)
	if err != nil || len(files) != 0 {
		t.Fatalf("repeat sync: files=%v err=%v", files, err)
	}
}

func TestSyncSourcesRejectsFutureHyphenatedBusinessVersion(t *testing.T) {
	dir := t.TempDir()
	version := time.Now().UTC().Add(time.Minute).Format("20060102150405")
	if err := os.WriteFile(filepath.Join(dir, version+"_create-product.sql"), []byte("business"), 0644); err != nil {
		t.Fatal(err)
	}
	files, err := syncSources(dir, fstest.MapFS{
		"20261001090001_tables.sql": &fstest.MapFile{Data: []byte("core")},
	})
	if err == nil || !strings.Contains(err.Error(), "in the future") || len(files) != 0 {
		t.Fatalf("future version must stop sync: files=%v err=%v", files, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("sync wrote before future version: %v %v", entries, err)
	}
}

func TestProductSyncOfflineAndGoVersion(t *testing.T) {
	for _, driver := range []string{"postgres", "mysql", "mariadb", "sqlite"} {
		t.Run(driver, func(t *testing.T) {
			root := t.TempDir()
			files, err := Sync(root, driver)
			if err != nil || len(files) != 3 {
				t.Fatalf("sync: %v %v", files, err)
			}
			for _, name := range files {
				if filepath.Dir(name) != filepath.Join(root, driver) {
					t.Fatal(name)
				}
				data, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasSuffix(name, ".go") && (!strings.Contains(string(data), "goose.NewGoMigration("+filepath.Base(name)[:14]) || strings.Contains(string(data), "__CORE_VERSION__")) {
					t.Fatalf("local version: %s", data)
				}
			}
			if err := os.WriteFile(files[0], []byte("local customization"), 0644); err != nil {
				t.Fatal(err)
			}
			if added, err := Sync(root, driver); err != nil || len(added) != 0 {
				t.Fatalf("repeat: %v %v", added, err)
			}
			data, err := os.ReadFile(files[0])
			if err != nil || string(data) != "local customization" {
				t.Fatalf("overwrite: %q %v", data, err)
			}
			if _, err := Sync(root, "unknown"); err == nil {
				t.Fatal("unsupported product")
			}

		})
	}
}
