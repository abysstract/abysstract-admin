package repo

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// ---- 操作日志 ----

func (r *Repo) AddOperLog(ctx context.Context, l *model.SysOperLog) error {
	if l.OperTime.IsZero() {
		l.OperTime = NormalizeTimestamp(time.Now())
	}
	l.OperTime = NormalizeTimestamp(l.OperTime)
	return r.db.WithContext(ctx).Create(l).Error
}

func (r *Repo) ListOperLogs(ctx context.Context, keyword string, pageNum, pageSize int) ([]*model.SysOperLog, int64, error) {
	pageNum, pageSize = normalizePage(pageNum, pageSize)
	q := r.db.WithContext(ctx).Model(&model.SysOperLog{})
	q = Search([]Column{"oper_name", "title"}, keyword).Apply(q)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*model.SysOperLog
	if err := q.Order("oper_time DESC").Order("id ASC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *Repo) CleanOperLogs(ctx context.Context) error {
	return r.db.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.SysOperLog{}).Error
}

// ---- 登录日志 ----

func (r *Repo) AddLoginLog(ctx context.Context, l *model.SysLoginLog) error {
	if l.LoginTime.IsZero() {
		l.LoginTime = NormalizeTimestamp(time.Now())
	}
	l.LoginTime = NormalizeTimestamp(l.LoginTime)
	return r.db.WithContext(ctx).Create(l).Error
}

func (r *Repo) ListLoginLogs(ctx context.Context, keyword string, pageNum, pageSize int) ([]*model.SysLoginLog, int64, error) {
	pageNum, pageSize = normalizePage(pageNum, pageSize)
	q := r.db.WithContext(ctx).Model(&model.SysLoginLog{})
	q = Search([]Column{"username"}, keyword).Apply(q)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []*model.SysLoginLog
	if err := q.Order("login_time DESC").Order("id ASC").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *Repo) CleanLoginLogs(ctx context.Context) error {
	return r.db.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.SysLoginLog{}).Error
}
