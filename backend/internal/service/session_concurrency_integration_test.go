//go:build integration

package service_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/internal/testutil"

	"connectrpc.com/connect/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

func TestConcurrentRefreshSingleConsumption(t *testing.T) {
	svc, r, _ := newAuthSvcForTest(t, true)
	login, err := svc.Login(context.Background(), &adminv1.LoginRequest{Username: "admin", Password: "admin123"})
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	var lookups atomic.Int32
	if err := r.DB().Callback().Query().After("gorm:query").Register("test:refresh_race", func(tx *gorm.DB) {
		if tx.Statement.Table == "sys_refresh_token" && lookups.Add(1) <= 2 {
			ready <- struct{}{}
			<-release
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.DB().Callback().Query().Remove("test:refresh_race"); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := svc.RefreshToken(ctx, &adminv1.RefreshTokenRequest{RefreshToken: login.RefreshToken})
			results <- err
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-ready:
		case <-ctx.Done():
			close(release)
			t.Fatal(ctx.Err())
		}
	}
	close(release)
	success, denied := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if connect.CodeOf(err) == connect.CodeUnauthenticated {
			denied++
		} else {
			t.Errorf("rotation error: %v", err)
		}
	}
	if success != 1 || denied != 1 {
		t.Fatalf("success=%d denied=%d", success, denied)
	}
	var active int64
	if err := r.DB().Table("sys_refresh_token").Where("user_id=1 AND revoked=0").Count(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active != 1 {
		t.Fatalf("active rotated tokens=%d", active)
	}
}

func TestRefreshCreateFailureRollsBackConsumption(t *testing.T) {
	svc, r, _ := newAuthSvcForTest(t, true)
	login, err := svc.Login(context.Background(), &adminv1.LoginRequest{Username: "admin", Password: "admin123"})
	if err != nil {
		t.Fatal(err)
	}
	var token model.SysRefreshToken
	if err := r.DB().Where("user_id=1 AND revoked=0").Take(&token).Error; err != nil {
		t.Fatal(err)
	}

	testutil.AddConstraint(t, r.DB(), "sys_refresh_token", "p3_reject_refresh", "CHECK(token_hash='"+token.TokenHash+"')")
	_, err = svc.RefreshToken(context.Background(), &adminv1.RefreshTokenRequest{RefreshToken: login.RefreshToken})
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("new token failure: %v", err)
	}
	var tokens []model.SysRefreshToken
	if err := r.DB().Find(&tokens).Error; err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 1 || tokens[0].Revoked != 0 || tokens[0].TokenHash != token.TokenHash {
		t.Fatalf("partial rotation: %+v", tokens)
	}
	if err := r.DB().Migrator().DropConstraint(&model.SysRefreshToken{}, "p3_reject_refresh"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RefreshToken(context.Background(), &adminv1.RefreshTokenRequest{RefreshToken: login.RefreshToken}); err != nil {
		t.Fatalf("old token remains usable after rollback: %v", err)
	}
}

func TestPasswordChangeAndRefreshSerialize(t *testing.T) {
	svc, r, _ := newAuthSvcForTest(t, true)
	login, err := svc.Login(context.Background(), &adminv1.LoginRequest{Username: "admin", Password: "admin123"})
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("P3-new-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	password := make(chan error, 1)
	refresh := make(chan error, 1)
	go func() {
		<-start
		password <- r.ChangeOwnPasswordAndRevokeRefresh(context.Background(), 1, string(hash), repo.NewRefreshTokenRepo(r.DB()))
	}()
	go func() {
		<-start
		_, err := svc.RefreshToken(context.Background(), &adminv1.RefreshTokenRequest{RefreshToken: login.RefreshToken})
		refresh <- err
	}()
	close(start)
	if err := <-password; err != nil {
		t.Fatal(err)
	}
	if err := <-refresh; err != nil && connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal(err)
	}
	var user model.SysUser
	if err := r.DB().First(&user, 1).Error; err != nil {
		t.Fatal(err)
	}
	if user.TokenVersion != 1 || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("P3-new-password")) != nil {
		t.Fatal("password/version state")
	}
	var active int64
	if err := r.DB().Model(&model.SysRefreshToken{}).Where("user_id=1 AND revoked=0").Count(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("active tokens after password change=%d", active)
	}
}
