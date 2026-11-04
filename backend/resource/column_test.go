package resource

import "testing"

func TestColumnSetCopiesConstructorInput(t *testing.T) {
	input := []Column{"name", "remark"}
	set := NewColumnSet(input...)

	input[0] = "changed"
	if !set.Contains("name") || set.Contains("changed") {
		t.Fatalf("ColumnSet must not share constructor input: %v", set.Values())
	}
}

func TestColumnSetValuesReturnsCopy(t *testing.T) {
	set := NewColumnSet("name", "remark")

	first := set.Values()
	first[0] = "changed"
	second := set.Values()

	if second[0] != "name" {
		t.Fatalf("Values must return a copy, got %v", second)
	}
	if set.Len() != 2 || !set.Contains("remark") {
		t.Fatalf("unexpected ColumnSet state: len=%d values=%v", set.Len(), set.Values())
	}
}

func TestZeroColumnSetIsEmpty(t *testing.T) {
	var set ColumnSet
	if set.Len() != 0 || set.Contains("name") || len(set.Values()) != 0 {
		t.Fatalf("zero ColumnSet must be empty: len=%d values=%v", set.Len(), set.Values())
	}
}
