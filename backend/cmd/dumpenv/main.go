// Command dumpenv 使用与后端 conf.Load 一致的 godotenv 语义加载 .env，
// 然后输出可被 shell source 的 KEY=VALUE 行，供 scripts/dev.sh 等聚合脚本使用。
//
// 与 godotenv.Load 同语义：已存在的环境变量优先，.env 仅补充未设置的项。
// 处理 godotenv 支持的完整语法：export 前缀、键名前后空白、行内注释、引号、转义等。
//
// 用法：
//
//	eval "$(./backend/bin/dumpenv path/to/.env)"
//	eval "$(go run ./backend/cmd/dumpenv path/to/.env)"
//
// 行为：
//   - 缺省参数 path 默认 ./.env
//   - 文件不存在 → 输出 stderr 警告并以 0 退出（不阻断聚合脚本）
//   - 已设置的环境变量不会被 .env 覆盖（与 godotenv.Load 一致）
//   - 每行输出 KEY=VALUE，VALUE 用单引号包裹并转义内部单引号（防 shell 注入）
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	path := ".env"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	// 文件不存在时静默退出：聚合脚本可能在 .env 尚未创建时调用。
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "dumpenv: %s 不存在，跳过\n", path)
		return
	}

	// godotenv.Read 不写回环境变量，只返回解析结果 map。
	// 自己实现「已设置不覆盖」语义，避免污染 dumpenv 自身进程环境。
	parsed, err := godotenv.Read(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dumpenv: 解析 %s 失败: %v\n", path, err)
		os.Exit(1)
	}

	// 稳定输出顺序：避免 map 遍历顺序导致 source 行抖动。
	keys := make([]string, 0, len(parsed))
	for k := range parsed {
		keys = append(keys, k)
	}
	// 简单字典序，足够可预测。
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	for _, k := range keys {
		// 仅导出当前未设置的变量（godotenv.Load 语义）。
		if _, set := os.LookupEnv(k); set {
			continue
		}
		// 单引号包裹值，并把值内的单引号转义为 '\''
		// （POSIX shell 单引号中唯一需要的转义）。
		v := strings.ReplaceAll(parsed[k], "'", `'\''`)
		fmt.Printf("%s='%s'\n", k, v)
	}
}
