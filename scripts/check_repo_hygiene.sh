#!/usr/bin/env bash
# 仓库卫生校验（需求 11.3 / 11.4 / 11.5）。
# 断言：已跟踪文件集合不含匹配以下任意规则的文件：
#   - **/*.tsbuildinfo（TypeScript 增量构建缓存，任意层级）
#   - frontend/dist/ 下文件（前端构建产物）
#   - frontend/node_modules/ 下文件（前端依赖）
#   - backend/bin/ 下文件（后端构建产物）
#
# 退出码：0 通过；1 失败（stdout 列出违规文件）。
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

violations=0

check_pattern() {
    local label="$1"
    local pattern="$2"
    local found
    found="$(git ls-files "$pattern" || true)"
    if [ -n "$found" ]; then
        echo "::error::仓库卫生校验失败：以下已跟踪文件命中忽略规则「$label」："
        echo "$found" | sed 's/^/  /'
        echo "请执行：git rm --cached <file> 后重新提交"
        violations=$((violations + 1))
    fi
}

# 需求 11.1 / 11.3：任意层级 tsbuildinfo
check_pattern "tsbuildinfo" "**/*.tsbuildinfo"
# git ls-files 的 ** 语义需配合 pathspec；若不生效，回退到 grep 过滤。
if git ls-files | grep -qE '(^|/)\.tsbuildinfo$|\.tsbuildinfo$'; then
    echo "::error::检测到 .tsbuildinfo 被跟踪"
    violations=$((violations + 1))
fi

# 需求 11.2 / 11.3：构建产物目录
check_pattern "frontend/dist/" "frontend/dist/*"
check_pattern "frontend/node_modules/" "frontend/node_modules/*"
check_pattern "backend/bin/" "backend/bin/*"

if [ "$violations" -gt 0 ]; then
    echo "::error::仓库卫生校验未通过（共 $violations 类违规）"
    exit 1
fi

echo "✓ 仓库卫生校验通过：已跟踪集合不含构建产物"
exit 0
