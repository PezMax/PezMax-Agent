package agent

import (
	"strings"
	"testing"

	"PezMax-Agent/internal/domain"
)

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

func TestBuildStudyPlanUsesSubjectTopics(t *testing.T) {
	req := normalizeStudyPlanRequest(domain.StudyPlanRequest{
		Goal:        "两天高等数学期末复习",
		Subject:     "高等数学",
		Days:        2,
		HoursPerDay: 2,
	})

	plan := buildStudyPlanDays(req, nil, nil)
	if len(plan) != 2 {
		t.Fatalf("len(plan) = %d, want 2", len(plan))
	}
	if !strings.Contains(plan[0].Title, "函数、极限与连续") {
		t.Fatalf("first day title = %q, want subject topic", plan[0].Title)
	}
	if !strings.Contains(plan[0].Tasks[0].Detail, "等价无穷小") {
		t.Fatalf("first task detail = %q, want concrete math concepts", plan[0].Tasks[0].Detail)
	}
}

func TestBuildFallbackMockQuestions(t *testing.T) {
	req := domain.MockExamRequest{
		Subject:       "高等数学",
		QuestionCount: 5,
		Difficulty:    "中等",
	}

	questions := buildFallbackMockQuestions(req, []domain.FileItem{{FileID: 1, Name: "高等数学期末真题.pdf"}}, nil)
	if len(questions) != 5 {
		t.Fatalf("len(questions) = %d, want 5", len(questions))
	}
	if questions[0].Stem == "" || questions[0].Answer == "" {
		t.Fatalf("question should include stem and answer: %+v", questions[0])
	}
	if !strings.Contains(questions[0].SourceBasis, "高等数学期末真题.pdf") {
		t.Fatalf("source basis = %q, want source file name", questions[0].SourceBasis)
	}
}
