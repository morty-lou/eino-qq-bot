package main

import (
	"context"
	"log"

	"github.com/joho/godotenv"
	"qqbottest/message"
	"qqbottest/tools"
)

func init() {
	_ = godotenv.Load()
	message.Init()
}

func main() {
	ctx := context.Background()

	tools.InitProviders()
	if err := InitAgent(ctx); err != nil {
		log.Fatalf("初始化 Agent 失败: %v", err)
	}

	for range 3 {
		go StartAgentWorker(ctx)
	}

	select {}
}
