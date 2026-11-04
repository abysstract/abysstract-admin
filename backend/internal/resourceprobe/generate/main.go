package main

import (
	"gorm.io/gen"

	"github.com/abysstract/abysstract-admin/backend/internal/resourceprobe/model"
)

func main() {
	g := gen.NewGenerator(gen.Config{OutPath: "./query"})
	g.ApplyBasic(model.Record{})
	g.Execute()
}
