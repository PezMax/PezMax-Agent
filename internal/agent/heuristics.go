package agent

import (
	"regexp"
	"strconv"
	"strings"
)

func extractYear(text string) int {
	re := regexp.MustCompile(`(20\d{2})`)
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0
	}
	year, _ := strconv.Atoi(match[1])
	if year < 2000 || year > 2100 {
		return 0
	}
	return year
}

func extractFileType(text string) int {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "期末"), strings.Contains(lower, "final"):
		return 1
	case strings.Contains(lower, "期中"), strings.Contains(lower, "midterm"):
		return 2
	case strings.Contains(lower, "补考"):
		return 4
	case strings.Contains(lower, "其他学校"):
		return 5
	case strings.Contains(lower, "资料"), strings.Contains(lower, "笔记"), strings.Contains(lower, "复习"):
		return 3
	default:
		return 0
	}
}

func extractSubject(text string) string {
	aliases := []struct {
		alias   string
		subject string
	}{
		{"高等数学", "高等数学"},
		{"高数", "高等数学"},
		{"线性代数", "线性代数"},
		{"线代", "线性代数"},
		{"概率论与数理统计", "概率论与数理统计"},
		{"概率统计", "概率论与数理统计"},
		{"概率论", "概率论与数理统计"},
		{"大学物理", "大学物理"},
		{"大物", "大学物理"},
		{"大学英语", "大学英语"},
		{"英语", "大学英语"},
		{"数据结构", "数据结构"},
		{"计算机网络", "计算机网络"},
		{"操作系统", "操作系统"},
		{"数据库", "数据库"},
		{"离散数学", "离散数学"},
		{"电路", "电路"},
		{"模拟电子技术", "模拟电子技术"},
		{"数字电子技术", "数字电子技术"},
		{"C语言", "C语言"},
		{"Java", "Java"},
		{"Python", "Python"},
	}
	for _, item := range aliases {
		if strings.Contains(strings.ToLower(text), strings.ToLower(item.alias)) {
			return item.subject
		}
	}
	return ""
}

func normalizeSubject(subject string) string {
	if subject == "" {
		return ""
	}
	normalized := extractSubject(subject)
	if normalized != "" {
		return normalized
	}
	return subject
}

func fileTypeName(value int) string {
	switch value {
	case 1:
		return "期末"
	case 2:
		return "期中"
	case 3:
		return "资料"
	case 4:
		return "补考"
	case 5:
		return "其他学校"
	default:
		return ""
	}
}

func isSuspiciousText(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{"qq", "微信", "vx", "代考", "广告", "加群", "http://", "https://"}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}
