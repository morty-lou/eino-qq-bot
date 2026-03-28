package tools

import (
	"github.com/cloudwego/eino/schema"
)

// WebSearchToolInfo 返回 web_search 工具的 ToolInfo
func WebSearchToolInfo() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "web_search",
		Desc: "搜索互联网获取实时信息。当用户询问新闻、天气、实时数据、百科知识等需要最新信息的问题时使用。返回搜索结果包括标题、链接和摘要。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "搜索关键词或问题",
				Required: true,
			},
			"count": {
				Type:     schema.Integer,
				Desc:     "返回结果数量，默认为 5",
				Required: false,
			},
		}),
	}
}
