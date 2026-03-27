package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi"
	"github.com/tencent-connect/botgo/token"
)

// 全局 API 客户端，供事件处理器使用
var api openapi.OpenAPI

// 全局 LLM 模型
var chatModel *agenticopenai.Model

func main() {
	_ = godotenv.Load()
	appIDStr := os.Getenv("QQ_APPID")
	appSecret := os.Getenv("QQ_SECRET")
	sandboxStr := os.Getenv("QQ_SANDBOX")

	if appIDStr == "" || appSecret == "" {
		log.Fatal("请设置环境变量 QQ_APPID 和 QQ_SECRET")
	}

	sandbox := sandboxStr == "true" // 是否使用沙箱环境

	// 1. 创建 Token Source
	creds := &token.QQBotCredentials{
		AppID:     appIDStr,
		AppSecret: appSecret,
	}
	tokenSrc := token.NewQQBotTokenSource(creds)

	// 2. 创建 OpenAPI 客户端
	if sandbox {
		api = botgo.NewSandboxOpenAPI(appIDStr, tokenSrc)
	} else {
		api = botgo.NewOpenAPI(appIDStr, tokenSrc)
	}

	// 3. 初始化 LLM 模型
	chatModel, _ = agenticopenai.New(context.Background(), &agenticopenai.Config{
		BaseURL: os.Getenv("BASE_URL"),
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   os.Getenv("OPENAI_MODEL_ID"),
	})
	log.Println("LLM 模型初始化完成")

	// 4. 手动注册 handler，避免 Go 类型匹配问题
	intents := registerHandlers()

	log.Printf("注册的 Intents: %d (0x%x)", intents, intents)

	// 5. 获取 WebSocket Gateway
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	apInfo, err := api.WS(ctx, nil, "")
	if err != nil {
		log.Fatalf("获取 WebSocket Gateway 失败: %v", err)
	}

	// 6. 启动 Session Manager
	sm := botgo.NewSessionManager()
	if err := sm.Start(apInfo, tokenSrc, &intents); err != nil {
		log.Fatalf("启动 Session Manager 失败: %v", err)
	}

	appID, _ := strconv.ParseUint(appIDStr, 10, 64)
	log.Printf("QQ 机器人已启动 (AppID: %d)，正在监听消息...", appID)

	select {}
}

// registerHandlers 手动注册所有 handler 并返回对应的 intents
func registerHandlers() dto.Intent {
	var i dto.Intent

	// Ready handler
	event.DefaultHandlers.Ready = func(payload *dto.WSPayload, data *dto.WSReadyData) {
		log.Printf("机器人已就绪，Bot ID: %s, Session ID: %s", data.User.ID, data.SessionID)
	}

	// C2C 私聊消息
	event.DefaultHandlers.C2CMessage = func(payload *dto.WSPayload, data *dto.WSC2CMessageData) error {
		log.Printf("[私聊] 收到 from %s(ID:%s): %s",
			data.Author.Username, data.Author.ID, data.Content)
		log.Println("查看附件内容：", data.Attachments[0].URL)
		// 启动 goroutine 处理 LLM，不阻塞回调
		go handleC2CMessageWithLLM(data)
		return nil
	}
	i = i | dto.EventToIntent(dto.EventC2CMessageCreate)

	// 群@消息
	event.DefaultHandlers.GroupATMessage = func(payload *dto.WSPayload, data *dto.WSGroupATMessageData) error {
		log.Printf("[群消息] 来自 %s/%s，%s(ID:%s): %s",
			data.GuildID, data.ChannelID, data.Author.Username, data.Author.ID, data.Content)

		go handleGroupMessageWithLLM(data)
		return nil
	}
	i = i | dto.EventToIntent(dto.EventGroupAtMessageCreate)

	// 频道私信
	event.DefaultHandlers.DirectMessage = func(payload *dto.WSPayload, data *dto.WSDirectMessageData) error {
		log.Printf("[频道私信] 来自 %s/%s，%s(ID:%s): %s",
			data.GuildID, data.ChannelID, data.Author.Username, data.Author.ID, data.Content)
		return nil
	}
	i = i | dto.EventToIntent(dto.EventDirectMessageCreate)

	return i
}

