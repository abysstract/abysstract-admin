//go:build integration

package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"gorm.io/gorm"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

func TestUsernameIdentityAndCounter(t *testing.T) {
	base, done := openUserSecurityIntegration(t)
	defer done()
	handler := newUserSecurityHandler(t, base)
	ctx := userClaimsContext(1, "admin")
	const password = " GoodPass1 "
	var previous int64
	for i, name := range []string{" Alice ", "alice", "ALICE"} {
		created, err := handler.CreateUser(ctx, &adminv1.CreateUserRequest{Username: name, Password: password, DeptId: 1})
		if err != nil {
			t.Fatal(err)
		}
		u := created.User
		if u.Username != "alice" || u.Id <= previous {
			t.Fatalf("canonical identity/sequence: %+v", u)
		}
		previous = u.Id
		_, err = handler.CreateUser(ctx, &adminv1.CreateUserRequest{Username: "ALICE", Password: password, DeptId: 1})
		if connect.CodeOf(err) != connect.CodeAlreadyExists {
			t.Fatalf("duplicate: %v", err)
		}
		verified, err := base.VerifyPassword(ctx, " ALIce ", password)
		if err != nil || verified.ID != u.Id {
			t.Fatalf("lookup: %v", err)
		}
		if _, err := base.VerifyPassword(ctx, name, strings.TrimSpace(password)); err == nil {
			t.Fatal("password whitespace lost")
		}
		if i < 2 {
			if _, err := handler.DeleteUser(ctx, &adminv1.DeleteUserRequest{Id: u.Id}); err != nil {
				t.Fatal(err)
			}
		}
	}
	security := repo.NewLoginSecurity(base.DB(), 3, 900)
	now := time.Now().UTC()
	for i, name := range []string{" Unknown ", "UNKNOWN", "unknown"} {
		locked, count, err := security.RecordFailure(ctx, name, now)
		if err != nil || count != int32(i+1) || locked != (i == 2) {
			t.Fatalf("counter %d %v %v", count, locked, err)
		}
	}
	if _, err := handler.CreateUser(ctx, &adminv1.CreateUserRequest{Username: " UNKNOWN ", Password: password, DeptId: 1}); err != nil {
		t.Fatal(err)
	}
	if locked, _, err := security.IsLocked(ctx, "Unknown", now); err != nil || !locked {
		t.Fatalf("unknown to known: %v %v", locked, err)
	}
	if locked, _, err := security.IsLocked(ctx, "UNKNOWN", now.Add(901*time.Second)); err != nil || locked {
		t.Fatalf("expiration: %v %v", locked, err)
	}
	// A successful real Login clears the same canonical failure key.
	if _, _, err := security.RecordFailure(ctx, "ALICE", now); err != nil {
		t.Fatal(err)
	}
	auth := NewAuthService(base, security, repo.NewRefreshTokenRepo(base.DB()), jwt.NewManager("p2-test", time.Hour), time.Hour, false, 7*24*time.Hour, false)
	response, err := auth.Login(context.Background(), &adminv1.LoginRequest{Username: " ALIce ", Password: password})
	if err != nil || response.Token == "" {
		t.Fatalf("Login: %v", err)
	}
	var count int64
	if err := base.DB().Model(&model.SysLoginAttempt{}).Where("username = ?", "alice").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("reset: %d %v", count, err)
	}
	concurrent := repo.NewLoginSecurity(base.DB(), 20, 900)
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "PARALLEL"
			if i%2 == 0 {
				name = " parallel "
			}
			_, _, err := concurrent.RecordFailure(ctx, name, now)
			failures <- err
		}(i)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var attempt model.SysLoginAttempt
	if err := base.DB().Where("username = ?", "parallel").Take(&attempt).Error; err != nil || attempt.FailCount != 8 {
		t.Fatalf("parallel: %+v %v", attempt, err)
	}
	sentinel := errors.New("caller rollback")
	err = base.DB().Transaction(func(tx *gorm.DB) error {
		if _, _, err := repo.NewLoginSecurity(tx, 3, 900).RecordFailure(ctx, "rollback", now); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if err := base.DB().Model(&model.SysLoginAttempt{}).Where("username = ?", "rollback").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("transaction leaked: %v %d", err, count)
	}
}

func TestConcurrentUsernameConstraint(t *testing.T) {
	base, done := openUserSecurityIntegration(t)
	defer done()
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, name := range []string{" Race ", "RACE"} {
		go func(name string) {
			<-start
			u := &model.SysUser{Username: name}
			results <- base.CreateUser(context.Background(), u, " GoodPass1 ", nil)
		}(name)
	}
	close(start)
	a, b := <-results, <-results
	if (a == nil) == (b == nil) {
		t.Fatalf("one success required: %v %v", a, b)
	}
	if a == nil {
		a = b
	}
	if !errors.Is(a, repo.ErrDuplicate) {
		t.Fatalf("unique constraint: %v", a)
	}
}
