package core

type RoutinePool interface {
	Submit(task func()) error
}
