package agent

import (
	"context"

	"PezMax-Agent/internal/domain"
)

func (s *Service) Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatResponse, error) {
	if looksLikeSearch(req.Message) {
		search, err := s.SearchFiles(ctx, domain.FileSearchRequest{Query: req.Message})
		if err != nil {
			return domain.ChatResponse{}, err
		}
		return domain.ChatResponse{
			Intent: "file_search",
			Answer: search.Summary,
			Data:   search,
		}, nil
	}

	if s.model == nil {
		return domain.ChatResponse{
			Intent: "general",
			Answer: "智能体服务已启动，但当前未配置 DASHSCOPE_API_KEY。资料搜索、元数据补全和审核建议仍可使用规则兜底能力。",
		}, nil
	}

	msg, err := s.generate(ctx, chatSystemPrompt, req.Message)
	if err != nil {
		return domain.ChatResponse{}, err
	}
	return domain.ChatResponse{Intent: "general", Answer: msg.Content}, nil
}
