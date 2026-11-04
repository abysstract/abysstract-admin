package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Finding identifies one public declaration path that reaches an internal
// implementation type. Path is deliberately human-readable because the CLI
// is used as a design feedback tool before package publication.
type Finding struct {
	Package  string
	Member   string
	Path     string
	Internal string
}

func (f Finding) String() string {
	return fmt.Sprintf("package %s member %s exposes internal type %s via %s", f.Package, f.Member, f.Internal, f.Path)
}

// checkPackages loads patterns from dir and checks every successfully loaded
// package. An empty dir uses the caller's current working directory, which is
// the backend module for the normal CLI invocation.
func checkPackages(patterns []string, dir string) ([]Finding, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no package patterns supplied")
	}
	config := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedSyntax |
			packages.NeedModule,
		Dir: dir,
	}
	loaded, err := packages.Load(config, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}
	var loadErrors []string
	for _, pkg := range loaded {
		for _, loadError := range pkg.Errors {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %s", pkg.PkgPath, loadError))
		}
	}
	if len(loadErrors) > 0 {
		return nil, fmt.Errorf("package loading failed: %s", strings.Join(loadErrors, "; "))
	}
	findings := make([]Finding, 0)
	for _, pkg := range loaded {
		packageFindings, checkErr := checkPackage(pkg)
		if checkErr != nil {
			return nil, checkErr
		}
		findings = append(findings, packageFindings...)
	}
	return findings, nil
}

func checkPackage(pkg *packages.Package) ([]Finding, error) {
	if pkg.Types == nil || pkg.Types.Scope() == nil {
		return nil, fmt.Errorf("package %s has no type information", pkg.PkgPath)
	}
	modulePath := pkg.PkgPath
	if pkg.Module != nil && pkg.Module.Path != "" {
		modulePath = pkg.Module.Path
	}
	checker := &apiChecker{
		pkg:        pkg.Types,
		modulePath: modulePath,
		findings:   make([]Finding, 0),
		visited:    make(map[types.Type]struct{}),
	}
	for _, name := range pkg.Types.Scope().Names() {
		if !ast.IsExported(name) {
			continue
		}
		object := pkg.Types.Scope().Lookup(name)
		checker.inspectObject(object, name)
	}
	return checker.findings, nil
}

type apiChecker struct {
	pkg        *types.Package
	modulePath string
	findings   []Finding
	visited    map[types.Type]struct{}
}

func (c *apiChecker) inspectObject(object types.Object, member string) {
	if object == nil {
		return
	}
	c.inspectType(object.Type(), member, member)
}

func (c *apiChecker) inspectType(typ types.Type, member, apiPath string) {
	if typ == nil {
		return
	}
	if alias, ok := typ.(*types.Alias); ok {
		c.inspectAlias(alias, member, apiPath)
		return
	}
	typ = types.Unalias(typ)
	if c.reachesInternal(typ, member, apiPath) {
		return
	}
	if _, seen := c.visited[typ]; seen {
		return
	}
	c.visited[typ] = struct{}{}

	switch current := typ.(type) {
	case *types.Named:
		c.inspectNamedType(current, member, apiPath)
	case *types.Pointer:
		c.inspectType(current.Elem(), member, apiPath+" pointer")
	case *types.Slice:
		c.inspectType(current.Elem(), member, apiPath+" slice element")
	case *types.Array:
		c.inspectType(current.Elem(), member, apiPath+" array element")
	case *types.Map:
		c.inspectType(current.Key(), member, apiPath+" map key")
		c.inspectType(current.Elem(), member, apiPath+" map value")
	case *types.Chan:
		c.inspectType(current.Elem(), member, apiPath+" channel element")
	case *types.Signature:
		for index := 0; index < current.TypeParams().Len(); index++ {
			parameter := current.TypeParams().At(index)
			c.inspectType(parameter.Constraint(), member, fmt.Sprintf("%s type parameter %s", apiPath, parameter.Obj().Name()))
		}
		c.inspectTuple(current.Params(), member, apiPath+" parameter")
		c.inspectTuple(current.Results(), member, apiPath+" result")
	case *types.Struct:
		c.inspectStruct(current, member, apiPath)
	case *types.Interface:
		c.inspectInterface(current, member, apiPath)
	case *types.Tuple:
		c.inspectTuple(current, member, apiPath)
	case *types.TypeParam:
		c.inspectType(current.Constraint(), member, apiPath+" constraint")
	case *types.Union:
		for index := 0; index < current.Len(); index++ {
			c.inspectType(current.Term(index).Type(), member, apiPath+" union term")
		}
	}
}

func (c *apiChecker) inspectTuple(tuple *types.Tuple, member, apiPath string) {
	if tuple == nil {
		return
	}
	for index := 0; index < tuple.Len(); index++ {
		c.inspectType(tuple.At(index).Type(), member, fmt.Sprintf("%s %d", apiPath, index))
	}
}

