package adminapp

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/abysstract/abysstract-admin/backend/module"
)

var supportedHTTPMethods = map[string]struct{}{
	http.MethodGet:    {},
	http.MethodPost:   {},
	http.MethodPut:    {},
	http.MethodPatch:  {},
	http.MethodDelete: {},
	http.MethodHead:   {},
}

type preparedRoute struct {
	Module    string
	Route     module.HTTPRoute
	Pattern   string
	MaxBytes  int64
	Connect   bool
	Framework bool
}

func normalizeAllowedHTTPMethods(methods []string) ([]string, error) {
	seen := map[string]struct{}{
		http.MethodGet:  {},
		http.MethodPost: {},
	}
	for _, method := range methods {
		if method != strings.ToUpper(method) || method == "" {
			return nil, fmt.Errorf("adminapp: AllowedHTTPMethods %q must be uppercase", method)
		}
		if _, ok := supportedHTTPMethods[method]; !ok {
			return nil, fmt.Errorf("adminapp: HTTP method %q is not supported", method)
		}
		seen[method] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead} {
		if _, ok := seen[method]; ok {
			out = append(out, method)
		}
	}
	return out, nil
}

func collectAndCheckRoutes(defs []module.Definition, procedures []procedureSpec, allowed []string, appLimit int64) ([]preparedRoute, error) {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, method := range allowed {
		allowedSet[method] = struct{}{}
	}
	routes := frameworkRoutes()
	names := make(map[string]string)
	for _, proc := range uniquePrefixes(procedures) {
		if proc.Prefix != canonicalHTTPPath(proc.Prefix) {
			return nil, fieldError(proc.Module, "Services", fmt.Sprintf("connect path %q is not canonical", proc.Prefix))
		}
		for _, method := range allowed {
			pattern := method + " " + proc.Prefix
			if err := validateServeMuxPattern(pattern); err != nil {
				return nil, fieldError(proc.Module, "Services", err.Error())
			}
			routes = append(routes, preparedRoute{
				Module:   proc.Module,
				Connect:  true,
				Pattern:  pattern,
				Route:    module.HTTPRoute{Method: method, Path: proc.Prefix, Subtree: true},
				MaxBytes: appLimit,
			})
		}
	}
	for _, def := range defs {
		for i, route := range def.HTTP {
			prepared, err := prepareHTTPRoute(def.Name, i, route, allowedSet, appLimit)
			if err != nil {
				return nil, err
			}
			if prev, ok := names[route.Name]; ok {
				return nil, fieldError(def.Name, "HTTP.Name", fmt.Sprintf("%q already used by module %q", route.Name, prev))
			}
			names[route.Name] = def.Name
			routes = append(routes, prepared)
		}
	}
	if err := checkRouteConflicts(routes); err != nil {
		return nil, err
	}
	business := make([]preparedRoute, 0, len(defs))
	for _, route := range routes {
		if !route.Connect && !route.Framework {
			business = append(business, route)
		}
	}
	return business, nil
}

func prepareHTTPRoute(moduleName string, index int, route module.HTTPRoute, allowed map[string]struct{}, appLimit int64) (preparedRoute, error) {
	field := fmt.Sprintf("HTTP[%d]", index)
	if strings.TrimSpace(route.Name) == "" {
		return preparedRoute{}, fieldError(moduleName, field+".Name", "is required")
	}
	if route.Method != strings.ToUpper(route.Method) || route.Method == "" {
		return preparedRoute{}, fieldError(moduleName, field+".Method", "must be an uppercase HTTP method")
	}
	if _, ok := supportedHTTPMethods[route.Method]; !ok {
		return preparedRoute{}, fieldError(moduleName, field+".Method", fmt.Sprintf("%q is not supported", route.Method))
	}
	if _, ok := allowed[route.Method]; !ok {
		return preparedRoute{}, fieldError(moduleName, field+".Method", fmt.Sprintf("%q is not in AllowedHTTPMethods", route.Method))
	}
	if !strings.HasPrefix(route.Path, "/") || strings.ContainsAny(route.Path, "{}*") {
		return preparedRoute{}, fieldError(moduleName, field+".Path", "must start with / and cannot use wildcards")
	}
	canonical := canonicalHTTPPath(route.Path)
	if canonical != route.Path {
		return preparedRoute{}, fieldError(moduleName, field+".Path", fmt.Sprintf("%q is not canonical (want %q)", route.Path, canonical))
	}
	if route.Subtree && !strings.HasSuffix(route.Path, "/") {
		return preparedRoute{}, fieldError(moduleName, field+".Path", "subtree paths must end with /")
	}
	if !route.Subtree && strings.HasSuffix(route.Path, "/") {
		return preparedRoute{}, fieldError(moduleName, field+".Path", "exact paths cannot end with /")
	}
	switch route.Access {
	case module.AccessPublic, module.AccessAuthenticated:
		if route.Permission != "" {
			return preparedRoute{}, fieldError(moduleName, field+".Permission", "must be empty unless AccessPermission")
		}
	case module.AccessPermission:
		if route.Permission == "" {
			return preparedRoute{}, fieldError(moduleName, field+".Permission", "is required")
		}
	default:
		return preparedRoute{}, fieldError(moduleName, field+".Access", "must be declared")
	}
	maxBytes := route.MaxBytes
	if maxBytes == 0 {
		maxBytes = appLimit
	}
	if maxBytes < 0 {
		return preparedRoute{}, fieldError(moduleName, field+".MaxBytes", "cannot be negative")
	}
	if maxBytes > appLimit {
		return preparedRoute{}, fieldError(moduleName, field+".MaxBytes", "cannot exceed the application limit")
	}
	return preparedRoute{
		Module:   moduleName,
		Route:    route,
		Pattern:  route.Method + " " + route.Path,
		MaxBytes: maxBytes,
	}, nil
}

