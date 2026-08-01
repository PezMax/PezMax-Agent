package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr       string
	BackendBaseURL string
	BackendToken   string
	BackendTimeout time.Duration

	LLMBaseURL     string
	LLMAPIKey      string
	LLMModel       string
	LLMTemperature float32
	LLMMaxTokens   int
}

func Load() Config {
	return Config{
		HTTPAddr:       getEnv("PEZMAX_AGENT_ADDR", ":8090"),
		BackendBaseURL: trimRightSlash(os.Getenv("PEZMAX_BACKEND_BASE_URL")),
		BackendToken:   os.Getenv("PEZMAX_BACKEND_TOKEN"),
		BackendTimeout: time.Duration(getEnvInt("PEZMAX_BACKEND_TIMEOUT_SECONDS", 15)) * time.Second,

		LLMBaseURL:     getEnv("PEZMAX_LLM_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		LLMAPIKey:      os.Getenv("DASHSCOPE_API_KEY"),
		LLMModel:       getEnv("PEZMAX_LLM_MODEL", "qwen-plus"),
		LLMTemperature: float32(getEnvFloat("PEZMAX_LLM_TEMPERATURE", 0.2)),
		LLMMaxTokens:   getEnvInt("PEZMAX_LLM_MAX_TOKENS", 1200),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvFloat(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return fallback
	}
	return parsed
}

func trimRightSlash(value string) string {
	for len(value) > 0 && value[len(value)-1] == '/' {
		value = value[:len(value)-1]
	}
	return value
}
