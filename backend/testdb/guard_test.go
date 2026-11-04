package testdb

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"gorm.io/gorm"
)

const runID = "0123456789abcdef0123456789abcdef"

func TestForDatabaseRoundTrip(t *testing.T) {
	name, _ := DatabaseName(runID)
	for _, base := range []string{
		"postgres://user:p%40ss@localhost:5433/abysstract_admin?sslmode=disable&application_name=probe",
		"postgresql://user:pass@localhost/old?dbname=abysstract_admin&sslmode=require",
		"host=localhost port=5433 user=user password='space \\' quoted' dbname=abysstract_admin sslmode=disable application_name=probe",
	} {
		t.Run(base[:8], func(t *testing.T) {
			old, err := pgx.ParseConfig(base)
			if err != nil {
				t.Fatal(err)
			}
			c, err := ForDatabase(base, name, runID)
			if err != nil {
				t.Fatal(err)
			}
			got, err := pgx.ParseConfig(c.DSN)
			if err != nil {
				t.Fatal(err)
			}
			if got.Database != name || got.Host != old.Host || got.Port != old.Port || got.User != old.User || got.Password != old.Password || got.RuntimeParams["application_name"] != old.RuntimeParams["application_name"] {
				t.Fatal("round trip lost target or connection options")
			}
			if (got.TLSConfig == nil) != (old.TLSConfig == nil) {
				t.Fatal("TLS mode changed")
			}
			old.Database = name
			broken, err := pgx.ParseConfig(old.ConnString())
			if err != nil {
				t.Fatal(err)
			}
			if broken.Database == name {
				t.Fatal("expected historical ConnString mismatch")
			}
			invalid := c
			invalid.DSN = old.ConnString()
			if invalid.Validate() == nil {
				t.Fatal("historical wrong DSN was admitted")
			}
		})
	}
}

func TestAdmissionRejectsBeforeDial(t *testing.T) {
	name, _ := DatabaseName(runID)
	valid, _ := ForDatabase("host=localhost dbname=old sslmode=disable", name, runID)
	for _, c := range []Config{
		{}, {DSN: valid.DSN, Database: name},
		{DSN: valid.DSN, Database: "abysstract_admin", RunID: runID},
		{DSN: "host=localhost dbname=abysstract_admin", Database: name, RunID: runID},
		{DSN: "postgres://:bad%secret", Database: name, RunID: runID},
	} {
		calls := 0
		_, err := open(c, func(string) (*gorm.DB, error) { calls++; return nil, errors.New("unexpected dial") }, func(*gorm.DB) error { calls++; return nil })
		if err == nil || calls != 0 {
			t.Fatalf("rejected config dialed: calls=%d err=%v", calls, err)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatal("error exposed DSN")
		}
	}
}

func TestDedicatedEnvironment(t *testing.T) {
	t.Setenv("ADMIN_TEST_DRIVER", "postgres")
	t.Setenv(DSNEnv, "")
	t.Setenv(DatabaseEnv, "")
	t.Setenv(RunIDEnv, "")
	t.Setenv("ADMIN_DB_DRIVER", "postgres")
	t.Setenv("ADMIN_DB_DSN", "host=localhost dbname=abysstract_admin")
	t.Setenv("ADMIN_LIBRARY_TEST_DSN", "host=localhost dbname=abysstract_admin")
	if _, err := FromEnv(); err == nil {
		t.Fatal("application or legacy DSN supplied test defaults")
	}
	name, _ := DatabaseName(runID)
	c, err := ForDatabase("host=localhost dbname=old", name, runID)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(DSNEnv, c.DSN)
	t.Setenv(DatabaseEnv, name)
	t.Setenv(RunIDEnv, runID)
	if got, err := FromEnv(); err != nil || got != c {
		t.Fatalf("dedicated config mismatch: %v", err)
	}
}

var catalogCounter atomic.Uint64
