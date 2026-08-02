package agent

import (
	"context"
	"encoding/json"
	"strings"

	"PezMax-Agent/internal/backend"
	pezllm "PezMax-Agent/internal/llm"
	"PezMax-Agent/internal/websearch"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type Service struct {
	model     pezllm.ChatModel
	backend   backend.Client
	webSearch websearch.Client
}

func NewService(model pezllm.ChatModel, backend backend.Client, webSearch websearch.Client) *Service {
	return &Service{model: model, backend: backend, webSearch: webSearch}
}

func (s *Service) generate(ctx context.Context, systemPrompt, userPrompt string) (*schema.Message, error) {
	return s.model.Generate(ctx, []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(userPrompt),
	}, model.WithTemperature(0.2))
}

const searchSystemPrompt = "你是 PezMax 试题下载平台的搜索参数抽取助手。你必须输出严格 JSON，不要解释。"
const metadataSystemPrompt = "你是 PezMax 试题下载平台的上传资料元数据补全助手。你必须输出严格 JSON，不要解释。"
const auditSystemPrompt = "你是 PezMax 试题下载平台的审核辅助助手。你必须输出严格 JSON，不要解释。"
const studySystemPrompt = "你是 PezMax 试题下载平台的学习规划智能体。你必须基于平台资料、网络搜索摘要和科目知识结构生成具体可执行计划，并输出严格 JSON。"
const mockExamSystemPrompt = "你是 PezMax 试题下载平台的模拟题生成智能体。你必须根据平台真题资料、网络题型摘要和科目知识结构生成原创模拟题，并输出严格 JSON。"
const chatSystemPrompt = "你是 PezMax 试题下载平台助手，回答要简洁、准确，优先围绕找资料、上传资料和审核建议。"

func containsFold(text, keyword string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(keyword))
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func decodeJSONObject(content string, target interface{}) error {
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end >= start {
		content = content[start : end+1]
	}
	return json.Unmarshal([]byte(content), target)
}

func mustJSON(value interface{}) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
