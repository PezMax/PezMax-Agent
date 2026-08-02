package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Result struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Source  string `json:"source,omitempty"`
}

type Client interface {
	Search(ctx context.Context, query string, limit int) ([]Result, error)
	FetchText(ctx context.Context, pageURL string, maxChars int) (string, error)
}

type HTTPClient struct {
	provider string
	apiKey   string
	baseURL  string
	client   *http.Client
}

func NewFromEnv() Client {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("PEZMAX_WEB_SEARCH_PROVIDER")))
	if provider == "" {
		provider = firstConfiguredProvider()
	}
	if provider == "" {
		return NoopClient{}
	}

	return &HTTPClient{
		provider: provider,
		apiKey:   firstNonEmpty(os.Getenv("PEZMAX_WEB_SEARCH_API_KEY"), os.Getenv("TAVILY_API_KEY"), os.Getenv("SERPAPI_API_KEY")),
		baseURL:  strings.TrimRight(os.Getenv("PEZMAX_WEB_SEARCH_BASE_URL"), "/"),
		client: &http.Client{
			Timeout: time.Duration(getEnvInt("PEZMAX_WEB_SEARCH_TIMEOUT_SECONDS", 12)) * time.Second,
		},
	}
}

func firstConfiguredProvider() string {
	switch {
	case os.Getenv("TAVILY_API_KEY") != "":
		return "tavily"
	case os.Getenv("SERPAPI_API_KEY") != "":
		return "serpapi"
	case os.Getenv("PEZMAX_WEB_SEARCH_BASE_URL") != "":
		return "searxng"
	default:
		return ""
	}
}

func (c *HTTPClient) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}

	switch c.provider {
	case "tavily":
		return c.searchTavily(ctx, query, limit)
	case "serpapi":
		return c.searchSerpAPI(ctx, query, limit)
	case "searxng":
		return c.searchSearXNG(ctx, query, limit)
	default:
		return nil, fmt.Errorf("unsupported web search provider: %s", c.provider)
	}
}

func (c *HTTPClient) searchTavily(ctx context.Context, query string, limit int) ([]Result, error) {
	if c.apiKey == "" {
		return nil, errors.New("TAVILY_API_KEY or PEZMAX_WEB_SEARCH_API_KEY is empty")
	}
	endpoint := firstNonEmpty(c.baseURL, "https://api.tavily.com/search")
	body, _ := json.Marshal(map[string]interface{}{
		"api_key":        c.apiKey,
		"query":          query,
		"max_results":    limit,
		"search_depth":   "basic",
		"include_answer": false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	var payload struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := c.doJSON(req, &payload); err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(payload.Results))
	for _, item := range payload.Results {
		out = append(out, Result{Title: item.Title, URL: item.URL, Snippet: item.Content, Source: "tavily"})
	}
	return out, nil
}

func (c *HTTPClient) searchSerpAPI(ctx context.Context, query string, limit int) ([]Result, error) {
	if c.apiKey == "" {
		return nil, errors.New("SERPAPI_API_KEY or PEZMAX_WEB_SEARCH_API_KEY is empty")
	}
	endpoint := firstNonEmpty(c.baseURL, "https://serpapi.com/search.json")
	values := url.Values{}
	values.Set("engine", "google")
	values.Set("q", query)
	values.Set("num", fmt.Sprintf("%d", limit))
	values.Set("api_key", c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var payload struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic_results"`
	}
	if err := c.doJSON(req, &payload); err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(payload.OrganicResults))
	for _, item := range payload.OrganicResults {
		out = append(out, Result{Title: item.Title, URL: item.Link, Snippet: item.Snippet, Source: "serpapi"})
	}
	return out, nil
}

func (c *HTTPClient) searchSearXNG(ctx context.Context, query string, limit int) ([]Result, error) {
	if c.baseURL == "" {
		return nil, errors.New("PEZMAX_WEB_SEARCH_BASE_URL is empty")
	}
	values := url.Values{}
	values.Set("q", query)
	values.Set("format", "json")
	values.Set("language", "zh-CN")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/search?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := c.doJSON(req, &payload); err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(payload.Results))
	for _, item := range payload.Results {
		out = append(out, Result{Title: item.Title, URL: item.URL, Snippet: item.Content, Source: "searxng"})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (c *HTTPClient) doJSON(req *http.Request, target interface{}) error {
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("web search returned status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *HTTPClient) FetchText(ctx context.Context, pageURL string, maxChars int) (string, error) {
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" {
		return "", nil
	}
	if maxChars <= 0 {
		maxChars = 4000
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "PezMax-Agent/1.0")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch page returned status %d", resp.StatusCode)
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "pdf") {
		return "", errors.New("pdf text extraction is not available")
	}
	if !strings.Contains(contentType, "text") && !strings.Contains(contentType, "html") && contentType != "" {
		return "", fmt.Errorf("unsupported content type: %s", contentType)
	}

	var buf bytes.Buffer
	limitReader := io.LimitReader(resp.Body, int64(maxChars*4))
	if _, err := buf.ReadFrom(limitReader); err != nil {
		return "", err
	}
	return cleanPageText(buf.String(), maxChars), nil
}

type NoopClient struct{}

func (NoopClient) Search(context.Context, string, int) ([]Result, error) {
	return nil, errors.New("web search is not configured")
}

func (NoopClient) FetchText(context.Context, string, int) (string, error) {
	return "", errors.New("web search is not configured")
}

func cleanPageText(raw string, maxChars int) string {
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
	if len([]rune(text)) > maxChars {
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
