//go:build integration

package resourcegen

import (
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
)

func TestResourceDBNameConverterCompileSample(t *testing.T) {
	p, err := schematest.RenamedColumnsFixtureResource().Build()
	if err != nil {
		t.Fatal(err)
	}
	compileWidgetPlanWithRuntime(t, p, `package gen

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	genquery "$GEN_QUERY_IMPORT"
 "github.com/abysstract/abysstract-admin/backend/resource"
 "gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type sqlCapture struct {
	logger.Interface
	sql []string
}

func (l *sqlCapture) LogMode(level logger.LogLevel) logger.Interface { return l }
func (l *sqlCapture) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	l.sql = append(l.sql, sql)
}

func TestDBNameGenAssignmentSQL(t *testing.T) {
	capture := &sqlCapture{Interface: logger.Default.LogMode(logger.Info)}
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := db.AutoMigrate(&schematest.RenamedColumnsFixtureModel{}); err != nil {
		t.Fatal(err)
	}
	db = db.Session(&gorm.Session{Logger: capture})
	if err := db.Create(&schematest.RenamedColumnsFixtureModel{ID: 1}).Error; err != nil {
		t.Fatal(err)
	}
	v := "x"
	model, cols, err := RenamedColumnsFixtureUpdateToModel(&schematest.RenamedColumnsFixtureModel{}, &testpb.UpdateRenamedColumnsRequest{DisplayName: &v})
	if err != nil {
		t.Fatal(err)
	}
	if got := cols.Values(); len(got) != 1 || got[0] != repo.Column("name") {
		t.Fatalf("columns=%v", got)
	}
	// This converter-only fixture consumes its formally generated query directly.
	model.UpdatedAt = resource.NormalizeTimestamp(time.Now())
	item := genquery.Use(db).RenamedColumnsFixtureModel
	result, err := item.WithContext(context.Background()).Where(item.ID.Eq(1)).UpdateColumnSimple(
		item.DisplayName.Value(model.DisplayName), item.UpdatedAt.Value(model.UpdatedAt))
	if err != nil || result.RowsAffected != 1 { t.Fatalf("update: %+v %v",result,err) }
	var saved schematest.RenamedColumnsFixtureModel
	if err := db.First(&saved,1).Error;err != nil {t.Fatal(err)}
	if saved.DisplayName != v || !saved.UpdatedAt.Equal(model.UpdatedAt) {t.Fatalf("saved: %+v",saved)}
	sql := strings.Join(capture.sql, "\n")
	if !strings.Contains(sql, "UPDATE") || !strings.Contains(sql, "name") || strings.Contains(sql, "display_name") {
		t.Fatalf("expected DBName SQL, got %s", sql)
	}
}
`)
}

