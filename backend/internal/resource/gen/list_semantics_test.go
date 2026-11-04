package gen

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	testutil "github.com/abysstract/abysstract-admin/backend/internal/testutil"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestResourceListRejectsNegativePageSizeBeforeSQL(t *testing.T) {
	// The request must be rejected before the repository is touched. A nil
	// BaseRepo makes any accidental SQL path fail immediately.
	controller := &DictTypeController{repo: NewDictTypeRepository()}

	_, err := controller.ListDictTypes(context.Background(), &adminv1.ListDictTypesRequest{
		Page: &adminv1.PageRequest{PageSize: -1},
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("negative page size error code = %v, want %v (err=%v)", connect.CodeOf(err), connect.CodeInvalidArgument, err)
	}
}

type captureDictDataListRepository struct {
	resource.Repository[model.SysDictDatum, DictDataListQuery]
	calls int
	query DictDataListQuery
}

var _ resource.Repository[model.SysDictDatum, DictDataListQuery] = (*captureDictDataListRepository)(nil)

func (r *captureDictDataListRepository) List(_ context.Context, _ resource.Session, query DictDataListQuery, _ repo.AccessScope) ([]*model.SysDictDatum, int64, error) {
	r.calls++
	r.query = query
	return nil, 0, nil
}

func TestResourceListNormalizesPresenceBoundsAndSortBeforeRepository(t *testing.T) {
	t.Run("explicit empty filter remains present and page bounds normalize", func(t *testing.T) {
		captured := &captureDictDataListRepository{}
		controller := &DictDataController{repo: captured}
		empty := ""

		response, err := controller.ListDictData(context.Background(), &adminv1.ListDictDataRequest{
			Page:     &adminv1.PageRequest{PageNum: 0, PageSize: 101},
			DictType: &empty,
			Orders:   []*adminv1.OrderBy{{Field: "dictSort", Desc: true}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if captured.calls != 1 {
			t.Fatalf("repository List calls = %d, want 1", captured.calls)
		}
		if captured.query.PageNum != 1 || captured.query.PageSize != repo.MaxPageSize {
			t.Fatalf("normalized page = %+v, want num=1 size=%d", captured.query, repo.MaxPageSize)
		}
		if captured.query.DictType == nil || *captured.query.DictType != "" {
			t.Fatalf("explicit empty DictType filter was lost: %#v", captured.query.DictType)
		}
		if got := captured.query.Orders; len(got) != 1 || got[0].Column != DictDataColumnDictSort || !got[0].Desc {
			t.Fatalf("resolved sort = %#v, want descending dict_sort", got)
		}
		if page := response.GetPage(); page.GetPageNum() != 1 || page.GetPageSize() != int32(repo.MaxPageSize) {
			t.Fatalf("response page = %+v, want normalized bounds", page)
		}
	})

	t.Run("omitted filter remains absent and zero page size uses default", func(t *testing.T) {
		captured := &captureDictDataListRepository{}
		controller := &DictDataController{repo: captured}

		if _, err := controller.ListDictData(context.Background(), &adminv1.ListDictDataRequest{Page: &adminv1.PageRequest{}}); err != nil {
			t.Fatal(err)
		}
		if captured.calls != 1 || captured.query.DictType != nil || captured.query.PageNum != 1 || captured.query.PageSize != repo.DefaultPageSize {
			t.Fatalf("omitted presence/default page semantics = calls=%d query=%+v", captured.calls, captured.query)
		}
	})

	t.Run("unknown sort is invalid argument with zero repository calls", func(t *testing.T) {
		captured := &captureDictDataListRepository{}
		controller := &DictDataController{repo: captured}

		_, err := controller.ListDictData(context.Background(), &adminv1.ListDictDataRequest{
			Page:   &adminv1.PageRequest{},
			Orders: []*adminv1.OrderBy{{Field: "untrusted_column"}},
		})
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("unknown sort error code = %v, want %v (err=%v)", connect.CodeOf(err), connect.CodeInvalidArgument, err)
		}
		if captured.calls != 0 {
			t.Fatalf("unknown sort reached repository %d times", captured.calls)
		}
	})
}

func TestResourceGeneratedControllerDefaultsHooksAndRepository(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: nil}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	controller, err := NewDictTypeController(DictTypeControllerDeps{Store: testutil.Store(t, repo.New(db))})
	if err != nil || controller.hooks == nil || controller.repo == nil {
		t.Fatalf("controller=%+v err=%v", controller, err)
	}
}
