package scheduler

import (
	"testing"
	"time"
)

func TestNextReportTime(t *testing.T) {
	cases := []struct {
		name string
		from string
		want string
	}{
		{"понедельник до 9:00", "2026-09-21T07:30:00Z", "2026-09-21T09:00:00Z"},
		{"понедельник после 9:00", "2026-09-21T17:35:00Z", "2026-09-28T09:00:00Z"},
		{"вторник", "2026-09-22T12:00:00Z", "2026-09-28T09:00:00Z"},
		{"воскресенье", "2026-09-27T23:00:00Z", "2026-09-28T09:00:00Z"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			from, _ := time.Parse(time.RFC3339, c.from)
			want, _ := time.Parse(time.RFC3339, c.want)
			if got := nextReportTime(from); !got.Equal(want) {
				t.Errorf("nextReportTime(%s) = %s, ожидалось %s",
					c.from, got.Format(time.RFC3339), want.Format(time.RFC3339))
			}
		})
	}
}
