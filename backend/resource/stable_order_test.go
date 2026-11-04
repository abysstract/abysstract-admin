package resource

import (
	"reflect"
	"testing"
)

func TestStableOrdersAppendsPrimaryKeyOnlyWhenAbsent(t *testing.T) {
	key := Column("id")
	tests := []struct {
		name string
		in   []Order
		want []Order
	}{
		{"default key ascending", nil, []Order{{Column: key}}},
		{"multiple requested fields preserve order then key", []Order{{Column: "name", Desc: true}, {Column: "created_at"}}, []Order{{Column: "name", Desc: true}, {Column: "created_at"}, {Column: key}}},
		{"requested key is not duplicated", []Order{{Column: key, Desc: true}}, []Order{{Column: key, Desc: true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := append([]Order(nil), tt.in...)
			if got := StableOrders(input, key); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("StableOrders(%v, %q) = %v, want %v", tt.in, key, got, tt.want)
			}
			if !reflect.DeepEqual(input, tt.in) {
				t.Fatalf("StableOrders mutated input: before=%v after=%v", tt.in, input)
			}
		})
	}
}
