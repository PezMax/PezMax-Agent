package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"PezMax-Agent/internal/agent"
	"PezMax-Agent/internal/api"
	"PezMax-Agent/internal/backend"
	"PezMax-Agent/internal/config"
	pezllm "PezMax-Agent/internal/llm"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	chatModel, err := pezllm.NewChatModel(ctx, cfg)
	if err != nil {
		log.Printf("llm disabled: %v", err)
	}

	backendClient := backend.NewJavaClient(cfg)
	service := agent.NewService(chatModel, backendClient)
	handler := api.NewHandler(service)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("PezMax-Agent listening on %s", cfg.HTTPAddr)
	log.Fatal(server.ListenAndServe())
}
