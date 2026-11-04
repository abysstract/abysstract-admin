package coremigrations

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
)

var sourceName = regexp.MustCompile(`^(\d{14})_([a-zA-Z0-9_]+)\.(sql|go\.tmpl)$`)
var coreIdentity = regexp.MustCompile(`^core_(\d{14})_.+$`)

// Sync imports missing Core sources into dir without changing existing migrations.
// Returned paths are the newly created files in execution order.
func Sync(root, driver string) ([]string, error) {
	sources, err := SourcesFor(driver)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, driver)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return syncSources(dir, sources)
}

func syncSources(dir string, sources fs.FS) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	imported := make(map[string]bool)
	var last int64
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		version, err := goose.NumericComponent(entry.Name())
		if err != nil {
			continue
		}
		if version > last {
			last = version
		}
		_, description, _ := strings.Cut(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), "_")
		if match := coreIdentity.FindStringSubmatch(description); match != nil {
			imported[match[1]] = true
		}
	}
	items, err := fs.ReadDir(sources, ".")
	if err != nil {
		return nil, err
	}
	var added []string
	for _, item := range items {
		name := item.Name()
		match := sourceName.FindStringSubmatch(name)
		if match == nil || imported[match[1]] {
			continue
		}
		content, err := fs.ReadFile(sources, name)
		if err != nil {
			return added, err
		}
		// goose timestamps have second precision; wait for an actual later UTC second.
		var version string
		var number int64
		for {
			now := time.Now().UTC()
			version = now.Format("20060102150405")
			number, err = strconv.ParseInt(version, 10, 64)
			if err != nil {
				return added, err
			}
			if number > last {
				break
			}
			previous, err := time.Parse("20060102150405", strconv.FormatInt(last, 10))
			if err != nil {
				return added, err
			}
			if previous.After(now) {
				return added, fmt.Errorf("local migration version %d is in the future", last)
			}
			time.Sleep(time.Until(now.Truncate(time.Second).Add(time.Second)))
		}
		ext := filepath.Ext(strings.TrimSuffix(name, ".tmpl"))
		filename := version + "_core_" + match[1] + "_" + match[2] + ext
		path := filepath.Join(dir, filename)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return added, err
		}
		_, writeErr := file.Write([]byte(strings.ReplaceAll(string(content), "__CORE_VERSION__", version)))
		closeErr := file.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return added, errors.Join(err, os.Remove(path))
		}
		added = append(added, path)
		last = number
		imported[match[1]] = true
	}
	return added, nil
}
