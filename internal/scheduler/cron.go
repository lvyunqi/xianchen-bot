package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronSpec 表示六段 cron 表达式：秒 分 时 日 月 周。
// 支持 * 、*/n 、a 、a-b 、a-b/n 、a,b,c 的字段组合，覆盖 config 默认值与常见运维配置。
type CronSpec struct {
	second fieldSet
	minute fieldSet
	hour   fieldSet
	dom    fieldSet // day of month
	month  fieldSet
	dow    fieldSet // day of week, 0=周日
	domAny bool
	dowAny bool
}

type fieldSet map[int]bool

// ParseCron 解析六段 cron 表达式，任何解析失败都返回错误而不是静默降级。
func ParseCron(expr string) (CronSpec, error) {
	parts := strings.Fields(strings.TrimSpace(expr))
	if len(parts) != 6 {
		return CronSpec{}, fmt.Errorf("cron 表达式必须为六段（秒 分 时 日 月 周）：%q", expr)
	}
	var spec CronSpec
	var err error
	if spec.second, err = parseField(parts[0], 0, 59); err != nil {
		return CronSpec{}, fmt.Errorf("cron 秒段无效: %w", err)
	}
	if spec.minute, err = parseField(parts[1], 0, 59); err != nil {
		return CronSpec{}, fmt.Errorf("cron 分段无效: %w", err)
	}
	if spec.hour, err = parseField(parts[2], 0, 23); err != nil {
		return CronSpec{}, fmt.Errorf("cron 时段无效: %w", err)
	}
	spec.domAny = parts[3] == "*"
	if spec.dom, err = parseField(parts[3], 1, 31); err != nil {
		return CronSpec{}, fmt.Errorf("cron 日段无效: %w", err)
	}
	if spec.month, err = parseField(parts[4], 1, 12); err != nil {
		return CronSpec{}, fmt.Errorf("cron 月段无效: %w", err)
	}
	spec.dowAny = parts[5] == "*"
	if spec.dow, err = parseField(parts[5], 0, 6); err != nil {
		return CronSpec{}, fmt.Errorf("cron 周段无效: %w", err)
	}
	return spec, nil
}

// Matches 判断给定时刻（本地时区）是否命中表达式。日/周同时受限时按标准 cron 语义取并集。
func (c CronSpec) Matches(t time.Time) bool {
	if !c.second[t.Second()] || !c.minute[t.Minute()] || !c.hour[t.Hour()] || !c.month[int(t.Month())] {
		return false
	}
	domHit := c.dom[t.Day()]
	dowHit := c.dow[int(t.Weekday())]
	switch {
	case c.domAny && c.dowAny:
		return true
	case c.domAny:
		return dowHit
	case c.dowAny:
		return domHit
	default:
		return domHit || dowHit
	}
}

func parseField(field string, min, max int) (fieldSet, error) {
	set := fieldSet{}
	for _, token := range strings.Split(field, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			return nil, fmt.Errorf("空字段片段")
		}
		step := 1
		if idx := strings.Index(token, "/"); idx >= 0 {
			parsed, err := strconv.Atoi(token[idx+1:])
			if err != nil || parsed <= 0 {
				return nil, fmt.Errorf("步长无效: %q", token)
			}
			step = parsed
			token = token[:idx]
		}
		lo, hi := min, max
		switch {
		case token == "*":
		case strings.Contains(token, "-"):
			bounds := strings.SplitN(token, "-", 2)
			a, err1 := strconv.Atoi(bounds[0])
			b, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil || a > b {
				return nil, fmt.Errorf("区间无效: %q", token)
			}
			lo, hi = a, b
		default:
			v, err := strconv.Atoi(token)
			if err != nil {
				return nil, fmt.Errorf("数值无效: %q", token)
			}
			lo, hi = v, v
		}
		if lo < min || hi > max || lo > hi {
			return nil, fmt.Errorf("取值越界: %q（范围 %d-%d）", field, min, max)
		}
		for v := lo; v <= hi; v += step {
			set[v] = true
		}
	}
	return set, nil
}
