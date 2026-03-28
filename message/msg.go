package message

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
type UnifiedMessage struct {
	Platform      Platform
	PlatformMsgID string
	ChannelID     string
	ChannelName   string
	GuildID       string

	Author struct {
		PlatformUserID string
		Username       string
		AvatarURL      string
	}

	ContentBlocks []*schema.ContentBlock

	ReplyToMsgID string

	Timestamp time.Time
	IsBot     bool

	Extra map[string]any
}

// MsgChan 全局消息 channel
var MsgChan = make(chan *UnifiedMessage, 100)
