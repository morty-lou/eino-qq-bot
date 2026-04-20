package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino/schema"
)

// SearchProvider 搜索接口
type SearchProvider interface {
	Search(ctx context.Context, query string, count int) (string, error)
}

// searchProvider 全局搜索 provider
var searchProvider SearchProvider

// InitProviders 初始化工具 provider
func InitProviders() {
	searchProvider = NewDuckDuckGoSearchProvider()
	log.Println("[工具] DuckDuckGo Search provider 已初始化（免费，无需 API Key）")
}

// GetToolInfos 返回所有工具的 ToolInfo
func GetToolInfos() []*schema.ToolInfo {
	return []*schema.ToolInfo{
		// WeatherTool{}.Info(),
		WebSearchToolInfo(),
		ScheduleTaskToolInfo(),
		ListTasksToolInfo(),
		DeleteTaskToolInfo(),
	}
}

// Invoke 根据工具名称执行对应工具
func Invoke(ctx context.Context, name, argumentsInJSON string) (string, error) {
	switch name {
	//case "get_weather":
	//	return WeatherTool{}.Run(argumentsInJSON)
	case "web_search":
		return invokeWebSearch(ctx, argumentsInJSON)
	case "schedule_task":
		return invokeScheduleTask(ctx, argumentsInJSON)
	case "list_tasks":
		return invokeListTasks(ctx, argumentsInJSON)
	case "delete_task":
		return invokeDeleteTask(ctx, argumentsInJSON)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func invokeWebSearch(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Query string `json:"query"`
		Count int    `json:"count"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}
	if args.Query == "" {
		return "", fmt.Errorf("query 参数不能为空")
	}
	count := args.Count
	if count <= 0 {
		count = 5
	}
	if count > 10 {
		count = 10
	}

	result, err := searchProvider.Search(ctx, args.Query, count)
	if err != nil {
		return "", fmt.Errorf("搜索失败: %w", err)
	}
	log.Printf("[工具] web_search(%s, count=%d) → 成功", args.Query, count)
	return result, nil
}
