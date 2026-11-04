package interceptor

import "testing"

// Feature: open-source-readiness, Property 2: 过程权限声明完整性
//
// 枚举冻结 Registry snapshot。每个 procedure 恰好归类为 public 或 required，
// 不存在「既非公开、又无权限声明」的过程，也不存在两类重复归属。
func TestRegistryProcedureCompleteness(t *testing.T) {
	registry := NewPermRegistry()
	if err := RegisterBuiltinPermissions(registry); err != nil {
		t.Fatalf("register builtin permissions: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("freeze registry: %v", err)
	}

	snapshot := registry.Snapshot()
	if !snapshot.Frozen {
		t.Fatal("registry snapshot is not frozen")
	}
	seen := make(map[string]bool, len(snapshot.PublicProcedures)+len(snapshot.RequiredPermissions))
	for _, procedure := range snapshot.PublicProcedures {
		if seen[procedure] {
			t.Errorf("Registry snapshot has duplicate procedure: %s", procedure)
		}
		seen[procedure] = true
		if _, protected := snapshot.RequiredPermissions[procedure]; protected {
			t.Errorf("procedure %s is both public and protected", procedure)
		}
	}
	for procedure, permission := range snapshot.RequiredPermissions {
		if seen[procedure] {
			t.Errorf("Registry snapshot has duplicate procedure: %s", procedure)
		}
		seen[procedure] = true
		if permission == "" {
			t.Errorf("protected procedure %s has empty permission", procedure)
		}
	}
}
