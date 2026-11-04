package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", relative, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relative, err)
	}
}

func fixtureModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "go.mod", "module example.com/adminapi-fixture\n\ngo 1.25\n\nrequire example.com/container v0.0.0\nreplace example.com/container => ./container\n")
	writeFixture(t, root, "container/go.mod", "module example.com/container\n\ngo 1.25\n")
	writeFixture(t, root, "container/container.go", `package container

type Box[T any] = []T
`)
	writeFixture(t, root, "internal/secret/secret.go", `package secret

type Secret struct{ Value string }
type Alias = string
type Constraint interface{ SecretMethod() }
`)
	writeFixture(t, root, "publicdep/publicdep.go", `package publicdep

import "example.com/adminapi-fixture/internal/secret"

type Wrapper struct {
    Value secret.Secret
}
`)
	writeFixture(t, root, "good/good.go", `package good

import (
    "example.com/adminapi-fixture/internal/secret"
    "example.com/container"
    "time"
)

type Node struct {
    Next *Node
    private secret.Secret
}
type PrivateOnly struct { private secret.Secret }
type Standard struct { When time.Time }
type Callback func(string) error
type Recursive interface { Next() Recursive }
type Generic[T interface{ ~int | ~string }] struct { Value T }
type Instantiated = Generic[int]
func implementation(secret.Secret) {}
func ThirdPartySafe() container.Box[int] { return nil }
`)
	writeFixture(t, root, "bad/bad.go", `package bad

import (
    "example.com/adminapi-fixture/internal/secret"
    "example.com/adminapi-fixture/publicdep"
    "example.com/container"
)

type Direct struct { Secret secret.Secret }
type Pointer struct { Secret *secret.Secret }
type Mapping struct { Values map[string]secret.Secret }
type Function func(func(secret.Secret) error)
type Embedded struct { secret.Secret }
type Alias = secret.Secret
type Generic[T interface{ secret.Constraint }] struct { Value T }
type BadInterface interface { Handle(secret.Secret) }
type Method struct{}
func (Method) Handle(secret.Secret) {}
func Exported(secret.Secret) {}
func ExportedCrossPackage() publicdep.Wrapper { return publicdep.Wrapper{} }
func ExportedInternalAlias() secret.Alias { return "alias" }
func ExportedThirdPartyAlias() container.Box[secret.Secret] { return nil }
`)
	return root
}

func TestCheckFixtureFindings(t *testing.T) {
	root := fixtureModule(t)
	findings, err := checkPackages([]string{"./bad"}, root)
	if err != nil {
		t.Fatalf("check bad fixture: %v", err)
	}
	if len(findings) < 8 {
		t.Fatalf("findings = %d, want direct and indirect public leaks: %#v", len(findings), findings)
	}
	joined := strings.Join(func() []string {
		values := make([]string, len(findings))
		for index, finding := range findings {
			values[index] = finding.String()
		}
		return values
	}(), "\n")
	for _, want := range []string{"member Direct", "member Pointer", "member Mapping", "member Function", "member Embedded", "member Alias", "member Generic", "member Exported", "member ExportedCrossPackage", "member ExportedInternalAlias", "member ExportedThirdPartyAlias"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("findings do not contain %q:\n%s", want, joined)
		}
	}
}

func TestCheckFixtureAllowsPrivateFieldsAndRecursivePublicTypes(t *testing.T) {
	root := fixtureModule(t)
	findings, err := checkPackages([]string{"./good"}, root)
	if err != nil {
		t.Fatalf("check good fixture: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("legal public API findings = %#v", findings)
	}
}

func TestCheckPackagesReportsLoadErrors(t *testing.T) {
	root := fixtureModule(t)
	findings, err := checkPackages([]string{"./does-not-exist"}, root)
	if err == nil || len(findings) != 0 {
		t.Fatalf("missing package result = findings=%v err=%v", findings, err)
	}
}

func TestInternalPathRequiresThePackageModuleBoundary(t *testing.T) {
	if !hasInternalSegment("example.com/app/internal/secret", "example.com/app") {
		t.Fatal("internal package should be recognized")
	}
	if hasInternalSegment("example.com/app-internal/secret", "example.com/app") {
		t.Fatal("module prefix collision must not be recognized")
	}
	if hasInternalSegment("example.com/other/internal/secret", "example.com/app") {
		t.Fatal("another module's internal package must not be recognized")
	}
}
