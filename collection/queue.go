package collection

import (
	"fmt"
	"sync"
)

type fastQueueNode[T any] struct {
	value *T
	next  *fastQueueNode[T]
}

type FastQueue[T any] struct {
	first *fastQueueNode[T]
	last  *fastQueueNode[T]
	size  int
	lock  sync.RWMutex
}

// O(1)
func (queue *FastQueue[T]) In(val *T) {
	queue.lock.Lock()
	defer queue.lock.Unlock()

	node := &fastQueueNode[T]{value: val, next: nil}
	if queue.last != nil {
		queue.last.next = node
	}
	queue.last = node

	if queue.first == nil {
		queue.first = node
	}
	queue.size++
}

// O(n)
func (queue *FastQueue[T]) InAll(vals ...T) {
	l := len(vals)
	if l == 0 {
		return
	}
	queue.lock.Lock()
	defer queue.lock.Unlock()

	for _, v := range vals {
		node := &fastQueueNode[T]{value: &v, next: nil}
		if queue.last != nil {
			queue.last.next = node
		}
		queue.last = node

		if queue.first == nil {
			queue.first = node
		}
	}

	queue.size += l
}

// O(1)
func (queue *FastQueue[T]) Out() *T {
	queue.lock.Lock()
	defer queue.lock.Unlock()

	if queue.first == nil {
		return nil
	}

	out := queue.first
	queue.first = out.next
	queue.size--

	return out.value
}

// O(1)
func (queue *FastQueue[T]) Clear() {
	queue.lock.Lock()
	defer queue.lock.Unlock()
	queue.first = nil
	queue.last = nil
	queue.size = 0
}

// O(1)
func (queue *FastQueue[T]) Size() int {
	queue.lock.RLock()
	defer queue.lock.RUnlock()
	return queue.size
}

// O(1)
func (queue *FastQueue[T]) IsEmpty() bool {
	queue.lock.RLock()
	defer queue.lock.RUnlock()
	return queue.size == 0
}

// for test only
func (queue *FastQueue[T]) Print() {
	fmt.Println("--------start print queue[first->last]----------")
	node := queue.first
	idx := 0
	for {
		if node == nil {
			break
		}
		fmt.Printf("item %d, value=%v", idx, *node.value)
		node = node.next
		idx++
	}
	fmt.Println("--------end print queue----------")
}
