package resourcegen

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

// outFile 一个待写出的产物（内存中的完整内容）。
type outFile struct {
	name    string
	content []byte
}

// renderGo 渲染一个资源的全部 Go 产物：converter/校验 与（有标准 List 时的）
// typed ListQuery + Repository adapter。全部在内存渲染并 gofmt，不做任何写入。
func (g *generator) renderGo(p *schema.Plan) ([]outFile, error) {
	view := buildView(p, g.cfg)
	render := func(tpl *template.Template, name string) (outFile, error) {
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, view); err != nil {
			return outFile{}, err
		}
		src, err := format.Source(buf.Bytes())
		if err != nil {
			return outFile{}, fmt.Errorf("gofmt %s: %w\n---\n%s", p.Res.Name, err, buf.String())
		}
		return outFile{name: name, content: src}, nil
	}
	conv, err := render(goConvTpl, fmt.Sprintf("%s_conv.gen.go", snake(p.Res.Name)))
	if err != nil {
		return nil, err
	}
	files := []outFile{conv}
	if len(view.Permissions) > 0 {
		permissions, err := render(goPermissionsTpl, fmt.Sprintf("%s_permissions.gen.go", snake(p.Res.Name)))
		if err != nil {
			return nil, err
		}
		files = append(files, permissions)
	}
	if view.List != nil {
		repoFile, err := render(goRepoTpl, fmt.Sprintf("%s_repo.gen.go", snake(p.Res.Name)))
		if err != nil {
			return nil, err
		}
		files = append(files, repoFile)
	}
	if hasFullGeneratedSix(p) || hasGeneratedReadController(p) {
		ctrl, err := render(goControllerTpl, fmt.Sprintf("%s_controller.gen.go", snake(p.Res.Name)))
		if err != nil {
			return nil, err
		}
		files = append(files, ctrl)
	} else if hasCustomOnlyController(p) {
		ctrl, err := render(goCustomControllerTpl, fmt.Sprintf("%s_controller.gen.go", snake(p.Res.Name)))
		if err != nil {
			return nil, err
		}
		files = append(files, ctrl)
	}
	return files, nil
}

// hasFullGeneratedSix 报告资源是否声明了完整且全部 OperationGenerated 的标准六操作。
func hasFullGeneratedSix(p *schema.Plan) bool {
	kinds := []schema.OperationKind{
		schema.OperationList, schema.OperationCreate, schema.OperationGet,
		schema.OperationUpdate, schema.OperationDelete, schema.OperationBatchDelete,
	}
	for _, kind := range kinds {
		op := p.Operation(kind)
		if op == nil || op.Implementation != schema.OperationGenerated {
			return false
		}
	}
	return true
}

// hasGeneratedReadController reports the supported low-risk partial shape:
// generated standard List plus zero or more typed CUSTOM workflows.
func hasGeneratedReadController(p *schema.Plan) bool {
	if p == nil || p.Res.Mode != schema.Partial {
		return false
	}
	if p.Operation(schema.OperationList) == nil {
		return false
	}
	for _, operation := range p.Operations {
		if operation.Kind != schema.OperationList && operation.Kind != schema.OperationCustom {
			return false
		}
	}
	return true
}

// hasCustomOnlyController reports a Partial Resource with at least one CUSTOM
// operation but without a complete standard-six controller. It needs only a
// typed override field, not a Repository/ListQuery dependency.
func hasCustomOnlyController(p *schema.Plan) bool {
	if p.Res.Mode != schema.Partial || hasFullGeneratedSix(p) {
		return false
	}
	for _, operation := range p.Operations {
		if operation.Kind == schema.OperationCustom {
			return true
		}
	}
	return false
}

// renderController 渲染六操作资源的 Controller 骨架（需求 8.1/8.5）。
func (g *generator) renderController(p *schema.Plan) (string, []byte, error) {
	var buf bytes.Buffer
	if err := goControllerTpl.Execute(&buf, buildView(p, g.cfg)); err != nil {
		return "", nil, err
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return "", nil, fmt.Errorf("gofmt controller %s: %w\n---\n%s", p.Res.Name, err, buf.String())
	}
	return fmt.Sprintf("%s_controller.gen.go", snake(p.Res.Name)), src, nil
}

