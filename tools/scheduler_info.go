package tools

import (
	"github.com/cloudwego/eino/schema"
)

// ScheduleTaskToolInfo 返回 schedule_task 工具的 ToolInfo
func ScheduleTaskToolInfo() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "schedule_task",
		Desc: "创建定时任务，让机器人在指定时间自动执行任务并回复用户。reply_to 为 JSON 字符串，私聊格式：{\"type\":\"c2c\",\"user_id\":\"用户ID\"}，群聊格式：{\"type\":\"group\",\"channel_id\":\"频道ID\"}。支持秒级 cron 表达式，如 \"0 8 * * * *\" 表示每天8点00分00秒。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"name": {
				Type:     schema.String,
				Desc:     "任务名称",
				Required: true,
			},
			"cron_expr": {
				Type:     schema.String,
				Desc:     "cron 表达式，支持秒级（6段），如 \"0 8 * * * *\" 表示每天8点00分00秒",
				Required: true,
			},
			"description": {
				Type:     schema.String,
				Desc:     "任务描述",
				Required: false,
			},
			"task_text": {
				Type:     schema.String,
				Desc:     "任务要执行的内容（给机器人看的指令）",
				Required: true,
			},
			"reply_to": {
				Type:     schema.String,
				Desc:     "回复目标 JSON，如 {\"type\":\"c2c\",\"user_id\":\"xxx\"}",
				Required: true,
			},
		}),
	}
}

// ListTasksToolInfo 返回 list_tasks 工具的 ToolInfo
func ListTasksToolInfo() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name:        "list_tasks",
		Desc:        "查询所有已创建的定时任务，返回任务列表包括名称、cron 表达式、下次执行时间等。无需参数。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}
}

// DeleteTaskToolInfo 返回 delete_task 工具的 ToolInfo
func DeleteTaskToolInfo() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "delete_task",
		Desc: "删除指定的定时任务，需要提供任务的 ID。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"id": {
				Type:     schema.Integer,
				Desc:     "要删除的任务 ID",
				Required: true,
			},
		}),
	}
}
