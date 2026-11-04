#!/usr/bin/env bash
# 发布材料校验（需求 1.1–1.8）。
# 断言下列文件存在并满足结构化条件：
#   - LICENSE：标准 Apache License 2.0 全文（不加 SPDX 头，便于 GitHub 识别许可证）
#   - CONTRIBUTING.md：链接开发说明与快速开始，并包含提交规范和 PR 流程
#   - SECURITY.md：含「以天为单位」或具体响应时限 SLA 描述 + 上报渠道
#   - CHANGELOG.md：含 SemVer 版本号（\d+\.\d+\.\d+）与 YYYY-MM-DD 日期
#   - .github/ISSUE_TEMPLATE/（至少一个模板）与 .github/PULL_REQUEST_TEMPLATE.md
#   - README.md：引用 LICENSE/CONTRIBUTING/SECURITY/CHANGELOG 的相对链接目标存在
#
# 退出码：0 通过；1 失败。
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

errors=0

require_file() {
    local f="$1"
    if [ ! -f "$f" ]; then
        echo "::error::缺少必需文件：$f"
        errors=$((errors + 1))
        return 1
    fi
    return 0
}

require_content() {
    local f="$1"
    local label="$2"
    local pattern="$3"
    if ! grep -qE "$pattern" "$f"; then
        echo "::error::$f 缺少必要内容「$label」（模式: $pattern）"
        errors=$((errors + 1))
    fi
}

# 1.1 LICENSE（Apache-2.0 标准全文）
require_file LICENSE
if [ -f LICENSE ]; then
    require_content LICENSE "Apache License 标题" "^[[:space:]]*Apache License$"
    require_content LICENSE "版本 2.0" "^[[:space:]]*Version 2\.0, January 2004$"
fi

# 1.2 CONTRIBUTING 四部分
require_file CONTRIBUTING.md
if [ -f CONTRIBUTING.md ]; then
    require_content CONTRIBUTING.md "开发说明入口" 'docs/development\.md'
    require_content CONTRIBUTING.md "快速开始入口" 'README\.md#'
    require_content CONTRIBUTING.md "提交规范" "(提交规范|Conventional Commits|commit)"
    require_content CONTRIBUTING.md "PR 流程" "(PR 流程|Pull Request|pull request)"
fi

# 1.3 SECURITY 上报渠道 + 响应时限
require_file SECURITY.md
if [ -f SECURITY.md ]; then
    require_content SECURITY.md "上报渠道" "(邮件|email|漏洞|report| disclose|披露)"
    require_content SECURITY.md "响应时限（以天为单位）" "([0-9]+\s*(天|day|days|小时|hour|hours))"
fi

# 1.4 CHANGELOG SemVer + 日期
require_file CHANGELOG.md
if [ -f CHANGELOG.md ]; then
    require_content CHANGELOG.md "SemVer 版本号" "\[[0-9]+\.[0-9]+\.[0-9]+\]"
    require_content CHANGELOG.md "YYYY-MM-DD 日期" "[0-9]{4}-[0-9]{2}-[0-9]{2}"
fi

# 1.5 Issue 模板
issue_count=$(find .github/ISSUE_TEMPLATE -type f 2>/dev/null | wc -l | tr -d ' ')
if [ "$issue_count" -lt 1 ]; then
    echo "::error::.github/ISSUE_TEMPLATE/ 至少需要一个模板文件"
    errors=$((errors + 1))
fi

# 1.6 PR 模板
require_file .github/PULL_REQUEST_TEMPLATE.md

# 1.7 / 1.8 README 相对链接目标存在
if [ -f README.md ]; then
    # 提取形如 [text](LICENSE) / [text](CONTRIBUTING.md) 的相对链接。
    # 仅检查指向仓库内文件的纯相对链接（不含 ://）。
    while IFS= read -r link; do
        # link 形如 "LICENSE" 或 "CONTRIBUTING.md"，可能带 #anchor
        target="${link%%#*}"
        target="${target%%\?*}"
        if [ -z "$target" ]; then
            continue
        fi
        if [ ! -e "$target" ]; then
            echo "::error::README 中引用的相对链接目标不存在：$link"
            errors=$((errors + 1))
        fi
    done < <(grep -oE '\]\([^)]+\)' README.md | sed -E 's/^\]\(//; s/\)$//;' | grep -vE '^[a-zA-Z]+://' || true)
fi

if [ "$errors" -gt 0 ]; then
    echo "::error::发布材料校验未通过（共 $errors 处问题）"
    exit 1
fi

echo "✓ 发布材料校验通过"
exit 0
