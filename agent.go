package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/agenticark"
)

// chatModel 全局 LLM 模型，供 worker 使用
var chatModel *agenticark.Model

// InitAgent 初始化 agenticark 模型
func InitAgent(ctx context.Context) error {
	var err error
	chatModel, err = agenticark.New(ctx, &agenticark.Config{
		APIKey: os.Getenv("ARK_API_KEY"),
		Model:  os.Getenv("ARK_MODEL"),
	})
	if err != nil {
		return err
	}

	log.Println("Agent 初始化完成")
	return nil
}
