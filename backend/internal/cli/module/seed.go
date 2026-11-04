package module

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/abysstract/abysstract-admin/backend/moduleseed"
)

type seedSnapshot struct {
	Action   string           `json:"action"`
	Seed     moduleseed.Spec  `json:"seed"`
	Previous *moduleseed.Spec `json:"previous,omitempty"`
}

func prepareSeed(root, action string, d descriptor) (map[string][]byte, error) {
	snapshot, changed, err := prepareSeedSnapshot(root, action, d)
	if err != nil || !changed {
		return nil, err
	}
	// A single timestamp is shared by all dialects; choose a later version without sleeping.
	stamp := time.Now().UTC().Truncate(time.Second)
	files, err := filepath.Glob(filepath.Join(root, "backend/migrations/*/*"))
	if err != nil {
		return nil, err
	}
	for _, path := range files {
		part, _, ok := strings.Cut(filepath.Base(path), "_")
		if !ok || len(part) != 14 {
			continue
		}
		t, err := time.Parse("20060102150405", part)
		if err != nil {
			return nil, err
		}
		if !stamp.After(t) {
			stamp = t.Add(time.Second)
		}
	}
	version := stamp.Format("20060102150405")
	data, err := jsonBytes(snapshot)
	if err != nil {
		return nil, err
	}
	seed, err := jsonBytes(snapshot.Seed)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{"backend/migrations/modules/" + version + "_" + d.Name + "_" + snapshot.Action + ".json": data}
	number, err := strconv.ParseInt(version, 10, 64)
	if err != nil {
		return nil, err
	}
	previousArg := ""
	if snapshot.Previous != nil {
		before, err := jsonBytes(snapshot.Previous)
		if err != nil {
			return nil, err
		}
		previousArg = "," + strconv.Quote(string(before))
	}
	for _, driver := range []string{"postgres", "mysql", "mariadb", "sqlite"} {
		src := fmt.Sprintf(`package %s
import (
 "github.com/pressly/goose/v3"
 "github.com/abysstract/abysstract-admin/backend/moduleseed"
)
func Module%s%sMigration() *goose.Migration {
 return moduleseed.Migration(%d,%q,%q,%q%s)
}
`, driver, strings.ToUpper(d.Name[:1])+d.Name[1:], version, number, driver, snapshot.Action, string(seed), previousArg)
		out["backend/migrations/"+driver+"/"+version+"_module_"+d.Name+"_"+snapshot.Action+".go"], err = goSource(src)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// equalSeed treats an omitted optional roles list and an empty list identically.
func equalSeed(a, b moduleseed.Spec) bool {
	if len(a.Roles) == 0 {
		a.Roles = nil
	}
	if len(b.Roles) == 0 {
		b.Roles = nil
	}
	return reflect.DeepEqual(a, b)
}

func prepareSeedSnapshot(root, action string, d descriptor) (seedSnapshot, bool, error) {
	var snapshot seedSnapshot
	snapshot.Action = map[string]string{"add": "install", "remove": "remove", "upgrade": "upgrade"}[action]
	previous, err := filepath.Glob(filepath.Join(root, "backend/migrations/modules/*_"+d.Name+"_*.json"))
	if err != nil {
		return seedSnapshot{}, false, err
	}
	sort.Strings(previous)
	if action == "add" || action == "upgrade" {
		if d.Seed == "" {
			return snapshot, false, nil
		}
		data, err := os.ReadFile(filepath.Join(root, d.Seed))
		if err != nil {
			return seedSnapshot{}, false, err
		}
		snapshot.Seed, err = moduleseed.Parse(data)
		if err != nil {
			return seedSnapshot{}, false, err
		}
		if snapshot.Seed.Module != d.Name {
			return seedSnapshot{}, false, fmt.Errorf("seed module %s differs from %s", snapshot.Seed.Module, d.Name)
		}
		if action == "upgrade" {
			return prepareSeedUpgrade(d.Name, previous, snapshot)
		}
	} else {
		if len(previous) == 0 {
			return snapshot, false, nil
		}
		var last seedSnapshot
		if err := readJSON(previous[len(previous)-1], &last); err != nil {
			return seedSnapshot{}, false, err
		}
		if last.Action == "remove" {
			return seedSnapshot{}, false, fmt.Errorf("module %s has no active installation snapshot", d.Name)
		}
		snapshot.Seed = last.Seed
	}

	return snapshot, true, nil
}

func prepareSeedUpgrade(name string, previous []string, snapshot seedSnapshot) (seedSnapshot, bool, error) {
	if len(previous) == 0 {
		return seedSnapshot{}, false, fmt.Errorf("module %s has no installation snapshot", name)
	}
	var last seedSnapshot
	if err := readJSON(previous[len(previous)-1], &last); err != nil {
		return seedSnapshot{}, false, err
	}
	if last.Action == "remove" {
		return seedSnapshot{}, false, fmt.Errorf("module %s is uninstalled", name)
	}
	if equalSeed(last.Seed, snapshot.Seed) {
		return snapshot, false, nil
	}
	snapshot.Previous = &last.Seed
	return snapshot, true, nil
}
