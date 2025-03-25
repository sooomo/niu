package id

import (
	"strings"

	"github.com/google/uuid"
)

// 生成UUID字符串
func NewUUID() string {
	return uuid.New().String()
}

// 生成没有短横线的UUID字符串
func NewUUIDWithoutDash() string {
	uid := uuid.New().String()
	idStr := strings.Replace(uid, "-", "", -1)
	return idStr
}
