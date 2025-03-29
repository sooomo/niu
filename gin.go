package niu

type ReplyDto[TCode any, TData any] struct {
	Code TCode  `json:"code"`
	Msg  string `json:"msg"`
	Data TData  `json:"data"`
}
