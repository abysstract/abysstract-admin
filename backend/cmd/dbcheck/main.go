// Command dbcheck 验证数据库连通性并列出现有表（临时工具）。
package main

import (
	"context"
	"fmt"
	"log"

	db "github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/internal/conf"
)

func main() {
	// 复用 conf.Load：自动加载 ./.env 或 ../.env，系统环境变量优先。
	cfg := conf.Load()
	if err := cfg.ValidateDatabase(); err != nil {
		log.Fatal(err)
	}

	g, err := db.Open(context.Background(), cfg.DBDriver, cfg.DBDSN)
	if err != nil {
		log.Fatalf("open failed: %v", err)
	}
	sqlDB, err := g.DB()
	if err != nil {
		log.Fatalf("get sql.DB failed: %v", err)
	}
	defer sqlDB.Close()
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("ping failed: %v", err)
	}
	fmt.Println("PING_OK")

	tables, err := g.Migrator().GetTables()
	if err != nil {
		log.Fatalf("list tables failed: %v", err)
	}
	fmt.Printf("existing public tables (%d):\n", len(tables))
	for _, t := range tables {
		fmt.Printf("  - %s\n", t)
	}
}
