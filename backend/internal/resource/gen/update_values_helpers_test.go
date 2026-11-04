package gen_test

import (
	"context"
	"time"

	"gorm.io/gorm/logger"
)

type updateSQLCapture struct {
	logger.Interface
	statements []string
}

func (l *updateSQLCapture) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	l.statements = append(l.statements, sql)
}