// extractText 从 AgenticMessage 中提取文本内容
func extractText(msg *schema.AgenticMessage) string {
	for _, block := range msg.ContentBlocks {
		if block.Type == schema.ContentBlockTypeAssistantGenText && block.AssistantGenText != nil {
			return block.AssistantGenText.Text
		}
	}
	return ""
}

// handleC2CMessageWithLLM 调用 LLM 生成回复并发送
func handleC2CMessageWithLLM(data *dto.WSC2CMessageData) {
	ctx := context.Background()

	// 构造消息内容，支持文字+语音附件
	msgs := []*schema.AgenticMessage{
		schema.SystemAgenticMessage("你是一位可爱的小猫娘对话小助手"),
	}

	// 如果有语音附件，加入音频内容块
	if len(data.Attachments) > 0 && data.Attachments[0].ContentType == "image" {
		msgs = append(msgs, &schema.AgenticMessage{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputImage{
					URL:      data.Attachments[0].URL,
					MIMEType: "image",
				}),
			},
		})
	} else {
		msgs = append(msgs, schema.UserAgenticMessage(data.Content))
	}

	// 调用 LLM
	resp, err := chatModel.Generate(ctx, msgs)
	if err != nil {
		log.Printf("[LLM] 生成回复失败: %v", err)
		return
	}
	log.Printf("[LLM] 生成回复: %s", extractText(resp))

	// 发送回复
	content := extractText(resp)
	if err := sendC2CReply(ctx, data.Author.ID, content, data.ID); err != nil {
		log.Printf("[私聊] 发送回复失败: %v", err)
	} else {
		log.Printf("[私聊] 已回复: %s", content)
	}
}

// handleGroupMessageWithLLM 调用 LLM 生成群消息回复
func handleGroupMessageWithLLM(data *dto.WSGroupATMessageData) {
	ctx := context.Background()

	// 构造消息内容，支持文字+语音附件
	var msgs []*schema.AgenticMessage

	// 如果有语音附件，加入音频内容块
	if len(data.Attachments) > 0 && data.Attachments[0].ContentType == "voice" {
		msgs = append(msgs, &schema.AgenticMessage{
			Role: schema.AgenticRoleTypeUser,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.UserInputText{Text: data.Content}),
				schema.NewContentBlock(&schema.UserInputAudio{
					URL:      data.Attachments[0].URL,
					MIMEType: data.Attachments[0].ContentType,
				}),
			},
		})
	} else {
		msgs = append(msgs, schema.UserAgenticMessage(data.Content))
	}

	// 调用 LLM
	resp, err := chatModel.Generate(ctx, msgs)
	if err != nil {
		log.Printf("[LLM] 生成回复失败: %v", err)
		return
	}
	log.Printf("[LLM] 生成回复: %s", extractText(resp))

	// 发送回复
	msg := &dto.MessageToCreate{
		Content: extractText(resp),
		MsgID:   data.ID,
	}
	if _, err := api.PostMessage(ctx, data.ChannelID, msg); err != nil {
		log.Printf("[群消息] 发送回复失败: %v", err)
	} else {
		log.Printf("[群消息] 已回复: %s", msg.Content)
	}
}

// sendC2CReply 发送 C2C 私信
func sendC2CReply(ctx context.Context, userID, content, msgID string) error {
	// 先创建私信会话
	dm, err := api.CreateDirectMessage(ctx, &dto.DirectMessageToCreate{
		RecipientID: userID,
	})
	if err != nil {
		// 创建失败，尝试直接发送
		return sendC2CMessageDirect(ctx, userID, content)
	}

	// 通过会话发送
	_, err = api.PostDirectMessage(ctx, dm, &dto.MessageToCreate{
		Content: content,
		MsgID:   msgID,
	})
	return err
}

// sendC2CMessageDirect 直接发送私信（备用方案）
func sendC2CMessageDirect(ctx context.Context, userID, content string) error {
	_, err := api.PostC2CMessage(ctx, userID, &dto.MessageToCreate{
		Content: content,
	})
	return err
}
