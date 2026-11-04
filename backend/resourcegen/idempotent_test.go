package resourcegen

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/coredefs"
)

func TestGenerateTwiceIsIdempotent(t *testing.T) {
	cfg, _ := testRootConfig(t)
	if err := Generate(cfg, coredefs.Resources()); err != nil {
		t.Fatal(err)
	}
	first := snapshotDir(t, filepath.Join(cfg.RootDir, cfg.GoOutputDir))
	firstTS := snapshotDir(t, filepath.Join(cfg.RootDir, cfg.TSOutputDir))
	if err := Generate(cfg, coredefs.Resources()); err != nil {
		t.Fatal(err)
	}
	second := snapshotDir(t, filepath.Join(cfg.RootDir, cfg.GoOutputDir))
	secondTS := snapshotDir(t, filepath.Join(cfg.RootDir, cfg.TSOutputDir))
	assertSameSnapshot(t, first, second)
	assertSameSnapshot(t, firstTS, secondTS)
}

func snapshotDir(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[entry.Name()] = data
	}
	return out
}

func assertSameSnapshot(t *testing.T, a, b map[string][]byte) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("file count %d != %d", len(a), len(b))
	}
	for name, data := range a {
		other, ok := b[name]
		if !ok {
			t.Fatalf("missing %s on second generate", name)
		}
		if !bytes.Equal(data, other) {
			t.Fatalf("second generate changed %s", name)
		}
	}
}
