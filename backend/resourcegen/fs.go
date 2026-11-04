package resourcegen

import (
	"io/fs"
	"os"
	"path/filepath"
)

type fileSystem interface {
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(name string, data []byte, perm os.FileMode) error
	ReadFile(name string) ([]byte, error)
	Remove(name string) error
	Stat(name string) (fs.FileInfo, error)
	Lstat(name string) (fs.FileInfo, error)
	Readlink(name string) (string, error)
	EvalSymlinks(path string) (string, error)
}

type osFS struct{}

func (osFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (osFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (osFS) Remove(name string) error {
	return os.Remove(name)
}

func (osFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

func (osFS) Lstat(name string) (fs.FileInfo, error) {
	return os.Lstat(name)
}

func (osFS) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

func (osFS) EvalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

type faultFS struct {
	base       fileSystem
	failWrite  int
	writes     int
	failRemove int
	removes    int
}

func (f *faultFS) MkdirAll(path string, perm os.FileMode) error {
	return f.base.MkdirAll(path, perm)
}

func (f *faultFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	f.writes++
	if f.failWrite > 0 && f.writes == f.failWrite {
		return fs.ErrInvalid
	}
	return f.base.WriteFile(name, data, perm)
}

func (f *faultFS) ReadFile(name string) ([]byte, error) {
	return f.base.ReadFile(name)
}

func (f *faultFS) Remove(name string) error {
	f.removes++
	if f.failRemove > 0 && f.removes == f.failRemove {
		return fs.ErrInvalid
	}
	return f.base.Remove(name)
}

func (f *faultFS) Stat(name string) (fs.FileInfo, error) {
	return f.base.Stat(name)
}

func (f *faultFS) Lstat(name string) (fs.FileInfo, error) {
	return f.base.Lstat(name)
}

func (f *faultFS) Readlink(name string) (string, error) {
	return f.base.Readlink(name)
}

func (f *faultFS) EvalSymlinks(path string) (string, error) {
	return f.base.EvalSymlinks(path)
}
