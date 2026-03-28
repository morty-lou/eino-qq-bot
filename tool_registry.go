package main

import (
	"fmt"

	"github.com/cloudwego/eino/schema"
)

// GetToolInfos 返回所有工具的 ToolInfo，用于注册到 agenticark
func GetToolInfos() []*schema.ToolInfo {
	return []*schema.ToolInfo{
		weatherTool{}.Info(),
		// 在此添加更多工具...
	}
}

// Invoke 根据工具名称执行对应工具，返回结果字符串
func Invoke(name, argumentsInJSON string) (string, error) {
	switch name {
	case "get_weather":
		return weatherTool{}.Run(argumentsInJSON)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}
