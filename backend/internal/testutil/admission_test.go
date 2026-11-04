package testutil

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// A zero GORM handle would panic if ResetSchema reached SQL. The child must
// instead exit via the dedicated-configuration check, before any DB operation.
func TestResetSchemaRejectsBeforeSQL(t *testing.T) {
	if os.Getenv("ADMIN_TEST_GUARD_CHILD") == "1" {
		ResetSchema(t, &gorm.DB{})
		t.Fatal("unexpected admission")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestResetSchemaRejectsBeforeSQL$")
	env := []string{}
	for _, v := range os.Environ() {
		if strings.HasPrefix(v, "ADMIN_TEST_") {
			continue
		}
		env = append(env, v)
	}
	cmd.Env = append(env, "ADMIN_TEST_GUARD_CHILD=1", "ADMIN_TEST_DSN=", "ADMIN_TEST_DATABASE=", "ADMIN_TEST_RUN_ID=")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "testdb: run ID must contain") || strings.Contains(string(out), "panic:") {
		t.Fatalf("expected fail-closed configuration check, got exit=%v output=%s", err, out)
	}
}
