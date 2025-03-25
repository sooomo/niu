package collection_test

import (
	"niu/collection"
	"testing"
)

var lq = &collection.FastQueue[string]{}

func TestLinkQueue_In(t *testing.T) {
	tests := []struct {
		val  string
		size int
	}{
		{val: "one", size: 1},
		{val: "two", size: 2},
		{val: "three", size: 3},
		{val: "four", size: 4},
	}
	for _, v := range tests {
		t.Run(v.val, func(t *testing.T) {
			lq.In(&v.val)
			sz := lq.Size()
			if sz != v.size {
				t.Errorf("In(): size = %v, want %v", sz, v.size)
			}
			lq.Print()
		})
	}
}

func TestFastQueue_InAll(t *testing.T) {
	tests := []struct {
		name string
		vals []string
		size int
	}{
		{name: "in_all", vals: []string{"two", "four"}, size: 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lq.InAll(tt.vals...)
			sz := lq.Size()
			if sz != tt.size {
				t.Errorf("InAll(): size = %v, want %v", sz, tt.size)
			}
			lq.Print()
		})
	}
}

func TestLinkQueue_Out(t *testing.T) {
	tests := []struct {
		val  string
		size int
	}{
		{val: "one", size: 5},
		{val: "two", size: 4},
		{val: "three", size: 3},
	}
	for _, v := range tests {
		t.Run(v.val, func(t *testing.T) {
			val := lq.Out()
			sz := lq.Size()
			if sz != v.size || *val != v.val {
				t.Errorf("value=%v,want %v, size= %v, want %v", *val, v.val, sz, v.size)
			}
			lq.Print()
		})
	}
}

func TestLinkQueue_Size(t *testing.T) {
	sz := lq.Size()
	if sz != 3 {
		t.Errorf("Size() = %v, want %v", sz, 3)
	}
}

func TestLinkQueue_IsEmpty(t *testing.T) {
	val := lq.IsEmpty()
	if val {
		t.Errorf("IsEmpty() = %v, want %t", val, false)
	}
}
