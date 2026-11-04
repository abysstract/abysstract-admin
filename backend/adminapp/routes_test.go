package adminapp

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/module"
)

func TestNewRejectsStaticAndConnectConflicts(t *testing.T) {
	cfg := testConfig(t)
	svc, _ := probeService(t, "company.contract.v1", "ProbeService", nil)
	_, err := New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
		systemModule(),
		{
			Name:     "files",
			Requires: []string{"system"},
			HTTP: []module.HTTPRoute{{
				Name:   "static-cover",
				Method: "GET",
				Path:   "/static/x",
				Access: module.AccessPublic,
			}},
			Create: emptyCreate,
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("static conflict err = %v", err)
	}
	_, err = New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
		{
			Name:        "system",
			Services:    []module.ConnectService{svc},
			Permissions: publicPermissions("/company.contract.v1.ProbeService/Ping"),
			HTTP: []module.HTTPRoute{{
				Name:   "connect-cover",
				Method: "POST",
				Path:   "/company.contract.v1.ProbeService/extra",
				Access: module.AccessPublic,
			}},
			Create: emptyCreate,
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("connect conflict err = %v", err)
	}
}

func TestNewAllowsSamePathDifferentMethodsAndRejectsGETHEAD(t *testing.T) {
	cfg := testConfig(t)
	_, err := New(Options{Config: cfg, Logger: silentLogger(), AllowedHTTPMethods: []string{"PUT"}, Modules: []module.Definition{
		systemModule(),
		{
			Name:     "billing",
			Requires: []string{"system"},
			HTTP: []module.HTTPRoute{
				{Name: "get", Method: "GET", Path: "/billing/item", Access: module.AccessPublic},
				{Name: "put", Method: "PUT", Path: "/billing/item", Access: module.AccessAuthenticated},
			},
			Create: emptyCreate,
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = New(Options{Config: cfg, Logger: silentLogger(), AllowedHTTPMethods: []string{"HEAD"}, Modules: []module.Definition{
		systemModule(),
		{
			Name:     "billing",
			Requires: []string{"system"},
			HTTP: []module.HTTPRoute{
				{Name: "get", Method: "GET", Path: "/billing/item", Access: module.AccessPublic},
				{Name: "head", Method: "HEAD", Path: "/billing/item", Access: module.AccessPublic},
			},
			Create: emptyCreate,
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("GET/HEAD err = %v", err)
	}
}

func TestNewRejectsUnsupportedMethodWildcardAndAccess(t *testing.T) {
	cfg := testConfig(t)
	cases := []module.HTTPRoute{
		{Name: "trace", Method: "TRACE", Path: "/x", Access: module.AccessPublic},
		{Name: "wild", Method: "GET", Path: "/x/{id}", Access: module.AccessPublic},
		{Name: "access", Method: "GET", Path: "/x", Access: module.AccessUnspecified},
		{Name: "perm", Method: "POST", Path: "/x", Access: module.AccessPermission},
	}
	for _, route := range cases {
		_, err := New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
			systemModule(),
			{Name: "billing", Requires: []string{"system"}, HTTP: []module.HTTPRoute{route}, Create: emptyCreate},
		}})
		if err == nil {
			t.Fatalf("expected error for %#v", route)
		}
	}
}

func TestNewRejectsUncleanHTTPPaths(t *testing.T) {
	cfg := testConfig(t)
	cases := []struct {
		name  string
		route module.HTTPRoute
	}{
		{"double slash", module.HTTPRoute{Name: "unclean", Method: "GET", Path: "/policy//target", Access: module.AccessPublic}},
		{"dot segment", module.HTTPRoute{Name: "unclean", Method: "GET", Path: "/policy/./target", Access: module.AccessPublic}},
		{"dot path", module.HTTPRoute{Name: "unclean", Method: "GET", Path: "/.", Access: module.AccessPublic}},
		{"parent segment", module.HTTPRoute{Name: "unclean", Method: "GET", Path: "/policy/../target", Access: module.AccessPublic}},
		{"parent directory", module.HTTPRoute{Name: "unclean", Method: "GET", Path: "/../", Subtree: true, Access: module.AccessPublic}},
		{"double slash directory", module.HTTPRoute{Name: "unclean", Method: "GET", Path: "/policy//", Subtree: true, Access: module.AccessPublic}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var created atomic.Int32
			_, err := New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
				{Name: "system", Create: countingCreate(&created)},
				{Name: "billing", Requires: []string{"system"}, HTTP: []module.HTTPRoute{tc.route}, Create: countingCreate(&created)},
			}})
			if err == nil {
				t.Fatalf("accepted %#v", tc.route)
			}
			if !strings.Contains(err.Error(), "canonical") && !strings.Contains(err.Error(), "Path") {
				t.Fatalf("err = %v", err)
			}
			if created.Load() != 0 {
				t.Fatalf("Create called %d times", created.Load())
			}
		})
	}
}

func TestNewAcceptsCanonicalExactAndSubtreePaths(t *testing.T) {
	cfg := testConfig(t)
	_, err := New(Options{Config: cfg, Logger: silentLogger(), Modules: []module.Definition{
		systemModule(),
		{
			Name:     "billing",
			Requires: []string{"system"},
			HTTP: []module.HTTPRoute{
				{Name: "exact", Method: "GET", Path: "/policy/target", Access: module.AccessPublic},
				{Name: "tree", Method: "GET", Path: "/exports/", Subtree: true, Access: module.AccessPublic},
			},
			Create: emptyCreate,
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
}
