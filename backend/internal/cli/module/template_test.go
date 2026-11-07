package module

import (
	"bytes"
	"go/ast"
	"go/parser"
	"strings"
	"testing"
	"text/template"
)

// TestResourceTemplatePassesVetAppends renders a Resource with and without
// custom operations and applies go vet's appends rule to its Operations
// expression: append needs values.
func TestResourceTemplatePassesVetAppends(t *testing.T) {
	tmpl, err := template.New("resource.go.tmpl").ParseFS(scaffoldTemplates, "templates/resource.go.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	view := crudView{Name: "Note", Plural: "Notes", File: "note", Model: "NotesNote", AuditGo: "UpdateTime"}
	view.Module = "notes"
	for _, operations := range [][]crudOperation{nil, {{Method: "PublishNote", Request: "PublishNoteRequest", Response: "PublishNoteResponse", Action: "publish", Title: "发布", Scope: "exempt"}}} {
		view.Operations = operations
		var out bytes.Buffer
		if err := tmpl.Execute(&out, view); err != nil {
			t.Fatal(err)
		}
		src := out.String()
		start := strings.Index(src, "Operations:")
		end := strings.Index(src, "\n Key:")
		if start < 0 || end < start {
			t.Fatalf("Operations field not found:\n%s", src)
		}
		value := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(src[start:end], "Operations:")), ",")
		expr, err := parser.ParseExpr(value)
		if err != nil {
			t.Fatalf("%v\n%s", err, value)
		}
		appends := 0
		ast.Inspect(expr, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if fn, ok := call.Fun.(*ast.Ident); ok && fn.Name == "append" {
				appends++
				if len(call.Args) < 2 {
					t.Errorf("append with no values (operations=%d):\n%s", len(operations), value)
				}
			}
			return true
		})
		if want := len(operations); want != appends {
			t.Errorf("operations=%d: %d append calls", want, appends)
		}
		if len(operations) > 0 && !strings.Contains(value, `Method:"PublishNote"`) {
			t.Errorf("custom operation missing:\n%s", value)
		}
	}
}
