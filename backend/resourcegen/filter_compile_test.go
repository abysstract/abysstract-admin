package resourcegen

import (
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestResourceFilterPlanRuntimeConversionBeforeRepository(t *testing.T) {
	plan, err := schematest.BoolCompileSampleResource().Build()
	if err != nil {
		t.Fatal(err)
	}
	compileServicePlansWithRuntime(t, []*schema.Plan{plan}, resourceTestFilterRuntimeTest)
}

const resourceTestFilterRuntimeTest = `package gen

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

type captureBoolFilterRepository struct {
	resource.Repository[schematest.BoolModel, BoolListQuery]
	calls int
	query BoolListQuery
}

func (r *captureBoolFilterRepository) List(_ context.Context, _ resource.Session, q BoolListQuery, _ repo.AccessScope) ([]*schematest.BoolModel, int64, error) {
	r.calls++
	r.query = q
	return nil, 0, nil
}

func TestFilterPlanConvertsBeforeRepository(t *testing.T) {
	captured := &captureBoolFilterRepository{}
	controller := &BoolController{repo: captured}
	narrow := int32(7)
	status := testpb.BatchStatus_BATCH_STATUS_ACTIVE
	if _, err := controller.ListBools(context.Background(), &testpb.ListBoolsRequest{NarrowValue: &narrow, Status: &status}); err != nil {
		t.Fatal(err)
	}
	if captured.calls != 1 || captured.query.NarrowValue == nil || *captured.query.NarrowValue != int16(7) || captured.query.Status == nil || *captured.query.Status != int16(1) {
		t.Fatalf("converted query = %+v calls=%d", captured.query, captured.calls)
	}

	captured.calls = 0
	unspecified := testpb.BatchStatus_BATCH_STATUS_UNSPECIFIED
	if _, err := controller.ListBools(context.Background(), &testpb.ListBoolsRequest{Status: &unspecified}); err != nil || captured.calls != 1 || captured.query.Status == nil || *captured.query.Status != 0 {
		t.Fatalf("explicit zero status err=%v calls=%d query=%+v, want status=0 filter", err, captured.calls, captured.query)
	}

	captured.calls = 0
	overflow := int32(32768)
	if _, err := controller.ListBools(context.Background(), &testpb.ListBoolsRequest{NarrowValue: &overflow}); connect.CodeOf(err) != connect.CodeInvalidArgument || captured.calls != 0 {
		t.Fatalf("overflow err=%v calls=%d", err, captured.calls)
	}

	captured.calls = 0
	unknown := testpb.BatchStatus(99)
	if _, err := controller.ListBools(context.Background(), &testpb.ListBoolsRequest{Status: &unknown}); connect.CodeOf(err) != connect.CodeInvalidArgument || captured.calls != 0 {
		t.Fatalf("unknown enum err=%v calls=%d", err, captured.calls)
	}
}
`
