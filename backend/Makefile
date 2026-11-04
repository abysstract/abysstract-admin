.PHONY: generate tidy run build test test-integration fmt lint dev-up

# 生成 proto 代码（需先安装 buf）：connect-go + connect-es
generate:
	cd ../proto && buf generate

tidy:
	go mod tidy

run:
	go run ./cmd/server

build:
	go build -o bin/admin ./cmd/server

# fmt 规范化所有 .go 文件的格式与 import 分组（gofmt -s 简化语法 + goimports 分组）。
# 本地写完代码后建议先跑一次，避免提交触发 husky / CI 的格式校验失败。
fmt:
	gofmt -w -s .
	goimports -w -local github.com/abysstract/abysstract-admin/backend .

# lint 跑 golangci-lint 聚合检查（需先安装：见 docs/development.md）。
lint:
	golangci-lint run ./...

test:
	GOWORK=off go test ./...

test-integration:
	GOWORK=off go test -tags=integration -p 1 ./...

# 一键开发环境：docker compose → 迁移 → 后端，仅在前一步成功后继续。
dev-up:
	cd .. && docker compose up -d --wait
	go run ./cmd/migrate
	go run ./cmd/server
