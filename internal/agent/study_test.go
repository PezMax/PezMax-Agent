package agent

import "testing"

func TestExtractDays(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "arabic days", text: "帮我制定2天高数复习计划", want: 2},
		{name: "chinese days", text: "帮我制定两天高数复习计划", want: 2},
		{name: "arabic weeks", text: "7周考研复习规划", want: 49},
		{name: "chinese weeks", text: "两周数据结构备考计划", want: 14},
		{name: "arabic months", text: "1个月高数学习计划", want: 30},
		{name: "chinese months", text: "一个月高数学习计划", want: 30},
		{name: "half month", text: "半个月期末复习安排", want: 15},
		{name: "twenty days", text: "二十天数据库复习计划", want: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractDays(tt.text); got != tt.want {
				t.Fatalf("extractDays(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}
