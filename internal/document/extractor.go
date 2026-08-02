package document

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"PezMax-Agent/internal/domain"
	"github.com/ledongthuc/pdf"
)

type Extractor interface {
	ExtractFileText(ctx context.Context, file domain.FileItem, maxChars int) domain.DocumentText
}

type HTTPExtractor struct {
	client  *http.Client
	baseURL string
	maxByte int64
}

func NewFromEnv() Extractor {
	return &HTTPExtractor{
		client: &http.Client{
			Timeout: time.Duration(getEnvInt("PEZMAX_DOCUMENT_TIMEOUT_SECONDS", 15)) * time.Second,
		},
		baseURL: strings.TrimRight(firstNonEmpty(os.Getenv("PEZMAX_FILE_BASE_URL"), os.Getenv("PEZMAX_BACKEND_BASE_URL")), "/"),
		maxByte: int64(getEnvInt("PEZMAX_DOCUMENT_MAX_BYTES", 15*1024*1024)),
	}
}

func (e *HTTPExtractor) ExtractFileText(ctx context.Context, file domain.FileItem, maxChars int) domain.DocumentText {
	result := domain.DocumentText{
		FileID:   file.FileID,
		FileName: file.Name,
		FileURL:  file.URL,
		Format:   detectFormat(file.Format, file.URL, ""),
	}
	if strings.TrimSpace(file.URL) == "" {
		result.Error = "文件没有可访问的 fileUrl"
		return result
	}
	if maxChars <= 0 {
		maxChars = 5000
	}

	fileURL, err := e.resolveURL(file.URL)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req.Header.Set("User-Agent", "PezMax-Agent/1.0")

	resp, err := e.client.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = fmt.Sprintf("文件下载返回状态码 %d", resp.StatusCode)
		return result
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	result.Format = detectFormat(file.Format, fileURL, contentType)
	body := io.LimitReader(resp.Body, e.maxByte)
	switch result.Format {
	case "pdf":
		text, err := extractPDFText(body, maxChars)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.Text = text
	case "txt", "text", "html":
		text, err := extractInlineText(body, maxChars)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.Text = text
	default:
		result.Error = fmt.Sprintf("暂不支持解析 %s 格式", result.Format)
	}
	if result.Text == "" && result.Error == "" {
		result.Error = "未从文件中抽取到可用文本，可能是扫描版图片 PDF"
	}
	return result
}

func (e *HTTPExtractor) resolveURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.IsAbs() {
		return raw, nil
	}
	if e.baseURL == "" {
		return "", errors.New("相对 fileUrl 需要配置 PEZMAX_FILE_BASE_URL 或 PEZMAX_BACKEND_BASE_URL")
	}
	if strings.HasPrefix(raw, "/") {
		return e.baseURL + raw, nil
	}
	return e.baseURL + "/" + raw, nil
}

func extractPDFText(reader io.Reader, maxChars int) (string, error) {
	tmp, err := os.CreateTemp("", "pezmax-document-*.pdf")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, reader); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	file, pdfReader, err := pdf.Open(tmpPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	textReader, err := pdfReader.GetPlainText()
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(io.LimitReader(textReader, int64(maxChars*6))); err != nil {
		return "", err
	}
	return CleanText(buf.String(), maxChars), nil
}

func extractInlineText(reader io.Reader, maxChars int) (string, error) {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(io.LimitReader(reader, int64(maxChars*4))); err != nil {
		return "", err
	}
	return CleanText(buf.String(), maxChars), nil
}

func detectFormat(format, fileURL, contentType string) string {
	format = strings.Trim(strings.ToLower(format), ". ")
	if format == "" {
		ext := strings.Trim(strings.ToLower(filepath.Ext(fileURL)), ".")
		format = ext
	}
	switch {
	case strings.Contains(contentType, "pdf"):
		return "pdf"
	case strings.Contains(contentType, "html"):
		return "html"
	case strings.Contains(contentType, "text"):
		return firstNonEmpty(format, "text")
	case format == "pdf" || format == "txt" || format == "text" || format == "html" || format == "htm":
		if format == "htm" {
			return "html"
		}
		return format
	default:
		return firstNonEmpty(format, "unknown")
	}
}

func CleanText(raw string, maxChars int) string {
	text := regexp.MustCompile(`(?is)<script.*?</script>|<style.*?</style>|<noscript.*?</noscript>`).ReplaceAllString(raw, " ")
	text = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(text, " ")
	replacements := map[string]string{
		"&nbsp;": " ",
		"&amp;":  "&",
		"&lt;":   "<",
		"&gt;":   ">",
		"&quot;": `"`,
		"&#39;":  "'",
	}
	for old, next := range replacements {
		text = strings.ReplaceAll(text, old, next)
	}
	text = strings.Join(strings.Fields(text), " ")
	if maxChars > 0 && len([]rune(text)) > maxChars {
		return string([]rune(text)[:maxChars])
	}
	return text
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
