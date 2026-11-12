package service

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

type LogService struct {
	repo   *repo.Repo
	access *accesslog.Logger
}

// NewLogService serves the operation, login and request logs. access may be
// nil when request logging is disabled.
func NewLogService(r *repo.Repo, access *accesslog.Logger) *LogService {
	return &LogService{repo: r, access: access}
}

func toProtoOperLog(l *model.SysOperLog) *adminv1.OperLog {
	return &adminv1.OperLog{
		Id:        l.ID,
		Title:     l.Title,
		Method:    l.Method,
		OperName:  l.OperName,
		OperId:    l.OperID,
		OperIp:    l.OperIP,
		Status:    int32(l.Status),
		ErrorMsg:  l.ErrorMsg,
		CostTime:  l.CostTime,
		OperParam: l.OperParam,
		OperTime:  timestamppb.New(l.OperTime),
	}
}

func toProtoLoginLog(l *model.SysLoginLog) *adminv1.LoginLog {
	return &adminv1.LoginLog{
		Id:        l.ID,
		Username:  l.Username,
		Status:    int32(l.Status),
		Ip:        l.IP,
		Msg:       l.Msg,
		LoginTime: timestamppb.New(l.LoginTime),
	}
}

func (s *LogService) ListOperLogs(
	ctx context.Context, req *adminv1.ListOperLogsRequest,
) (*adminv1.ListOperLogsResponse, error) {
	msg := req
	pageNum, pageSize := 1, 10
	if msg.Page != nil {
		pageNum, pageSize = int(msg.Page.PageNum), int(msg.Page.PageSize)
	}
	var err error
	pageNum, pageSize, err = normalizePageStrict(pageNum, pageSize)
	if err != nil {
		return nil, err
	}
	list, total, err := s.repo.ListOperLogs(ctx, msg.Keyword, pageNum, pageSize)
	if err != nil {
		return nil, toConnectError(err)
	}
	items := make([]*adminv1.OperLog, 0, len(list))
	for _, l := range list {
		items = append(items, toProtoOperLog(l))
	}
	return &adminv1.ListOperLogsResponse{
		Items: items,
		Page:  &adminv1.PageInfo{Total: total, PageNum: int32(pageNum), PageSize: int32(pageSize)},
	}, nil
}

func (s *LogService) CleanOperLogs(
	ctx context.Context, _ *adminv1.CleanOperLogsRequest,
) (*adminv1.CleanOperLogsResponse, error) {
	if err := s.repo.CleanOperLogs(ctx); err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.CleanOperLogsResponse{}, nil
}

func (s *LogService) ListLoginLogs(
	ctx context.Context, req *adminv1.ListLoginLogsRequest,
) (*adminv1.ListLoginLogsResponse, error) {
	msg := req
	pageNum, pageSize := 1, 10
	if msg.Page != nil {
		pageNum, pageSize = int(msg.Page.PageNum), int(msg.Page.PageSize)
	}
	var err error
	pageNum, pageSize, err = normalizePageStrict(pageNum, pageSize)
	if err != nil {
		return nil, err
	}
	list, total, err := s.repo.ListLoginLogs(ctx, msg.Keyword, pageNum, pageSize)
	if err != nil {
		return nil, toConnectError(err)
	}
	items := make([]*adminv1.LoginLog, 0, len(list))
	for _, l := range list {
		items = append(items, toProtoLoginLog(l))
	}
	return &adminv1.ListLoginLogsResponse{
		Items: items,
		Page:  &adminv1.PageInfo{Total: total, PageNum: int32(pageNum), PageSize: int32(pageSize)},
	}, nil
}

func (s *LogService) CleanLoginLogs(
	ctx context.Context, _ *adminv1.CleanLoginLogsRequest,
) (*adminv1.CleanLoginLogsResponse, error) {
	if err := s.repo.CleanLoginLogs(ctx); err != nil {
		return nil, toConnectError(err)
	}
	return &adminv1.CleanLoginLogsResponse{}, nil
}
