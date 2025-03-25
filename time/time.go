package time

import (
	"strconv"
	"time"

	"github.com/sooomo/niu/strs"
)

const (
	Day = 24 * time.Hour
)

// 计算指定一段时间的天数
func Days(dur time.Duration) int64 {
	return int64(dur.Hours() / 24)
}

// 上个月此时
func LastMonthNow(t time.Time) time.Time {
	return t.AddDate(0, -1, 0)
}

// 上个月第一天
func LastMonthFirstDay() time.Time {
	now := LastMonthNow(time.Now())
	return BeginOfMonth(now)
}

// 上个月最后一天
func LastMonthLastDay() time.Time {
	now := LastMonthNow(time.Now())
	return EndOfMonth(now)
}

// 这个月第一天
func ThisMonthFirstDay() time.Time {
	now := time.Now()
	return BeginOfMonth(now)
}

// 计算一个月的天数
func DaysOfMonth(t time.Time) int64 {
	nextMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())   // 下个月的 1 号
	curMonthStart := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()) // 这个月1号
	return int64(nextMonth.Sub(curMonthStart).Hours() / 24)
}

// 这个月的天数
func DaysOfThisMonth() int64 {
	return DaysOfMonth(time.Now())
}

// 这个月升剩余的天数
func ThisMonthRemainDays() int64 {
	now := time.Now()
	dayCount := DaysOfMonth(now) // 计算这个月的天数
	return dayCount - int64(now.Day())
}

// 将时间格式化为 YYYYMMDD 格式的字符串
func TimeToYYYYMMDD(t time.Time) string {
	year := strconv.Itoa(t.Year())
	monthStr := strs.PadLeftInt(int(t.Month()), 2, "0")
	dayStr := strs.PadLeftInt(t.Day(), 2, "0")
	return year + monthStr + dayStr
}

// 将时间格式化为 YYYYMMDD 格式的整数
func TimeToYYYYMMDDInt(t time.Time) (int, error) {
	str := TimeToYYYYMMDD(t)
	return strconv.Atoi(str)
}

// 返回指定天的开始时间
func BeginOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// 返回指定天的结束时间
func EndOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 23, 59, 59, int(time.Second-time.Nanosecond), t.Location())
}

// 一个月的第一天
func BeginOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

// 一个月的最后一天
func EndOfMonth(t time.Time) time.Time {
	// 指定时间的下一月的第一天
	nextStart := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
	return time.Date(nextStart.Year(), nextStart.Month(), nextStart.Day()-1, 23, 59, 59, 0, t.Location())
}

// 一年的开始时间
func BeginOfYear(t time.Time) time.Time {
	return time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, t.Location())
}

// 一年的结束时间
func EndOfYear(t time.Time) time.Time {
	return time.Date(t.Year(), time.December, 31, 23, 59, 59, int(time.Second-time.Nanosecond), t.Location())
}