func TestGeneratedUpdateWithUnchangedAudit(t *testing.T) {
	definition := schematest.WidgetFixtureResource()
	p, err := definition.Build()
	if err != nil {
		t.Fatal(err)
	}
	compileWidgetPlanWithRuntime(t, p, `package gen
import (
 "context"
 "errors"
 "testing"

 mysqldriver "github.com/go-sql-driver/mysql"
 "github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
 "github.com/abysstract/abysstract-admin/backend/internal/testutil"
 "github.com/abysstract/abysstract-admin/backend/resource"
 "github.com/abysstract/abysstract-admin/backend/testdb"
 "gorm.io/gorm"
)
func TestUnchangedAuditAndFoundRows(t *testing.T) {
 cfg,err:=testdb.FromEnv();if err!=nil{t.Fatal(err)}
 flags:=[]bool{false};if (cfg.Driver == "mysql" || cfg.Driver == "mariadb"){flags=append(flags,true)}
 for _,found:=range flags {
  local:=cfg
  if (cfg.Driver == "mysql" || cfg.Driver == "mariadb") {dsn,err:=mysqldriver.ParseDSN(cfg.DSN);if err!=nil{t.Fatal(err)};dsn.ClientFoundRows=found;local.DSN=dsn.FormatDSN()}
  db,err:=testdb.Open(local);if err!=nil{t.Fatal(err)};pool,err:=db.DB();if err!=nil{t.Fatal(err)}
  testutil.ResetSchema(t,db)
  timeType:="TIMESTAMPTZ";if cfg.Driver=="sqlite"{timeType="DATETIME"};if (cfg.Driver == "mysql" || cfg.Driver == "mariadb"){timeType="DATETIME(6)"}
  if err:=db.Exec("CREATE TABLE test_widget(id BIGINT PRIMARY KEY,name VARCHAR(100),enabled BOOLEAN,updated_at "+timeType+",dept_id BIGINT,create_by BIGINT)").Error;err!=nil{t.Fatal(err)}
  // Keep the audit value fixed in the database, so a same-value generated
  // UPDATE exercises changed-rows=0 independently of the audit assignment.
  if (cfg.Driver == "mysql" || cfg.Driver == "mariadb") {
   if err:=db.Exec("CREATE TRIGGER fixed_audit BEFORE UPDATE ON test_widget FOR EACH ROW SET NEW.updated_at=OLD.updated_at").Error;err!=nil{t.Fatal(err)}
  } else if cfg.Driver=="sqlite" {
   if err:=db.Exec("CREATE TRIGGER fixed_audit AFTER UPDATE ON test_widget FOR EACH ROW BEGIN UPDATE test_widget SET updated_at=OLD.updated_at WHERE id=NEW.id; END").Error;err!=nil{t.Fatal(err)}
  } else {
   if err:=db.Exec("CREATE FUNCTION fixed_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.updated_at=OLD.updated_at; RETURN NEW; END $$").Error;err!=nil{t.Fatal(err)}
   if err:=db.Exec("CREATE TRIGGER fixed_audit BEFORE UPDATE ON test_widget FOR EACH ROW EXECUTE FUNCTION fixed_audit()").Error;err!=nil{t.Fatal(err)}
  }
  row:=schematest.WidgetModel{ID:1,Name:"stable"}
  if err:=db.Create(&row).Error;err!=nil{t.Fatal(err)}
  var before schematest.WidgetModel;if err:=db.First(&before,row.ID).Error;err!=nil{t.Fatal(err)}
  native:=db.Model(&schematest.WidgetModel{}).Where("id=?",row.ID).UpdateColumns(map[string]any{"name":before.Name,"updated_at":before.UpdatedAt})
  if native.Error!=nil{t.Fatal(native.Error)}
  want:=int64(1);if (cfg.Driver == "mysql" || cfg.Driver == "mariadb")&&!found{want=0};if native.RowsAffected!=want{t.Fatalf("foundRows=%v rows=%d",found,native.RowsAffected)}
  store,err:=resource.NewStore(db);if err!=nil{t.Fatal(err)}
  repo:=NewWidgetRepository();ctx:=context.Background();all:=resource.UnrestrictedAccessScope()
  if err:=store.InTransaction(ctx,func(tx resource.Tx)error{return repo.Update(ctx,tx,row.ID,&before,resource.NewColumnSet("name"),all)});err!=nil{t.Fatal(err)}
  if !before.UpdatedAt.Equal(row.UpdatedAt){t.Fatal("same update changed audit")}
  before.Name=""
  if err:=store.InTransaction(ctx,func(tx resource.Tx)error{return repo.Update(ctx,tx,row.ID,&before,resource.NewColumnSet("name"),all)});err!=nil{t.Fatal(err)}
  var saved schematest.WidgetModel;if err:=db.First(&saved,row.ID).Error;err!=nil{t.Fatal(err)};if saved.Name!=""||!saved.UpdatedAt.Equal(before.UpdatedAt){t.Fatal("zero value or audit changed")}
  for _,key:=range []int64{row.ID,-1}{
   access:=all;if key==row.ID{access=resource.NewAccessScope(func(db *gorm.DB)*gorm.DB{return db.Where("id=-1")})}
   err:=store.InTransaction(ctx,func(tx resource.Tx)error{return repo.Update(ctx,tx,key,&before,resource.NewColumnSet("name"),access)})
   if !errors.Is(err,resource.ErrNotFound){t.Fatalf("key=%d err=%v",key,err)}
  }
  if err:=pool.Close();err!=nil{t.Fatal(err)}
 }
}
`)
}
