package service

import (
	"context"
	"time"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

// DashboardService 实现 adminv1connect.DashboardServiceHandler（需求 9.14）。
type DashboardService struct {
	repo *repo.Repo
}

func NewDashboardService(r *repo.Repo) *DashboardService {
	return &DashboardService{repo: r}
}

func (s *DashboardService) Overview(
	ctx context.Context, _ *adminv1.OverviewRequest,
) (*adminv1.OverviewResponse, error) {
	stats, err := s.repo.DashboardStats(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	recent := make([]*adminv1.RecentLogin, 0, len(stats.RecentLogins))
	for _, r := range stats.RecentLogins {
		recent = append(recent, &adminv1.RecentLogin{
			Username:  r.Username,
			Ip:        r.IP,
			LoginTime: r.LoginTime.Format(time.RFC3339),
			Status:    r.Status,
		})
	}
	return &adminv1.OverviewResponse{
		UserCount:       stats.UserCount,
		RoleCount:       stats.RoleCount,
		TodayLoginCount: stats.TodayLoginCount,
		OnlineCount:     0, // 本期未维护在线会话表，返回 0 占位
		RecentLogins:    recent,
	}, nil
}
