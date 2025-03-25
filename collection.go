package niu

type Collection[T any] interface {
	Size() int
	IsEmpty() bool
	Clear()
}
