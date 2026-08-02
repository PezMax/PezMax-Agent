package document

import "testing"

func TestCleanText(t *testing.T) {
	got := CleanText("<html><script>x</script><body>高数&nbsp;期末<br>试卷</body></html>", 20)
	want := "高数 期末 试卷"
	if got != want {
		t.Fatalf("CleanText() = %q, want %q", got, want)
	}
}

func TestCleanTextLimit(t *testing.T) {
	got := CleanText("一二三四五", 3)
	if got != "一二三" {
		t.Fatalf("CleanText() = %q, want %q", got, "一二三")
	}
}
