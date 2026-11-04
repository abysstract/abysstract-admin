package coredefs

import (
	"reflect"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestResourcesOwnSliceAndPreserveRelationIdentity(t *testing.T) {
	want := []*schema.Resource{DictTypeResource, DictDataResource, ConfigResource, NoticeResource, DeptResource, MenuResource, RoleResource, UserResource}
	first := Resources()
	if !reflect.DeepEqual(first, want) {
		t.Fatal("system resource order changed")
	}
	first[0] = nil
	next := Resources()
	if !reflect.DeepEqual(next, want) {
		t.Fatal("caller mutated the system resource list")
	}
	seen := map[*schema.Resource]bool{}
	for _, r := range next {
		if seen[r] || r.Name == "Widget" {
			t.Fatalf("unexpected resource %s", r.Name)
		}
		seen[r] = true
	}
	for _, r := range next {
		for _, f := range r.Fields {
			if f.Relation != nil && !seen[f.Relation.Target()] {
				t.Fatalf("%s relation points outside the explicit list", r.Name)
			}
		}
	}
	if _, err := schema.BuildRegistry(next); err != nil {
		t.Fatal(err)
	}
}
