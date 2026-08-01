package llm

import (
	"context"
	"errors"

	"PezMax-Agent/internal/config"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type ChatModel interface {
	Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error)
}

func NewChatModel(ctx context.Context, cfg config.Config) (*openai.ChatModel, error) {
	if cfg.LLMAPIKey == "" {
		return nil, errors.New("DASHSCOPE_API_KEY is empty")
	}
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL:     cfg.LLMBaseURL,
		APIKey:      cfg.LLMAPIKey,
		Model:       cfg.LLMModel,
		Temperature: &cfg.LLMTemperature,
		MaxTokens:   &cfg.LLMMaxTokens,
	})
}
