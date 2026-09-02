package scheduler

import (
	"testing"
	"time"
)

func TestParseCronDefaults(t *testing.T) {
	cases := []struct {
		expr  string
		at    string
		match bool
	}{
		{"0 0 0 * * *", "2026-09-03T00:00:00Z", true},
		{"0 0 0 * * *", "2026-09-03T00:00:01Z", false},
		{"0 */10 * * * *", "2026-09-03T08:10:00Z", true},
		{"0 */10 * * * *", "2026-09-03T08:15:00Z", false},
		{"0 30 3 * * *", "2026-09-03T03:30:00Z", true},
		{"0 15 * * * *", "2026-09-03T09:15:00Z", true},
	}
	for _, tc := range cases {
		spec, err := ParseCron(tc.expr)
		if err != nil {
			t.Fatalf("ParseCron(%q): %v", tc.expr, err)
		}
		at, _ := time.Parse(time.RFC3339, tc.at)
		if got := spec.Matches(at); got != tc.match {
			t.Errorf("ParseCron(%q).Matches(%s) = %v, want %v", tc.expr, tc.at, got, tc.match)
		}
	}
}

func TestParseCronRejectsBadInput(t *testing.T) {
	for _, expr := range []string{"", "* * * * *", "61 * * * * *", "0 0 0 0 * *", "0 0 0 * * 7", "a b c d e f"} {
		if _, err := ParseCron(expr); err == nil {
			t.Errorf("ParseCron(%q) 应当报错", expr)
		}
	}
}

func TestParseCronStepAndRange(t *testing.T) {
	spec, err := ParseCron("*/15 45 1 15 3 *")
	if err != nil {
		t.Fatalf("ParseCron: %v", err)
	}
	at, _ := time.Parse(time.RFC3339, "2026-03-15T01:45:00Z")
	if !spec.Matches(at) {
		t.Errorf("应命中 2026-03-15T01:45:00Z")
	}
	at2, _ := time.Parse(time.RFC3339, "2026-03-15T01:50:00Z")
	if spec.Matches(at2) {
		t.Errorf("不应命中 01:50:00Z（分钟段固定为 45）")
	}
	rangeSpec, err := ParseCron("0 0 0 1-5 * *")
	if err != nil {
		t.Fatalf("ParseCron range: %v", err)
	}
	at3, _ := time.Parse(time.RFC3339, "2026-09-03T00:00:00Z")
	if !rangeSpec.Matches(at3) {
		t.Errorf("日段区间 1-5 应命中 3 号")
	}
	at4, _ := time.Parse(time.RFC3339, "2026-09-06T00:00:00Z")
	if rangeSpec.Matches(at4) {
		t.Errorf("日段区间 1-5 不应命中 6 号")
	}
}