func (g *generator) renderTS(p *schema.Plan) (string, []byte, error) {
	var buf bytes.Buffer
	if err := tsSchemaTpl.Execute(&buf, buildView(p, g.cfg)); err != nil {
		return "", nil, err
	}
	name := fmt.Sprintf("%s.schema.gen.ts", strings.ReplaceAll(snake(p.Res.Name), "_", "-"))
	return name, buf.Bytes(), nil
}

// renderTSOperations emits the dependency-ordered frontend operation layers:
// protobuf service/client -> raw RPC calls -> presence-aware UI adapter. The
// static cross-resource registry is rendered separately in task 12.8.
func (g *generator) renderTSOperations(p *schema.Plan) ([]outFile, error) {
	view := buildView(p, g.cfg)
	if len(view.TSRawOperations) == 0 {
		return nil, nil
	}
	render := func(tpl *template.Template, name string) (outFile, error) {
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, view); err != nil {
			return outFile{}, err
		}
		return outFile{name: name, content: buf.Bytes()}, nil
	}
	base := strings.ReplaceAll(snake(p.Res.Name), "_", "-")
	raw, err := render(tsRawOperationsTpl, base+".raw.gen.ts")
	if err != nil {
		return nil, err
	}
	files := []outFile{raw}
	if view.TSFacade != nil || view.TSUpdate != nil || view.TSReadFacade != nil {
		templateToRender := tsOperationsTpl
		if view.TSFacade == nil && view.TSReadFacade != nil {
			templateToRender = tsReadOperationsTpl
		}
		adapter, err := render(templateToRender, base+".operations.gen.ts")
		if err != nil {
			return nil, err
		}
		files = append(files, adapter)
	}
	return files, nil
}

// renderTSOperationsRegistry creates one acyclic static registry from all
// Resources that expose a validated standard paginated List operation.
func (g *generator) renderTSOperationsRegistry(plans []*schema.Plan) (outFile, error) {
	view := tsRegistryView{Header: fileHeader(g.version, g.cfg.FormatVersion, g.cfg.SourceIdentity)}
	for _, plan := range plans {
		list := plan.Operation(schema.OperationList)
		if list == nil || list.List == nil {
			continue
		}
		planView := buildView(plan, g.cfg)
		view.Entries = append(view.Entries, tsRegistryEntry{
			Name:       plan.Res.Name,
			TSVar:      planView.TSVar,
			TSFileBase: planView.TSFileBase,
			ListMethod: lowerFirst(list.Method),
		})
		if planView.TSFacade != nil || planView.TSReadFacade != nil {
			view.Facades = append(view.Facades, tsRegistryEntry{
				Name:       plan.Res.Name,
				TSVar:      planView.TSVar,
				TSFileBase: planView.TSFileBase,
			})
		}
	}
	if len(view.Entries) == 0 {
		return outFile{}, nil
	}
	var buf bytes.Buffer
	if err := tsOperationsRegistryTpl.Execute(&buf, view); err != nil {
		return outFile{}, err
	}
	return outFile{name: "resource-operations.gen.ts", content: buf.Bytes()}, nil
}

