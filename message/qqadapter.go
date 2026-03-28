package message

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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

// 全局 API 客户端，供 reply 使用
var api openapi.OpenAPI

// 全局 Token Source，供下载资源使用
var tokenSrc oauth2.TokenSource

// qqAdapter QQ 平台适配器
type qqAdapter struct {
	msgChan chan<- *UnifiedMessage
}

// GetAdapter 创建 QQ Adapter
func GetAdapter() *qqAdapter {
	return &qqAdapter{msgChan: MsgChan}
}

// API 返回全局 OpenAPI 客户端
func API() openapi.OpenAPI { return api }

// TokenSrc 返回全局 Token Source
func TokenSrc() oauth2.TokenSource { return tokenSrc }

// Init 初始化 QQ Bot：读取配置、创建客户端、注册事件回调
// 供 main 的 init() 调用
func Init() {
	_ = godotenv.Load()

	appIDStr := os.Getenv("QQ_APPID")
	appSecret := os.Getenv("QQ_SECRET")
	sandboxStr := os.Getenv("QQ_SANDBOX")

	if appIDStr == "" || appSecret == "" {
		log.Fatal("请设置环境变量 QQ_APPID 和 QQ_SECRET")
	}

	sandbox := sandboxStr == "true"

	// 创建 Token Source
	creds := &token.QQBotCredentials{
		AppID:     appIDStr,
		AppSecret: appSecret,
	}
	tokenSrc = token.NewQQBotTokenSource(creds)

	// 创建 OpenAPI 客户端
	if sandbox {
		api = botgo.NewSandboxOpenAPI(appIDStr, tokenSrc)
	} else {
		api = botgo.NewOpenAPI(appIDStr, tokenSrc)
	}

	// 注册事件回调
	registerQQHandlers(GetAdapter())

	// 启动 WS
	go startWS(appIDStr)
}

func startWS(appIDStr string) {
	ctxWS, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	apInfo, err := api.WS(ctxWS, nil, "")
	if err != nil {
		log.Fatalf("获取 WebSocket Gateway 失败: %v", err)
	}

	sm := botgo.NewSessionManager()
	var intents dto.Intent = dto.EventToIntent(dto.EventC2CMessageCreate) |
		dto.EventToIntent(dto.EventGroupAtMessageCreate) |
		dto.EventToIntent(dto.EventDirectMessageCreate)
	if err := sm.Start(apInfo, tokenSrc, &intents); err != nil {
		log.Fatalf("启动 Session Manager 失败: %v", err)
	}

	select {}
}

// registerQQHandlers 注册 QQ 事件回调
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

// HandleC2CMessage 处理私聊消息
func (a *qqAdapter) HandleC2CMessage(ctx context.Context, data *dto.WSC2CMessageData) {
	msg := &UnifiedMessage{
		Platform:      PlatformQQ,
		PlatformMsgID: data.ID,
		Author: struct {
			PlatformUserID string
			Username       string
			AvatarURL      string
		}{
			PlatformUserID: data.Author.ID,
			Username:       data.Author.Username,
		},
		Timestamp:     time.Now(),
		IsBot:         data.Author.Bot,
		ContentBlocks: a.buildContentBlocks(ctx, data.Content, data.Attachments),
		Extra: map[string]any{
			"type":    "c2c",
			"user_id": data.Author.ID,
			"msg_id":  data.ID,
		},
	}
	a.push(msg)
}

// HandleGroupATMessage 处理群@消息
func (a *qqAdapter) HandleGroupATMessage(ctx context.Context, data *dto.WSGroupATMessageData) {
	msg := &UnifiedMessage{
		Platform:      PlatformQQ,
		PlatformMsgID: data.ID,
		ChannelID:     data.ChannelID,
		GuildID:       data.GuildID,
		Author: struct {
			PlatformUserID string
			Username       string
			AvatarURL      string
		}{
			PlatformUserID: data.Author.ID,
			Username:       data.Author.Username,
		},
		Timestamp:     time.Now(),
		IsBot:         data.Author.Bot,
		ContentBlocks: a.buildContentBlocks(ctx, data.Content, data.Attachments),
		Extra: map[string]any{
			"type":       "group",
			"channel_id": data.ChannelID,
			"guild_id":   data.GuildID,
			"msg_id":     data.ID,
		},
	}
	a.push(msg)
}

