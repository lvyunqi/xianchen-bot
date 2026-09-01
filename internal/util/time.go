package util

import (
	"fmt"
	"time"
)

func Today() string { return time.Now().Format("2006-01-02") }
func FormatDuration(value time.Duration) string {
	if value < 0 {
		value = 0
	}
	hours := int(value.Hours())
	minutes := int(value.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes)
	}
	return fmt.Sprintf("%d分钟", int(value.Round(time.Minute).Minutes()))
}
