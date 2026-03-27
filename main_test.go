package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/openapi"
	"github.com/tencent-connect/botgo/token"
)

func init() {
	_ = godotenv.Load()
}

func TestConfig(t *testing.T) {
	appID := os.Getenv("QQ_APPID")
	appSecret := os.Getenv("QQ_SECRET")
	sandbox := os.Getenv("QQ_SANDBOX")

	if appID == "" {
		t.Fatal("QQ_APPID 未设置")
	}
	if appSecret == "" {
		t.Fatal("QQ_SECRET 未设置")
	}
	if sandbox == "" {
		t.Log("QQ_SANDBOX 未设置，默认使用正式环境")
	}

	t.Logf("配置正常: AppID=%s, Sandbox=%s", appID, sandbox)
}

func TestTokenSource(t *testing.T) {
	appID := os.Getenv("QQ_APPID")
	appSecret := os.Getenv("QQ_SECRET")

	if appID == "" || appSecret == "" {
		t.Skip("QQ_APPID or QQ_SECRET not set, skipping")
	}

	creds := &token.QQBotCredentials{
		AppID:     appID,
		AppSecret: appSecret,
	}
	tokenSrc := token.NewQQBotTokenSource(creds)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = ctx

	tk, err := tokenSrc.Token()
	if err != nil {
		t.Fatalf("获取 BotToken 失败: %v", err)
	}
	if tk == nil || tk.AccessToken == "" {
		t.Fatal("BotToken 为空")
	}
	t.Logf("BotToken 获取成功: %s... (有效期 %d 秒)", tk.AccessToken[:20], tk.ExpiresIn)
}

func TestHealthCheck(t *testing.T) {
	appID := os.Getenv("QQ_APPID")
	appSecret := os.Getenv("QQ_SECRET")

	if appID == "" || appSecret == "" {
		t.Skip("QQ_APPID or QQ_SECRET not set, skipping")
	}

	creds := &token.QQBotCredentials{
		AppID:     appID,
		AppSecret: appSecret,
	}
	tokenSrc := token.NewQQBotTokenSource(creds)

	tk, err := tokenSrc.Token()
	if err != nil {
		t.Fatalf("获取 BotToken 失败: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"https://sandbox.api.sgroup.qq.com/gateway/bot", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "QQBot "+tk.AccessToken)
	req.Header.Set("X-Union-Appid", appID)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Gateway 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Gateway 返回错误状态码: %d, body: %s", resp.StatusCode, string(body))
		return
	}

	t.Log("Gateway 凭证验证通过，Bot 可正常连接")
}

func TestIntentCreation(t *testing.T) {
	i := dto.IntentDirectMessages | dto.IntentGroupMessages
	if i == 0 {
		t.Fatal("Intent 组合失败")
	}
	t.Logf("Intent 组合正常: %d", i)
}

func TestSendC2CMessage(t *testing.T) {
	appID := os.Getenv("QQ_APPID")
	appSecret := os.Getenv("QQ_SECRET")
	sandbox := os.Getenv("QQ_SANDBOX")
	testUserID := os.Getenv("QQ_TEST_USER_ID")

	if appID == "" || appSecret == "" {
		t.Skip("QQ_APPID or QQ_SECRET not set, skipping")
	}
	if testUserID == "" {
		t.Fatal("请在 .env 中设置 QQ_TEST_USER_ID（你的用户ID）后再运行此测试")
	}
	for _, c := range testUserID {
		if c < '0' || c > '9' {
			t.Fatalf("QQ_TEST_USER_ID 必须为纯数字，当前值含非数字字符: %s", testUserID)
		}
	}

	creds := &token.QQBotCredentials{
		AppID:     appID,
		AppSecret: appSecret,
	}
	tokenSrc := token.NewQQBotTokenSource(creds)

	var api openapi.OpenAPI
	if sandbox == "true" {
		api = botgo.NewSandboxOpenAPI(appID, tokenSrc)
	} else {
		api = botgo.NewOpenAPI(appID, tokenSrc)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	msg := &dto.MessageToCreate{
		Content: "这是一条来自单测的测试消息！",
		MsgType: dto.TextMsg,
	}

	resp, err := api.PostC2CMessage(ctx, testUserID, msg)
	if err != nil {
		t.Fatalf("发送 C2C 消息失败: %v", err)
	}

	t.Logf("C2C 消息发送成功，msgId: %s, 内容: %s", resp.ID, resp.Content)
}