func (c *apiChecker) inspectNamedMethods(named *types.Named, member, apiPath string) {
	methodSets := []*types.MethodSet{
		types.NewMethodSet(named),
		types.NewMethodSet(types.NewPointer(named)),
	}
	seen := make(map[string]struct{})
	for _, methodSet := range methodSets {
		for index := 0; index < methodSet.Len(); index++ {
			method := methodSet.At(index).Obj()
			if !method.Exported() {
				continue
			}
			key := method.Id()
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			c.inspectType(method.Type(), member, apiPath+" method "+method.Name())
		}
	}
}

func (c *apiChecker) reachesInternal(typ types.Type, member, apiPath string) bool {
	named, ok := typ.(*types.Named)
	if !ok {
		return false
	}
	return c.reachesInternalObject(named.Obj(), member, apiPath)
}

func (c *apiChecker) reachesInternalObject(object *types.TypeName, member, apiPath string) bool {
	if object == nil || object.Pkg() == nil || !sameModule(object.Pkg().Path(), c.modulePath) {
		return false
	}
	if !hasInternalSegment(object.Pkg().Path(), c.modulePath) {
		return false
	}
	c.findings = append(c.findings, Finding{
		Package:  c.pkg.Path(),
		Member:   member,
		Path:     apiPath,
		Internal: object.Pkg().Path() + "." + object.Name(),
	})
	return true
}

func sameModule(packagePath, modulePath string) bool {
	return packagePath == modulePath || strings.HasPrefix(packagePath, modulePath+"/")
}

func hasInternalSegment(packagePath, modulePath string) bool {
	if !sameModule(packagePath, modulePath) {
		return false
	}
	relative := strings.TrimPrefix(packagePath, modulePath)
	return strings.Contains("/"+relative+"/", "/internal/") || strings.HasSuffix(relative, "/internal")
}

func (c *apiChecker) inspectAlias(alias *types.Alias, member, apiPath string) {
	// Type arguments belong to the use site, not to the package that
	// declared the alias. Inspect them before applying the third-party
	// implementation boundary.
	for index := 0; index < alias.TypeArgs().Len(); index++ {
		c.inspectType(alias.TypeArgs().At(index), member, fmt.Sprintf("%s type argument %d", apiPath, index))
	}
	// The alias declaration itself is part of the exported signature.
	// Check its declaring package before following the RHS: an alias to a
	// predeclared type still leaks an internal package name.
	if c.reachesInternalObject(alias.Obj(), member, apiPath+" alias") {
		return
	}
	if alias.Obj() != nil && alias.Obj().Pkg() != nil && sameModule(alias.Obj().Pkg().Path(), c.modulePath) {
		for index := 0; index < alias.TypeParams().Len(); index++ {
			parameter := alias.TypeParams().At(index)
			c.inspectType(parameter.Constraint(), member, fmt.Sprintf("%s type parameter %s", apiPath, parameter.Obj().Name()))
		}
		c.inspectType(alias.Rhs(), member, apiPath+" alias RHS")
	}
}

func (c *apiChecker) inspectNamedType(current *types.Named, member, apiPath string) {
	for index := 0; index < current.TypeArgs().Len(); index++ {
		c.inspectType(current.TypeArgs().At(index), member, fmt.Sprintf("%s type argument %d", apiPath, index))
	}
	// Third-party named types are already part of an external public
	// contract. Types from this module, including another public package,
	// remain part of the surface being checked and must be traversed.
	if current.Obj().Pkg() != nil && !sameModule(current.Obj().Pkg().Path(), c.modulePath) {
		return
	}
	for index := 0; index < current.TypeParams().Len(); index++ {
		parameter := current.TypeParams().At(index)
		c.inspectType(parameter.Constraint(), member, fmt.Sprintf("%s type parameter %s", apiPath, parameter.Obj().Name()))
	}
	c.inspectType(current.Underlying(), member, apiPath)
	c.inspectNamedMethods(current, member, apiPath)
}

func (c *apiChecker) inspectStruct(current *types.Struct, member, apiPath string) {
	for index := 0; index < current.NumFields(); index++ {
		field := current.Field(index)
		if field.Exported() || field.Embedded() {
			c.inspectType(field.Type(), member, apiPath+" field "+field.Name())
		}
	}
}

func (c *apiChecker) inspectInterface(current *types.Interface, member, apiPath string) {
	current.Complete()
	for index := 0; index < current.NumMethods(); index++ {
		method := current.Method(index)
		if method.Exported() {
			c.inspectType(method.Type(), member, apiPath+" method "+method.Name())
		}
	}
	for index := 0; index < current.NumEmbeddeds(); index++ {
		c.inspectType(current.EmbeddedType(index), member, apiPath+" embedded interface")
	}
}
