//go:build integration

package repo

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/database"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
)

func TestConcurrentLastRoleAndDeptMoves(t *testing.T) {
	t.Run("last_role", func(t *testing.T) {
		r := newRepositoryFixture(t)
		execFixtureSQL(t, r.db, "INSERT INTO sys_user_role(user_id,role_id) VALUES(8101,8102)")
		ctx, cancel := context.WithTimeout(interceptor.WithClaims(context.Background(), &jwt.Claims{UserID: 8101, Username: "p1-one"}), 10*time.Second)
		defer cancel()
		ready := make(chan struct{}, 2)
		release := make(chan struct{})
		results := make(chan error, 2)
		for _, id := range []int64{8101, 8102} {
			go func(id int64) {
				results <- r.InTransaction(ctx, func(tx *Repo) error {
					if _, err := tx.GetRoleResourceForUpdate(ctx, id, UnrestrictedAccessScope()); err != nil {
						return err
					}
					ready <- struct{}{}
					<-release
					return tx.ChangeRoleStatusResource(ctx, id, 2, UnrestrictedAccessScope())
				})
			}(id)
		}
		readyCount := 2
		if database.DriverOf(r.db) == "sqlite" {
			readyCount = 1
		}
		for i := 0; i < readyCount; i++ {
			select {
			case <-ready:
			case <-ctx.Done():
				close(release)
				t.Fatal(ctx.Err())
			}
		}
		close(release)
		success := 0
		for i := 0; i < 2; i++ {
			err := <-results
			if err == nil {
				success++
			} else {
				t.Logf("protected concurrent role change: %v", err)
			}
		}
		if success != 1 {
			t.Fatalf("successful changes=%d", success)
		}
		var active int64
		if err := r.db.Model(&model.SysRole{}).Where("id IN ? AND status=1 AND del_flag=0", []int64{8101, 8102}).Count(&active).Error; err != nil {
			t.Fatal(err)
		}
		if active != 1 {
			t.Fatalf("remaining active roles=%d", active)
		}
	})
	t.Run("opposing_subtrees", func(t *testing.T) {
		r := newRepositoryFixture(t)
		execFixtureSQL(t, r.db, `INSERT INTO sys_dept(id,parent_id,ancestors,dept_name) VALUES(8101,1,'0,1','a'),(8102,1,'0,1','b'),(8103,8101,'0,1,8101','a-child'),(8104,8102,'0,1,8102','b-child')`)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ready := make(chan struct{}, 2)
		release := make(chan struct{})
		results := make(chan error, 2)
		for _, pair := range [][2]int64{{8101, 8102}, {8102, 8101}} {
			go func(pair [2]int64) {
				results <- r.InTransaction(ctx, func(tx *Repo) error {
					if _, err := tx.GetDeptResourceForUpdate(ctx, pair[0], UnrestrictedAccessScope()); err != nil {
						return err
					}
					ready <- struct{}{}
					<-release
					if err := tx.MoveDeptSubtree(ctx, pair[0], pair[1]); err != nil {
						return err
					}
					return tx.db.Model(&model.SysDept{}).Where("id=?", pair[0]).Update("parent_id", pair[1]).Error
				})
			}(pair)
		}
		readyCount := 2
		if database.DriverOf(r.db) == "sqlite" {
			readyCount = 1
		}
		for i := 0; i < readyCount; i++ {
			select {
			case <-ready:
			case <-ctx.Done():
				close(release)
				t.Fatal(ctx.Err())
			}
		}
		close(release)
		success := 0
		for i := 0; i < 2; i++ {
			err := <-results
			if err == nil {
				success++
			} else {
				t.Logf("rejected opposing move: %v", err)
			}
		}
		if success != 1 {
			t.Fatalf("successful moves=%d", success)
		}
		var rows []model.SysDept
		if err := r.db.Where("del_flag=0").Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		byID := map[int64]model.SysDept{}
		for _, row := range rows {
			byID[row.ID] = row
		}
		for _, row := range rows {
			if row.ParentID != 0 {
				parent := byID[row.ParentID]
				want := fmt.Sprintf("%s,%d", parent.Ancestors, parent.ID)
				if row.Ancestors != want || containsID(row.Ancestors, row.ID) {
					t.Fatalf("invalid tree: %+v parent=%+v", row, parent)
				}
			}
		}
		rollback := errors.New("move rollback")
		var before model.SysDept
		if err := r.db.First(&before, 8103).Error; err != nil {
			t.Fatal(err)
		}
		err := r.InTransaction(ctx, func(tx *Repo) error {
			if err := tx.MoveDeptSubtree(ctx, 8103, 1); err != nil {
				return err
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatal(err)
		}
		var after model.SysDept
		if err := r.db.First(&after, 8103).Error; err != nil {
			t.Fatal(err)
		}
		if before.Ancestors != after.Ancestors {
			t.Fatal("rollback changed descendant path")
		}
	})
}
