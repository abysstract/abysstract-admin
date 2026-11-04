package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 正式 generated Dict service 必须依赖 formal generator output，且不得回退到 spike runtime。
func TestGeneratedDictServiceUsesFormalGeneratedConverters(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("dict_generated.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, `resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"`) {
		t.Fatal("generated Dict service must import formal internal/resource/gen as resourcegen")
	}
	if strings.Contains(text, `"github.com/abysstract/abysstract-admin/backend/internal/resource/spike/gen"`) {
		t.Fatal("generated Dict service must not retain a spike/gen runtime dependency")
	}
	if strings.Contains(text, "spikegen.") {
		t.Fatal("generated Dict service must use resourcegen, not spikegen")
	}
}
