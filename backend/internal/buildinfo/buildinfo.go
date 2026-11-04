// Package buildinfo 暴露构建版本信息，由 ldflags 在构建时注入。
//
// 用法：
//
//	go build -ldflags "-X github.com/abysstract/abysstract-admin/backend/internal/buildinfo.Version=v1.0.0 \
//	                   -X .../buildinfo.Commit=$(git rev-parse --short HEAD)" \
//	         ./cmd/server
//
// 未注入时默认 "dev" / "unknown"，便于本地开发无需额外参数即可启动。
package buildinfo

import "fmt"

// 这些变量由 ldflags 在构建时注入；默认值仅用于本地开发。
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// Info 返回当前构建的版本信息。
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Current 返回当前构建信息。
func Current() Info {
	return Info{Version: Version, Commit: Commit, Date: Date}
}

// String 返回可读的版本字符串。
func (i Info) String() string {
	return fmt.Sprintf("%s (%s) built %s", i.Version, i.Commit, i.Date)
}
