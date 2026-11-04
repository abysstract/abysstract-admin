# 贡献指南

欢迎提交缺陷修复、功能改进和文档调整。

## 准备开发环境

首次启动按 [README 快速开始](README.md#快速开始体验自带后台)操作。迁移、模型、Proto 或 Resource 的修改与生成方式统一见[开发说明](docs/development.md)。

生成文件通过源定义和对应生成命令更新。提交时同时包含源文件及其生成结果。业务接入方式见[模块开发教程](docs/new-module-tutorial.md)，字段和回调规则见 [Resource 手册](docs/ai-cookbook.md)。商品模块的手写扩展见[商品参考](docs/admin-core-getting-started.md)。

## 提交信息

采用 Conventional Commits：

```text
<type>(<scope>): <subject>
```

常用类型为 `feat`、`fix`、`docs`、`refactor`、`test`、`chore` 和 `ci`。例如：

```text
fix(auth): 修复刷新令牌过期判断
docs(readme): 补充首次启动步骤
```

破坏性变更使用 `!`，或在提交说明中填写 `BREAKING CHANGE:`，说明影响与升级方式。

## 提交 Pull Request

1. 从 `main` 创建工作分支，完成范围明确的修改。
2. 根据改动选择验证方式，检查命令见[开发说明](docs/development.md#提交前检查)。
3. 提交 PR，说明问题、修改后的行为、影响范围和实际验证结果。
4. 根据评审意见调整，完成后由维护者合并。

安全问题通过 [SECURITY.md](SECURITY.md) 中的私密渠道报告。
