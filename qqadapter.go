package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/tencent-connect/botgo/dto"
)

// qqAdapter QQ 平台适配器：将 botgo 事件转为 UnifiedMessage 推入 channel
type qqAdapter struct {
	msgChan chan<- *UnifiedMessage
}

// NewQQAdapter 创建 QQ Adapter
func NewQQAdapter(msgChan chan<- *UnifiedMessage) *qqAdapter {
	return &qqAdapter{msgChan: msgChan}
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
	// 有图片附件
	for _, att := range attachments {
		if strings.HasPrefix(att.ContentType, "image/") {
			return a.buildImageBlocks(ctx, text, att)
		}
		if att.ContentType == "voice" {
			return a.buildAudioBlocks(ctx, text, att)
		}
	}

	// 纯文本
	return []*schema.ContentBlock{
		schema.NewContentBlock(&schema.UserInputText{Text: text}),
	}
}

// buildImageBlocks 图片消息内容块（下载为 base64 或回退到 URL）
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

// buildAudioBlocks 音频消息内容块
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

// push 推送消息到 channel，带背压保护
func (a *qqAdapter) push(msg *UnifiedMessage) {
	select {
	case a.msgChan <- msg:
	default:
		log.Printf("[警告] channel 满了，丢弃消息 %s", msg.PlatformMsgID)
	}
}

// downloadImageAsBase64 尝试下载图片，失败返回空字符串和错误
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
