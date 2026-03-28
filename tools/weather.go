package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/cloudwego/eino/schema"
)

// weatherTool 天气查询工具
type WeatherTool struct{}

func (WeatherTool) Info() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "get_weather",
		Desc: "获取城市天气信息，输入城市名称，返回该城市的天气状况和温度。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"city": {Type: schema.String, Desc: "城市名称", Required: true},
		}),
	}
}

func (WeatherTool) Run(argumentsInJSON string) (string, error) {
	var args struct {
		City string `json:"city"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	url := geoCityToURL(args.City)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求天气 API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取天气响应失败: %w", err)
	}

	log.Printf("[工具] get_weather(%s) → %s", args.City, string(body))
	return string(body), nil
}

// geoCityToURL 简化版城市到天气 URL 映射
func geoCityToURL(city string) string {
	coords := map[string][2]float64{
		"北京": {39.9042, 116.4074},
		"上海": {31.2304, 121.4737},
		"广州": {23.1291, 113.2644},
		"深圳": {22.5431, 114.0579},
		"香港": {22.3193, 114.1694},
		"东京": {35.6762, 139.6503},
	}
	if c, ok := coords[city]; ok {
		return fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current_weather=true&timezone=Asia/Shanghai", c[0], c[1])
	}
	return "https://api.open-meteo.com/v1/forecast?latitude=39.9&longitude=116.4&current_weather=true&timezone=Asia/Shanghai"
}
