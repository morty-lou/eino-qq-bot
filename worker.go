package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/tencent-connect/botgo/dto"
	"qqbottest/message"
	"qqbottest/tools"
)

// StartAgentWorker 启动 worker goroutine
func StartAgentWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[Worker] context 被取消，退出")
			return
		case msg := <-message.MsgChan:
			if msg == nil {
				continue
			}
			processMessage(ctx, msg)
		}
	}
}

func processMessage(ctx context.Context, msg *message.UnifiedMessage) {
	if msg.IsBot {
		return
	}

	log.Printf("[Worker] 收到消息 from %s (平台:%s): %s",
		msg.Author.Username, msg.Platform, describeContent(msg.ContentBlocks))

	// 将消息元信息注入 LLM 上下文，使其能正确使用 reply 相关工具
	platformStr := string(msg.Platform)
	authorStr := fmt.Sprintf("%s (ID: %s)", msg.Author.Username, msg.Author.PlatformUserID)

	metaText := fmt.Sprintf("[消息上下文] 平台: %s | 用户: %s", platformStr, authorStr)

	contentBlocks := append(
		[]*schema.ContentBlock{schema.NewContentBlock(&schema.UserInputText{Text: metaText})},
		msg.ContentBlocks...,
	)

	today := time.Now().Format("2006年01月02日 15:04")
	systemPrompt := fmt.Sprintf(
		"你是一位可爱的对话小助手，你会收到来自多个平台不同用户的消息，请你耐心温柔的帮助用户解答问题。当前时间：%s。当你需要查询实时信息时，可以使用 web_search 工具搜索互联网。适当使用工具获取实时信息，不要凭空编造事实。",
		today,
	)

	allMsgs := []*schema.AgenticMessage{
		schema.SystemAgenticMessage(systemPrompt),
		{
			Role:          schema.AgenticRoleTypeUser,
			ContentBlocks: contentBlocks,
		},
	}

	functionTools := tools.GetToolInfos()
	opts := []model.Option{model.WithTools(functionTools)}

	var replyText string

	for range 20 {
		resp, err := chatModel.Generate(ctx, allMsgs, opts...)
		if err != nil {
			log.Printf("[Agent] 生成失败: %v", err)
			replyText = "抱歉，处理消息时出错了。"
			break
		}

		hasToolCall := false
		respAppended := false

		for _, block := range resp.ContentBlocks {
			if block.Type == schema.ContentBlockTypeFunctionToolCall && block.FunctionToolCall != nil {
				hasToolCall = true
				tc := block.FunctionToolCall

				log.Printf("[Agent] 调用函数工具: %s, 参数: %s", tc.Name, tc.Arguments)

				result, toolErr := tools.Invoke(ctx, tc.Name, tc.Arguments)
				if toolErr != nil {
					result = `{"error": "` + toolErr.Error() + `"}`
				}

				allMsgs = append(allMsgs, resp)
				respAppended = true
				allMsgs = append(allMsgs, schema.FunctionToolResultAgenticMessage(
					tc.CallID, tc.Name, result,
				))
			}

			if block.Type == schema.ContentBlockTypeServerToolCall && block.ServerToolCall != nil {
				hasToolCall = true
				log.Printf("[Agent] 服务端工具调用: %s", block.ServerToolCall.Name)
			}
		}

		if hasToolCall {
			if !respAppended {
				allMsgs = append(allMsgs, resp)
			}
			continue
		}

		replyText = extractText(resp)
		break
	}

	if replyText != "" {
		replyToPlatform(ctx, msg, replyText)
	}
}

func extractText(msg *schema.AgenticMessage) string {
	for _, block := range msg.ContentBlocks {
		if block.Type == schema.ContentBlockTypeAssistantGenText && block.AssistantGenText != nil {
			return block.AssistantGenText.Text
		}
	}
	return ""
}

func describeContent(blocks []*schema.ContentBlock) string {
	for _, b := range blocks {
		if b.Type == schema.ContentBlockTypeUserInputText && b.UserInputText != nil {
			return b.UserInputText.Text
		}
	}
	return "(多模态消息)"
}

func replyToPlatform(ctx context.Context, msg *message.UnifiedMessage, text string) {
	switch msg.Platform {
	case message.PlatformQQ:
		replyToQQ(ctx, msg, text)
	default:
		log.Printf("[Worker] 未知平台: %s，无法回复", msg.Platform)
	}
}

func replyToQQ(ctx context.Context, msg *message.UnifiedMessage, text string) {
	extra := msg.Extra
	if extra == nil {
		log.Printf("[Worker] QQ 消息缺少 Extra 上下文，无法回复")
		return
	}

	switch extra["type"] {
	case "c2c":
		replyToQQC2C(ctx, extra, text)
	case "group":
		replyToQQGroup(ctx, extra, text)
	default:
		log.Printf("[Worker] 未知 QQ 消息类型: %v", extra["type"])
	}
}

func replyToQQC2C(ctx context.Context, extra map[string]any, text string) {
	userID, ok := extra["user_id"].(string)
	if !ok {
		log.Printf("[Worker] C2C 缺少 user_id")
		return
	}
	msgID, _ := extra["msg_id"].(string)
	if err := message.SendC2CReply(ctx, userID, text, msgID); err != nil {
		log.Printf("[Worker] C2C 回复失败: %v", err)
	} else {
		log.Printf("[Worker] C2C 已回复: %s", text)
	}
}

func replyToQQGroup(ctx context.Context, extra map[string]any, text string) {
	channelID, ok := extra["channel_id"].(string)
	if !ok {
		log.Printf("[Worker] Group 消息缺少 channel_id")
		return
	}
	msgID, _ := extra["msg_id"].(string)
	if _, err := message.API().PostMessage(ctx, channelID, &dto.MessageToCreate{
		Content: text,
		MsgID:   msgID,
	}); err != nil {
		log.Printf("[Worker] 群消息回复失败: %v", err)
	} else {
		log.Printf("[Worker] 群消息已回复: %s", text)
	}
}
