package collection_test

import (
	"testing"

	"github.com/sooomo/niu/collection"
)

var ls = &collection.FastStack[string]{}

func TestLinkStack_Push(t *testing.T) {
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
			ls.Push(&v.val)
			sz := ls.Size()
			if sz != v.size {
				t.Errorf("Size() = %v, want %v", sz, v.size)
			}
			ls.Print()
		})
	}
}

func TestLinkStack_Pop(t *testing.T) {
	tests := []struct {
		name string
		val  string
		size int
	}{
		{name: "four", size: 3},
		{name: "three", size: 2},
		{name: "two", size: 1},
	}
	for _, v := range tests {
		t.Run(v.name, func(t *testing.T) {
			val := ls.Pop()
			sz := ls.Size()
			if sz != v.size || *val != v.name {
				t.Errorf("value=%v,want %v, size= %v, want %v", *val, v.name, sz, v.size)
			}
			ls.Print()
		})
	}
}

func TestLinkStack_Peek(t *testing.T) {
	got := ls.Peek()
	if *got != "one" {
		t.Errorf("Peek() = %v, want %v", got, "one")
	}
	ls.Print()
}

func TestLinkStack_Size(t *testing.T) {
	sz := ls.Size()
	if sz != 1 {
		t.Errorf("Size() = %v, want %v", sz, 1)
	}
}

func TestLinkStack_IsEmpty(t *testing.T) {
	val := ls.IsEmpty()
	if val {
		t.Errorf("IsEmpty() = %v, want %t", val, false)
	}
}

func TestFastStack_PushAll(t *testing.T) {
	tests := []struct {
		name string
		vals []string
		size int
	}{
		{name: "push_all", vals: []string{"two", "four"}, size: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls.PushAll(tt.vals...)
			sz := ls.Size()
			if sz != tt.size {
				t.Errorf("PushAll(): size = %v, want %v", sz, tt.size)
			}
			ls.Print()
		})
	}
}