// renderTSRelations emits per-source relation loaders after the static raw
// registry. BuildRegistry has already validated every target List binding.
func (g *generator) renderTSRelations(plan *schema.Plan, all []*schema.Plan) ([]outFile, error) {
	view := buildView(plan, g.cfg)
	byName := make(map[string]*schema.Plan, len(all))
	for _, candidate := range all {
		byName[candidate.Res.Name] = candidate
	}
	for _, field := range plan.Fields {
		if field.Relation == nil {
			continue
		}
		target := byName[field.Relation.TargetResource]
		if target == nil {
			return nil, fmt.Errorf("%s: relation target %q missing from frontend plans", plan.Res.Name, field.Relation.TargetResource)
		}
		list := target.Operation(schema.OperationList)
		if list == nil || list.List == nil {
			return nil, fmt.Errorf("%s: relation target %q lacks standard List", plan.Res.Name, field.Relation.TargetResource)
		}
		var valueField *schema.FieldPlan
		for _, targetField := range target.Fields {
			if targetField.JSONName == field.Relation.RefJSONName {
				valueField = targetField
				break
			}
		}
		if valueField == nil {
			return nil, fmt.Errorf("%s: relation target %q RefKey JSON field %q missing", plan.Res.Name, target.Res.Name, field.Relation.RefJSONName)
		}
		targetView := buildView(target, g.cfg)
		view.TSRelations = append(view.TSRelations, tsRelationView{
			FieldName:           lowerFirst(field.ProtoGoName),
			TargetResource:      target.Res.Name,
			TargetTSVar:         targetView.TSVar,
			TargetTSFileBase:    targetView.TSFileBase,
			TargetListMethod:    lowerFirst(list.Method),
			PageField:           lowerFirst(list.List.PageRequestField),
			KeywordField:        lowerFirst(list.List.KeywordField),
			ItemsField:          lowerFirst(list.List.ItemsField),
			PageInfoField:       lowerFirst(list.List.PageInfoField),
			ValueField:          field.Relation.RefJSONName,
			ValueType:           valueField.TSType,
			LabelField:          field.Relation.LabelJSONName,
			RemoteSearchEnabled: field.Relation.RemoteSearchEnabled,
		})
	}
	if len(view.TSRelations) == 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	if err := tsRelationsTpl.Execute(&buf, view); err != nil {
		return nil, err
	}
	return []outFile{{name: view.TSFileBase + ".relations.gen.ts", content: buf.Bytes()}}, nil
}

type serviceAdapterControllerView struct {
	Field string
	Type  string
}

type serviceAdapterMethodView struct {
	Method          string
	RequestRef      string
	ResponseRef     string
	ControllerField string
}

type serviceAdapterImport struct {
	Alias string
	Path  string
}

type serviceAdapterView struct {
	Name              string
	Header            string
	GoPackage         string
	HandlerAlias      string
	HandlerImportPath string
	HandlerType       string
	Controllers       []serviceAdapterControllerView
	Methods           []serviceAdapterMethodView
	Imports           []serviceAdapterImport
}

var goServiceAdapterTpl = template.Must(template.New("service-adapter").Parse(`{{ .Header }}

package {{ .GoPackage }}

import (
  "context"
  "fmt"

  "{{ .HandlerImportPath }}"
{{- range .Imports }}
  "{{ .Path }}"
{{- end }}
)

// {{ .Name }}Adapter aggregates the method-owning Resource Controllers for one
// Connect service. BuildRegistry has already rejected duplicate ownership.
type {{ .Name }}Adapter struct {
{{- range .Controllers }}
  {{ .Field }} *{{ .Type }}
{{- end }}
}

func New{{ .Name }}Adapter(
{{- range .Controllers }}
  {{ .Field }} *{{ .Type }},
{{- end }}
) (*{{ .Name }}Adapter, error) {
{{- range .Controllers }}
  if {{ .Field }} == nil {
    return nil, fmt.Errorf("{{ $.Name }}Adapter: {{ .Field }} controller is nil")
  }
{{- end }}
  return &{{ .Name }}Adapter{
{{- range .Controllers }}
    {{ .Field }}: {{ .Field }},
{{- end }}
  }, nil
}

var _ {{ .HandlerAlias }}.{{ .HandlerType }} = (*{{ .Name }}Adapter)(nil)

{{- range .Methods }}
func (a *{{ $.Name }}Adapter) {{ .Method }}(ctx context.Context, req *{{ .RequestRef }}) (*{{ .ResponseRef }}, error) {
  return a.{{ .ControllerField }}.{{ .Method }}(ctx, req)
}
{{- end }}
`))

// renderServiceAdapters emits only groups whose generated controllers cover
// every method in the declared Connect handler interface. Incomplete business
// Resources remain unadapted until their wire contract is complete.
func (g *generator) renderServiceAdapters(plans []*schema.Plan) ([]outFile, error) {
	groups := map[string][]*schema.Plan{}
	var keys []string
	for _, plan := range plans {
		service := plan.Res.Service.Descriptor
		if service == nil || len(plan.Operations) == 0 {
			continue
		}
		key := string(service.FullName())
		if _, exists := groups[key]; !exists {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], plan)
	}
	sort.Strings(keys)
	var files []outFile
	for _, key := range keys {
		group := groups[key]
		view, ok := buildServiceAdapterView(group, g.cfg)
		if !ok {
			continue
		}
		var buf bytes.Buffer
		if err := goServiceAdapterTpl.Execute(&buf, view); err != nil {
			return nil, err
		}
		source, err := format.Source(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("gofmt service adapter %s: %w\n---\n%s", view.Name, err, buf.String())
		}
		files = append(files, outFile{name: snake(view.Name) + "_adapter.gen.go", content: source})
	}
	return files, nil
}

