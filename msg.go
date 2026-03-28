package main

import (
	"time"

	"github.com/cloudwego/eino/schema"
)

// Platform IM 平台标识
type Platform string

const (
	PlatformQQ Platform = "qq"
)

// UnifiedMessage 跨平台统一消息
// ContentBlocks 直接复用 eino/schema 的 ContentBlock，LLM 无需转换直接消费
type UnifiedMessage struct {
	Platform      Platform // 来源平台
	PlatformMsgID string   // 原生消息 ID（用于回复引用）
	ChannelID     string   // 频道/群组 ID
	ChannelName   string   // 频道名称
	GuildID       string   // 频道 ID（群消息场景）

	Author struct {
		PlatformUserID string // 平台原生用户 ID
		Username       string // 显示名
		AvatarURL      string // 头像（可选）
	}

	// 核心：复用 schema.ContentBlock，LLM 直接消费
	ContentBlocks []*schema.ContentBlock

	// 回复目标（群消息需要引用 msgId）
	ReplyToMsgID string

	Timestamp time.Time // 消息时间
	IsBot     bool      // 是否是机器人发的

	// 扩展上下文（由 adapter 填充，供 reply handler 使用）
	Extra map[string]any
}

// MsgChan 全局消息 channel，buffer 默认 100
var MsgChan = make(chan *UnifiedMessage, 100)
