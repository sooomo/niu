package collection

import "sync"

type empty struct{}

var voidPlaceholder empty

type Set[T comparable] struct {
	underlying map[T]empty
	lock       sync.RWMutex
}

func (s *Set[T]) ensureInit() {
	if s.underlying == nil {
		s.underlying = map[T]empty{}
	}
}

// O(1)~O(n)
func (s *Set[T]) Add(item T) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.ensureInit()
	s.underlying[item] = voidPlaceholder
}

// O(n)
func (s *Set[T]) AddRange(items ...T) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.ensureInit()
	for _, v := range items {
		s.underlying[v] = voidPlaceholder
	}
}

// O(1)~O(n)
func (s *Set[T]) Remove(item T) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.underlying, item)
}

// O(1)
func (s *Set[T]) Clear() {
	s.lock.Lock()
	defer s.lock.Unlock()

	clear(s.underlying)
}

// O(n)
func (s *Set[T]) Size() int {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return len(s.underlying)
}

// O(n)
func (s *Set[T]) IsEmpty() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return len(s.underlying) == 0
}

// O(n)
func (s *Set[T]) ToSlice() []T {
	s.lock.RLock()
	defer s.lock.RUnlock()

	out := []T{}
	for k := range s.underlying {
		out = append(out, k)
	}
	return out
}

// O(1)~O(n)
func (s *Set[T]) Contains(item T) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	_, ok := s.underlying[item]
	return ok
}
