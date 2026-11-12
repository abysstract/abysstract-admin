package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

// AccessLogListPerm is the permission of the request log page and its RPCs.
const AccessLogListPerm = "system:accesslog:list"

// ErrAccessLogUnreadable is returned by ListAccessLogs when no configured
// sink implements accesslog.Reader.
var ErrAccessLogUnreadable = errors.New("请求日志未配置可查询的存储：ADMIN_ACCESS_LOG_SINKS 需包含 db（或应用注册的、实现 accesslog.Reader 的 sink）")

// AccessLogReadable reports whether the request log page can query entries.
func AccessLogReadable(access *accesslog.Logger) bool { return access.Reader() != nil }

func (s *LogService) ListAccessLogs(
	ctx context.Context, req *adminv1.ListAccessLogsRequest,
) (*adminv1.ListAccessLogsResponse, error) {
	reader := s.access.Reader()
	if reader == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, ErrAccessLogUnreadable.Error()).WithCause(ErrAccessLogUnreadable)
	}
	pageNum, pageSize := 1, 10
	if req.Page != nil {
		pageNum, pageSize = int(req.Page.PageNum), int(req.Page.PageSize)
	}
	pageNum, pageSize, err := normalizePageStrict(pageNum, pageSize)
	if err != nil {
		return nil, err
	}
	q := accesslog.Query{
		Principal:     req.Principal,
		PrincipalType: req.PrincipalType,
		Procedure:     req.Procedure,
		Code:          strings.TrimSpace(req.Code),
		RequestID:     req.RequestId,
		Offset:        (pageNum - 1) * pageSize,
		Limit:         pageSize,
	}
	switch accesslog.Result(req.Result) {
	case accesslog.ResultAny, accesslog.ResultSuccess, accesslog.ResultFailure:
		q.Result = accesslog.Result(req.Result)
	default:
		return nil, connect.Errorf(connect.CodeInvalidArgument, "result 只能为空、success 或 failure")
	}
	if req.StartTime != nil {
		q.From = req.StartTime.AsTime()
	}
	if req.EndTime != nil {
		q.To = req.EndTime.AsTime()
	}
	if !q.From.IsZero() && !q.To.IsZero() && !q.From.Before(q.To) {
		return nil, connect.Errorf(connect.CodeInvalidArgument, "start_time 必须早于 end_time")
	}
	page, err := reader.Query(ctx, q)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	items := make([]*adminv1.AccessLog, 0, len(page.Entries))
	for _, e := range page.Entries {
		items = append(items, toProtoAccessLog(e))
	}
	return &adminv1.ListAccessLogsResponse{
		Items: items,
		Page:  &adminv1.PageInfo{Total: page.Total, PageNum: int32(pageNum), PageSize: int32(pageSize)},
	}, nil
}

func (s *LogService) GetAccessLogStatus(
	context.Context, *adminv1.GetAccessLogStatusRequest,
) (*adminv1.GetAccessLogStatusResponse, error) {
	stats := s.access.Stats()
	resp := &adminv1.GetAccessLogStatusResponse{
		Enabled:     s.access != nil,
		Readable:    s.access.Reader() != nil,
		Queued:      int64(stats.Queued),
		QueueSize:   int64(stats.QueueSize),
		Enqueued:    clampInt64(stats.Enqueued),
		Dropped:     clampInt64(stats.Dropped),
		WriteErrors: clampInt64(stats.WriteErrors),
	}
	for _, sink := range stats.Sinks {
		resp.Sinks = append(resp.Sinks, &adminv1.AccessLogSinkStatus{
			Name:      sink.Name,
			Reader:    sink.Reader,
			Written:   clampInt64(sink.Written),
			Failed:    clampInt64(sink.Failed),
			Dropped:   clampInt64(sink.Dropped),
			Errors:    clampInt64(sink.Errors),
			LastError: sink.LastError,
		})
	}
	return resp, nil
}

func toProtoAccessLog(e accesslog.Entry) *adminv1.AccessLog {
	out := &adminv1.AccessLog{
		Id:            e.ID,
		Time:          timestamppb.New(e.Time),
		RequestId:     e.RequestID,
		Kind:          e.Kind,
		Service:       e.Service,
		Procedure:     e.Procedure,
		Method:        e.Method,
		Path:          e.Path,
		PrincipalType: e.Principal.Type,
		PrincipalId:   e.Principal.ID,
		PrincipalName: e.Principal.Name,
		ClientIp:      e.ClientIP,
		UserAgent:     e.UserAgent,
		Code:          e.Code,
		Status:        int32(e.Status),
		Success:       !e.Failed(),
		Error:         e.Error,
		DurationUs:    e.Duration.Microseconds(),
	}
	if len(e.Attrs) > 0 {
		if data, err := json.Marshal(e.Attrs); err == nil {
			out.AttrsJson = string(data)
		}
	}
	return out
}

func clampInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}