func frameworkRoutes() []preparedRoute {
	exact := []string{"/healthz", "/readyz", "/metrics", "/version"}
	out := make([]preparedRoute, 0, len(exact)+2)
	for _, path := range exact {
		out = append(out, preparedRoute{
			Framework: true,
			Pattern:   http.MethodGet + " " + path,
			Route:     module.HTTPRoute{Method: http.MethodGet, Path: path},
		})
	}
	// /static/ is reserved for the storage handler: GET/HEAD downloads and
	// signed PUT uploads of the local driver.
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		out = append(out, preparedRoute{
			Framework: true,
			Pattern:   method + " " + storageMountPath,
			Route:     module.HTTPRoute{Method: method, Path: storageMountPath, Subtree: true},
		})
	}
	return out
}

func uniquePrefixes(procedures []procedureSpec) []procedureSpec {
	seen := make(map[string]struct{})
	out := make([]procedureSpec, 0)
	for _, proc := range procedures {
		if _, ok := seen[proc.Prefix]; ok {
			continue
		}
		seen[proc.Prefix] = struct{}{}
		out = append(out, proc)
	}
	return out
}

func checkRouteConflicts(routes []preparedRoute) error {
	for i := 0; i < len(routes); i++ {
		for j := i + 1; j < len(routes); j++ {
			if routesOverlap(routes[i], routes[j]) {
				return fmt.Errorf("adminapp: HTTP path conflict between %s and %s", routes[i].Pattern, routes[j].Pattern)
			}
		}
	}
	return nil
}

func routesOverlap(a, b preparedRoute) bool {
	if !methodsOverlap(a.Route.Method, b.Route.Method) {
		return false
	}
	return pathsOverlap(a.Route.Path, a.Route.Subtree || a.Connect || (a.Framework && strings.HasSuffix(a.Route.Path, "/")), b.Route.Path, b.Route.Subtree || b.Connect || (b.Framework && strings.HasSuffix(b.Route.Path, "/")))
}

func methodsOverlap(a, b string) bool {
	if a == b {
		return true
	}
	if (a == http.MethodGet && b == http.MethodHead) || (a == http.MethodHead && b == http.MethodGet) {
		return true
	}
	return false
}

func pathsOverlap(aPath string, aSub bool, bPath string, bSub bool) bool {
	if aPath == bPath {
		return true
	}
	if aSub && strings.HasPrefix(bPath, aPath) {
		return true
	}
	if bSub && strings.HasPrefix(aPath, bPath) {
		return true
	}
	return false
}

func canonicalHTTPPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	cleaned := path.Clean(p)
	if p[len(p)-1] == '/' && cleaned != "/" {
		cleaned += "/"
	}
	return cleaned
}

func validateServeMuxPattern(pattern string) error {
	pathToCheck := pattern
	if _, rest, ok := strings.Cut(pattern, " "); ok {
		pathToCheck = rest
	}
	if pathToCheck != canonicalHTTPPath(pathToCheck) {
		return fmt.Errorf("adminapp: HTTP path %q is not canonical", pathToCheck)
	}
	return nil
}