// buildContentBlocks 根据消息内容构造 ContentBlocks
func (a *qqAdapter) buildContentBlocks(ctx context.Context, text string, attachments []*dto.MessageAttachment) []*schema.ContentBlock {
	for _, att := range attachments {
		if strings.HasPrefix(att.ContentType, "image/") {
			return a.buildImageBlocks(ctx, text, att)
		}
		if att.ContentType == "voice" {
			return a.buildAudioBlocks(ctx, text, att)
		}
	}
	return []*schema.ContentBlock{
		schema.NewContentBlock(&schema.UserInputText{Text: text}),
	}
}

// buildImageBlocks 图片消息
func (a *qqAdapter) buildImageBlocks(ctx context.Context, text string, att *dto.MessageAttachment) []*schema.ContentBlock {
	displayText := text
	if displayText == "" {
		displayText = "用户发来一张图片，请浏览并根据这张图片进行回答"
	}

	var imageInput *schema.UserInputImage
	b64, err := downloadImageAsBase64(ctx, att.URL)
	if err != nil {
		log.Printf("[图片] 下载失败，回退使用URL: %v", err)
		mimeType := att.ContentType
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
		imageInput = &schema.UserInputImage{
			URL:      att.URL,
			MIMEType: mimeType,
			Detail:   schema.ImageURLDetailAuto,
		}
	} else {
		mimeType := att.ContentType
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
		imageInput = &schema.UserInputImage{
			Base64Data: b64,
			MIMEType:   mimeType,
			Detail:     schema.ImageURLDetailAuto,
		}
		log.Printf("[图片] 下载成功，大小: %.1fKB", float64(len(b64))/1024)
	}

	return []*schema.ContentBlock{
		schema.NewContentBlock(&schema.UserInputText{Text: displayText}),
		schema.NewContentBlock(imageInput),
	}
}

// buildAudioBlocks 音频消息
func (a *qqAdapter) buildAudioBlocks(ctx context.Context, text string, att *dto.MessageAttachment) []*schema.ContentBlock {
	displayText := text
	if displayText == "" {
		displayText = "用户发来一段语音消息"
	}
	return []*schema.ContentBlock{
		schema.NewContentBlock(&schema.UserInputText{Text: displayText}),
		schema.NewContentBlock(&schema.UserInputAudio{
			URL:      att.URL,
			MIMEType: att.ContentType,
		}),
	}
}

// push 推送消息到 channel
func (a *qqAdapter) push(msg *UnifiedMessage) {
	select {
	case a.msgChan <- msg:
	default:
		log.Printf("[警告] channel 满了，丢弃消息 %s", msg.PlatformMsgID)
	}
}

// downloadImageAsBase64 下载图片并转为 base64
func downloadImageAsBase64(ctx context.Context, imageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", err
	}
	tk, err := tokenSrc.Token()
	if err != nil {
		return "", fmt.Errorf("获取token失败: %w", err)
	}
	req.Header.Set("Authorization", tk.TokenType+" "+tk.AccessToken)
	req.Header.Set("User-Agent", "QQBot/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return "", fmt.Errorf("下载图片返回状态码%d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取图片数据失败: %w", err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// SendC2CReply 发送 C2C 私信
func SendC2CReply(ctx context.Context, userID, content, msgID string) error {
	dm, err := api.CreateDirectMessage(ctx, &dto.DirectMessageToCreate{
		RecipientID: userID,
	})
	if err != nil {
		return SendC2CMessageDirect(ctx, userID, content)
	}

	_, err = api.PostDirectMessage(ctx, dm, &dto.MessageToCreate{
		Content: content,
		MsgID:   msgID,
	})
	return err
}

// SendC2CMessageDirect 直接发送私信
func SendC2CMessageDirect(ctx context.Context, userID, content string) error {
	_, err := api.PostC2CMessage(ctx, userID, &dto.MessageToCreate{
		Content: content,
	})
	return err
}
