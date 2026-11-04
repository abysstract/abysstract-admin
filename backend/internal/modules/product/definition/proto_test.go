package productdef

import (
	"testing"

	productv1 "github.com/abysstract/abysstract-admin/backend/internal/modules/product/gen/proto/company/product/v1"
)

func TestCreateRequestHasNoCreateBy(t *testing.T) {
	desc := (&productv1.CreateProductRequest{}).ProtoReflect().Descriptor()
	if field := desc.Fields().ByName("create_by"); field != nil {
		t.Fatal("create_by must not be a CreateProductRequest field")
	}
	if field := desc.Fields().ByName("id"); field != nil {
		t.Fatal("id must not be a CreateProductRequest field")
	}
}