func buildServiceAdapterView(group []*schema.Plan, cfg Config) (serviceAdapterView, bool) {
	if len(group) == 0 {
		return serviceAdapterView{}, false
	}
	service := group[0].Res.Service.Descriptor
	handler := reflect.TypeOf(group[0].Res.Service.Handler)
	for handler != nil && handler.Kind() == reflect.Pointer {
		handler = handler.Elem()
	}
	if service == nil || handler == nil || handler.Kind() != reflect.Interface {
		return serviceAdapterView{}, false
	}
	handlerAlias, handlerPath := typePackage(handler)
	if handlerAlias == "" || handlerPath == "" || handler.Name() == "" {
		return serviceAdapterView{}, false
	}
	view := serviceAdapterView{Name: string(service.Name()), Header: fileHeaderFrom(cfg), GoPackage: cfg.GoPackage, HandlerAlias: handlerAlias, HandlerImportPath: handlerPath, HandlerType: handler.Name()}
	methodOwners := map[string]*schema.Plan{}
	methodPlans := map[string]*schema.OperationPlan{}
	for _, plan := range group {
		if !hasFullGeneratedSix(plan) && !hasGeneratedReadController(plan) && !hasCustomOnlyController(plan) {
			return serviceAdapterView{}, false
		}
		field := lowerFirst(plan.Res.Name)
		view.Controllers = append(view.Controllers, serviceAdapterControllerView{Field: field, Type: plan.Res.Name + "Controller"})
		for _, operation := range plan.Operations {
			methodOwners[operation.Method] = plan
			methodPlans[operation.Method] = operation
		}
	}
	for i := 0; i < handler.NumMethod(); i++ {
		method := handler.Method(i).Name
		owner := methodOwners[method]
		operation := methodPlans[method]
		if owner == nil || operation == nil {
			return serviceAdapterView{}, false
		}
		aliases := collectGoPackages(operation.RequestType, operation.ResponseType)
		view.Methods = append(view.Methods, serviceAdapterMethodView{
			Method: method, RequestRef: aliases.typeRef(operation.RequestType), ResponseRef: aliases.typeRef(operation.ResponseType), ControllerField: lowerFirst(owner.Res.Name),
		})
		for _, typ := range []reflect.Type{operation.RequestType, operation.ResponseType} {
			for typ.Kind() == reflect.Pointer {
				typ = typ.Elem()
			}
			alias, path := typePackage(typ)
			if path != "" && path != handlerPath && alias != "" {
				seen := false
				for _, existing := range view.Imports {
					if existing.Path == path {
						seen = true
					}
				}
				if !seen {
					view.Imports = append(view.Imports, serviceAdapterImport{Alias: alias, Path: path})
				}
			}
		}
	}
	sort.Slice(view.Imports, func(i, j int) bool { return view.Imports[i].Path < view.Imports[j].Path })
	return view, true
}

func snake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// moduleRoot 向上查找 go.mod 所在目录。
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("未找到 go.mod")
		}
		dir = parent
	}
}

func renderGo(p *schema.Plan) ([]outFile, error) {
	return testGenerator().renderGo(p)
}

func renderTS(p *schema.Plan) (string, []byte, error) {
	return testGenerator().renderTS(p)
}

func renderTSOperations(p *schema.Plan) ([]outFile, error) {
	return testGenerator().renderTSOperations(p)
}

func renderController(p *schema.Plan) (string, []byte, error) {
	return testGenerator().renderController(p)
}

func renderServiceAdapters(plans []*schema.Plan) ([]outFile, error) {
	return testGenerator().renderServiceAdapters(plans)
}

func testGenerator() *generator {
	return &generator{cfg: fixtureConfig(), fs: osFS{}, version: toolVersion()}
}

func renderTSOperationsRegistry(plans []*schema.Plan) (outFile, error) {
	return testGenerator().renderTSOperationsRegistry(plans)
}
