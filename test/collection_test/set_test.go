package collection_test

import (
	"slices"
	"testing"

	"github.com/sooomo/niu/collection"
)

var set = &collection.Set[string]{}

func TestSet_Add(t *testing.T) {
	tests := []struct {
		val  string
		size int
	}{
		{val: "one", size: 1},
		{val: "two", size: 2},
		{val: "three", size: 3},
		{val: "four", size: 4},
		{val: "one", size: 4},
	}
	for _, tt := range tests {
		t.Run(tt.val, func(t *testing.T) {
			set.Add(tt.val)
			sz := set.Size()
			if sz != tt.size {
				t.Errorf("Add():val=%v, size=%v, want=%v", tt.val, sz, tt.size)
			}
		})
	}
}

func TestSet_AddRange(t *testing.T) {
	tests := []struct {
		name string
		vals []string
		size int
	}{
		{name: "range_1", vals: []string{"one", "five", "six", "seven"}, size: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set.AddRange(tt.vals...)
			sz := set.Size()
			if sz != tt.size {
				t.Errorf("AddRange():vals=%v, size=%v, want=%v", tt.vals, sz, tt.size)
			}
		})
	}
}

func TestSet_Remove(t *testing.T) {
	tests := []struct {
		val  string
		size int
	}{
		{val: "two", size: 6},
		{val: "three", size: 5},
		{val: "four", size: 4},
		{val: "four", size: 4},
		{val: "seven", size: 3},
	}
	for _, tt := range tests {
		t.Run(tt.val, func(t *testing.T) {
			set.Remove(tt.val)
			sz := set.Size()
			if sz != tt.size {
				t.Errorf("Remove():val=%v, size=%v, want=%v", tt.val, sz, tt.size)
			}
		})
	}
}

func TestSet_ToSlice(t *testing.T) {
	t.Run("to_slice", func(t *testing.T) {
		arr := set.ToSlice()
		sz := len(arr)
		if sz != 3 {
			t.Errorf("ToSlice():val=%v, size=%v, want=%v", arr, sz, 3)
		}

		if !slices.Contains(arr, "one") {
			t.Error("ToSlice() should contains 'one'")
		}
		if !slices.Contains(arr, "five") {
			t.Error("ToSlice() should contains 'five'")
		}
		if !slices.Contains(arr, "six") {
			t.Error("ToSlice() should contains 'six'")
		}
	})
}

func TestSet_Contains(t *testing.T) {
	tests := []struct {
		val  string
		want bool
	}{
		{val: "one", want: true},
		{val: "four", want: false},
		{val: "future", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.val, func(t *testing.T) {
			ret := set.Contains(tt.val)
			if ret != tt.want {
				t.Errorf("Contains(): val= %v, got %v, want %v", tt.val, ret, tt.want)
			}
		})
	}
}

func TestSet_Size(t *testing.T) {
	//
}

func TestSet_Clear(t *testing.T) {
	t.Run("clear", func(t *testing.T) {
		set.Clear()
		sz := set.Size()
		if sz != 0 {
			t.Errorf("Clear(): size=%v, want=%v", sz, 0)
		}
	})
}

func TestSet_IsEmpty(t *testing.T) {
	t.Run("is_empty", func(t *testing.T) {
		val := set.IsEmpty()
		if !val {
			t.Errorf("IsEmpty(): val=%v, want=%t", val, true)
		}
	})
}
