//go:build integration

package resourcegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gen"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
)

// Exercise the same template against the existing bool/int16/enum fixture.
// The query API is generated from its model, independently of Resource output.
func TestGenQuerySecondModel(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(filepath.Join(root, "internal", "resource"), "genquery-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	queryDir := filepath.Join(dir, "query")
	queryGenerator := gen.NewGenerator(gen.Config{OutPath: queryDir})
	queryGenerator.ApplyBasic(schematest.BoolModel{})
	queryGenerator.Execute()
	plan, err := schematest.BoolCompileSampleResource().Build()
	if err != nil {
		t.Fatal(err)
	}
	cfg := fixtureConfig()
	cfg.GenQueryImport = "github.com/abysstract/abysstract-admin/backend/internal/resource/" + filepath.Base(dir) + "/query"
	files, err := (&generator{cfg: cfg}).renderGo(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, file.name), file.content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "gen_query_test.go"), []byte(genQueryRuntime), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-v", "./internal/resource/"+filepath.Base(dir))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("second model: %v\n%s", err, out)
	}
	t.Log(string(out))
	// The same generated assignment rejects a different model field's type.
	for _, file := range files {
		if !strings.HasSuffix(file.name, "_repo.gen.go") {
			continue
		}
		for _, pair := range [][2]string{
			{"item.Enabled.Value(m.Enabled)", "item.Enabled.Value(m.ID)"},
			{"item.Enabled.Value(m.Enabled)", "item.Enabled.Value(m.NarrowValue)"},
		} {
			broken := strings.Replace(string(file.content), pair[0], pair[1], 1)
			if broken == string(file.content) {
				t.Fatalf("assignment missing: %s", pair[0])
			}
			if err := os.WriteFile(filepath.Join(dir, file.name), []byte(broken), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("go", "test", "-run", "^$", "./internal/resource/"+filepath.Base(dir))
			cmd.Dir = root
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "cannot use") {
				t.Fatalf("wrong assignment compiled or failed for another reason: %v\n%s", err, out)
			}
		}
		if err := os.WriteFile(filepath.Join(dir, file.name), file.content, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

const genQueryRuntime = `package gen
import (
 "context"
 "testing"
 "github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
 "github.com/abysstract/abysstract-admin/backend/internal/testutil"
 "github.com/abysstract/abysstract-admin/backend/resource"
)
func TestGeneratedGenBoolAndNarrow(t *testing.T) {
 db := testutil.OpenDB(t)
 testutil.ResetSchema(t,db)
 if err := db.AutoMigrate(&schematest.BoolModel{}); err != nil { t.Fatal(err) }
 store,err := resource.NewStore(db); if err != nil { t.Fatal(err) }
 m := &schematest.BoolModel{Enabled:true, NarrowValue:7, Status:1}
 if err := db.Create(m).Error; err != nil { t.Fatal(err) }
 repo := NewBoolRepository()
 ctx := context.Background()
 all := resource.UnrestrictedAccessScope()
 enabled,narrow,status := true,int16(7),int16(1)
 rows,total,err := repo.List(ctx,store,BoolListQuery{Enabled:&enabled,NarrowValue:&narrow,Status:&status},all)
 if err != nil || len(rows)!=1 || total!=1 { t.Fatalf("list: %v %d %v",rows,total,err) }
 m.Enabled=false
 if err := store.InTransaction(ctx,func(tx resource.Tx)error {
   return repo.Update(ctx,tx,m.ID,m,resource.NewColumnSet(BoolColumnEnabled),all)
 }); err != nil { t.Fatal(err) }
 var saved schematest.BoolModel
 if err := db.First(&saved,m.ID).Error; err != nil { t.Fatal(err) }
 if saved.Enabled || saved.NarrowValue!=7 || saved.Status!=1 || saved.UpdatedAt.IsZero() { t.Fatalf("saved: %+v",saved) }
 enabled=false
 rows,total,err=repo.List(ctx,store,BoolListQuery{Enabled:&enabled,NarrowValue:&narrow},all)
 if err!=nil || len(rows)!=1 || total!=1 { t.Fatalf("zero filters: %v %d %v",rows,total,err) }
}
`
