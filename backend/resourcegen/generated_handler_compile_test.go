//go:build integration

package resourcegen

import (
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

// TestResourceCompileSamplesAndConnectHandlerGate is the formal generated handler compile check for
// Requirement 14. It drives Build -> render into an isolated package -> real
// go test compilation; formatting or source inspection alone is not evidence
// for this gate.
func TestResourceCompileSamplesAndConnectHandlerGate(t *testing.T) {
	runResourceFormalCompileGates(t)
}

func runResourceFormalCompileGates(t *testing.T) {
	t.Helper()

	t.Run("sample A: generated default repository", func(t *testing.T) {
		plan, err := schematest.BoolCompileSampleResource().Build()
		if err != nil {
			t.Fatalf("build Sample A: %v", err)
		}
		compileServicePlansWithRuntime(t, []*schema.Plan{plan}, widgetSampleARuntimeTest)
	})

	t.Run("sample B: HookSet and injected typed repository", func(t *testing.T) {
		res := schematest.BoolCompileSampleResource()
		res.Repository.Implementation = schema.RepositoryInjected
		plan, err := res.Build()
		if err != nil {
			t.Fatalf("build Sample B: %v", err)
		}
		compileServicePlansWithRuntime(t, []*schema.Plan{plan}, boolSampleBRuntimeTest)
	})

	t.Run("sample C: generated and partial resources share a service", func(t *testing.T) {
		compileServicePlansWithRuntime(t, widgetServicePlans(t), widgetServiceAdapterRuntimeTest)
	})

	t.Run("generated Dict adapter satisfies Connect handler", func(t *testing.T) {
		compileServicePlansWithRuntime(t, dictServicePlans(t), dictServiceAdapterRuntimeTest)
	})
}
