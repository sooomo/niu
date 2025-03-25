package collection

import (
	"fmt"
	"sync"
)

type fastStackNode[T any] struct {
	value *T
	next  *fastStackNode[T]
}

type FastStack[T any] struct {
	top  *fastStackNode[T]
	size int
	lock sync.RWMutex
}

// O(1)
func (stack *FastStack[T]) Push(val *T) {
	stack.lock.Lock()
	defer stack.lock.Unlock()
	stack.top = &fastStackNode[T]{value: val, next: stack.top}
	stack.size++
}

// O(n)
func (stack *FastStack[T]) PushAll(vals ...T) {
	l := len(vals)
	if l == 0 {
		return
	}
	stack.lock.Lock()
	defer stack.lock.Unlock()
	for _, v := range vals {
		stack.top = &fastStackNode[T]{value: &v, next: stack.top}
	}
	stack.size += l
}

// O(1)
func (stack *FastStack[T]) Pop() *T {
	stack.lock.Lock()
	defer stack.lock.Unlock()
	if stack.top == nil {
		return nil
	}
	out := stack.top
	stack.top = out.next
	stack.size--
	return out.value
}

// O(1)
func (stack *FastStack[T]) Clear() {
	stack.lock.Lock()
	defer stack.lock.Unlock()
	stack.top = nil
	stack.size = 0
}

// O(1)
func (stack *FastStack[T]) Peek() *T {
	stack.lock.RLock()
	defer stack.lock.RUnlock()
	if stack.top == nil {
		return nil
	}
	return stack.top.value
}

// O(1)
func (stack *FastStack[T]) Size() int {
	stack.lock.RLock()
	defer stack.lock.RUnlock()
	return stack.size
}

// O(1)
func (stack *FastStack[T]) IsEmpty() bool {
	stack.lock.RLock()
	defer stack.lock.RUnlock()
	return stack.size == 0
}

// for test only
func (stack *FastStack[T]) Print() {
	fmt.Println("--------start print stack[top->bottom]----------")
	node := stack.top
	idx := 0
	for {
		if node == nil {
			break
		}
		fmt.Printf("item %d, value=%v", idx, *node.value)
		node = node.next
		idx++
	}
	fmt.Println("--------end print stack----------")
}
