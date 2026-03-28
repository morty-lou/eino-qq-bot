package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi"
	"github.com/tencent-connect/botgo/token"
	"golang.org/x/oauth2"
)

// 全局 API 客户端
var api openapi.OpenAPI

// 全局 Token Source，供下载资源使用
var tokenSrc oauth2.TokenSource

func main() {
	_ = godotenv.Load()
	ctx := context.Background()

	appIDStr := os.Getenv("QQ_APPID")
	appSecret := os.Getenv("QQ_SECRET")
	sandboxStr := os.Getenv("QQ_SANDBOX")

	if appIDStr == "" || appSecret == "" {
		log.Fatal("请设置环境变量 QQ_APPID 和 QQ_SECRET")
	}

	sandbox := sandboxStr == "true"

	// 1. 初始化 Agent
	if err := InitAgent(ctx); err != nil {
		log.Fatalf("初始化 Agent 失败: %v", err)
	}

	// 2. 创建 Token Source
	creds := &token.QQBotCredentials{
		AppID:     appIDStr,
		AppSecret: appSecret,
	}
	tokenSrc = token.NewQQBotTokenSource(creds)

	// 3. 创建 OpenAPI 客户端
	if sandbox {
		api = botgo.NewSandboxOpenAPI(appIDStr, tokenSrc)
	} else {
		api = botgo.NewOpenAPI(appIDStr, tokenSrc)
	}

	// 4. 创建 QQ Adapter 并注册事件
	qqAdapter := NewQQAdapter(MsgChan)
	registerQQHandlers(qqAdapter)

	// 5. 获取 WebSocket Gateway
	ctxWS, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	apInfo, err := api.WS(ctxWS, nil, "")
	if err != nil {
		log.Fatalf("获取 WebSocket Gateway 失败: %v", err)
	}

	// 6. 启动 Worker（可启动多个并发）
	for range 3 {
		go StartAgentWorker(ctx)
	}

	// 7. 启动 Session Manager
	sm := botgo.NewSessionManager()
	var intents dto.Intent = dto.EventToIntent(dto.EventC2CMessageCreate) |
		dto.EventToIntent(dto.EventGroupAtMessageCreate) |
		dto.EventToIntent(dto.EventDirectMessageCreate)
	if err := sm.Start(apInfo, tokenSrc, &intents); err != nil {
		log.Fatalf("启动 Session Manager 失败: %v", err)
	}

	appID, _ := strconv.ParseUint(appIDStr, 10, 64)
	log.Printf("QQ 机器人已启动 (AppID: %d)，Worker: 3，正在监听消息...", appID)

	select {}
}

// registerQQHandlers 注册 QQ 事件回调，转发给 adapter
func registerQQHandlers(qqAdapter *qqAdapter) {
	event.DefaultHandlers.Ready = func(payload *dto.WSPayload, data *dto.WSReadyData) {
		log.Printf("机器人已就绪，Bot ID: %s, Session ID: %s", data.User.ID, data.SessionID)
	}

	event.DefaultHandlers.C2CMessage = func(payload *dto.WSPayload, data *dto.WSC2CMessageData) error {
		log.Printf("[WS] [私聊] 收到 from %s(ID:%s): %s",
			data.Author.Username, data.Author.ID, data.Content)
		qqAdapter.HandleC2CMessage(context.Background(), data)
		return nil
	}

	event.DefaultHandlers.GroupATMessage = func(payload *dto.WSPayload, data *dto.WSGroupATMessageData) error {
		log.Printf("[WS] [群消息] 来自 %s/%s，%s(ID:%s): %s",
			data.GuildID, data.ChannelID, data.Author.Username, data.Author.ID, data.Content)
		qqAdapter.HandleGroupATMessage(context.Background(), data)
		return nil
	}

	event.DefaultHandlers.DirectMessage = func(payload *dto.WSPayload, data *dto.WSDirectMessageData) error {
		log.Printf("[WS] [频道私信] 来自 %s/%s，%s(ID:%s): %s",
			data.GuildID, data.ChannelID, data.Author.Username, data.Author.ID, data.Content)
		return nil
	}
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

// sendC2CReply 发送 C2C 私信
func sendC2CReply(ctx context.Context, userID, content, msgID string) error {
	dm, err := api.CreateDirectMessage(ctx, &dto.DirectMessageToCreate{
		RecipientID: userID,
	})
	if err != nil {
		return sendC2CMessageDirect(ctx, userID, content)
	}

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
