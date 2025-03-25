package strs

import (
	"strconv"
	"strings"
)

// 将输入转换为字符串，且在左边填充为指定长度
func PadLeft(val string, fixLen int, char string) string {
	valLen := len(val)
	if valLen >= fixLen {
		return val
	}

	padChars := strings.Repeat(char, fixLen-valLen)
	return padChars + val
}

// 将输入转换为字符串，且在左边填充为指定长度
func PadLeftInt(num int, fixLen int, char string) string {
	val := strconv.Itoa(num)
	valLen := len(val)
	if valLen >= fixLen {
		return val
	}

	padChars := strings.Repeat(char, fixLen-valLen)
	return padChars + val
}

// 将输入转换为字符串，且在右边填充为指定长度
func PadRight(val string, fixLen int, char string) string {
	valLen := len(val)
	if valLen >= fixLen {
		return val
	}

	padChars := strings.Repeat(char, fixLen-valLen)
	return val + padChars
}

// 将输入转换为字符串，且在右边填充为指定长度
func PadRightInt(num int, fixLen int, char string) string {
	val := strconv.Itoa(num)
	valLen := len(val)
	if valLen >= fixLen {
		return val
	}

	padChars := strings.Repeat(char, fixLen-valLen)
	return val + padChars
}

// 将手机号脱敏，即中间4为变为 ****
func MaskPhone(phone string) string {
	var builder strings.Builder
	builder.WriteString(phone[:3])
	builder.WriteString("****")
	builder.WriteString(phone[7:])
	return builder.String()
}

// 将以逗号分隔的字符串拆分成数组
func SplitCommaString(str string) []string {
	return SplitJoinedItems(str, ",")
}

func SplitJoinedItems(joinedStr string, sep ...string) []string {
	if len(joinedStr) <= 0 {
		return []string{}
	}

	sepStr := ","
	if len(sep) > 0 {
		sepStr = sep[0]
	}

	items := []string{}
	spl := strings.SplitSeq(joinedStr, sepStr)
	for v := range spl {
		val := strings.Trim(v, " ")
		if len(val) > 0 {
			items = append(items, val)
		}
	}

	return items
}
