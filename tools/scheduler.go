package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/robfig/cron/v3"
	"qqbottest/message"
)

// ScheduledTask 定时任务记录
type ScheduledTask struct {
	ID          cron.EntryID `json:"id"`
	Name        string       `json:"name"`
	CronExpr    string       `json:"cron_expr"`
	Description string       `json:"description"`
	CreatedAt   time.Time    `json:"created_at"`
	NextRun     time.Time    `json:"next_run"`
}

// SchedulerManager 定时任务管理器
type SchedulerManager struct {
	c     *cron.Cron
	tasks map[cron.EntryID]ScheduledTask
	mu    sync.RWMutex
}

// 全局调度器
var schedulerMgr *SchedulerManager

// InitScheduler 初始化调度器
func InitScheduler() {
	schedulerMgr = NewSchedulerManager()
	schedulerMgr.Start()
	log.Println("[调度] 定时任务管理器已启动")
}

// NewSchedulerManager 创建调度器
func NewSchedulerManager() *SchedulerManager {
	c := cron.New(
		cron.WithSeconds(),
		cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		),
	)
	return &SchedulerManager{
		c:     c,
		tasks: make(map[cron.EntryID]ScheduledTask),
	}
}

// Start 启动调度器
func (m *SchedulerManager) Start() { m.c.Start() }

// Stop 停止调度器
func (m *SchedulerManager) Stop() { m.c.Stop() }

// AddTask 新增定时任务
// extra 是 map[string]any，存放平台回复所需的上下文：
//
//	QQ C2C: {"type":"c2c","user_id":"xxx","msg_id":"yyy"}
//	QQ 群:  {"type":"group","channel_id":"xxx","guild_id":"yyy","msg_id":"zzz"}
func (m *SchedulerManager) AddTask(name, cronExpr, description, taskText string, extra map[string]any) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var newEntryID cron.EntryID
	newEntryID, err := m.c.AddFunc(cronExpr, func() {
		m.fireTask(newEntryID, taskText, extra)
	})
	if err != nil {
		return 0, fmt.Errorf("无效的 cron 表达式: %w", err)
	}

	entry := m.c.Entry(newEntryID)
	nextRun := time.Time{}
	if entry.ID != 0 {
		nextRun = entry.Next
	}

	m.tasks[newEntryID] = ScheduledTask{
		ID:          newEntryID,
		Name:        name,
		CronExpr:    cronExpr,
		Description: description,
		CreatedAt:   time.Now(),
		NextRun:     nextRun,
	}

	log.Printf("[调度] 新增任务: %s (ID:%d, Cron:%s, 下次:%v)", name, newEntryID, cronExpr, nextRun)
	return int(newEntryID), nil
}

// fireTask 触发任务：向 channel 放入消息
func (m *SchedulerManager) fireTask(entryID cron.EntryID, taskText string, extra map[string]any) {
	// 补充平台标识
	if extra == nil {
		extra = make(map[string]any)
	}

	msg := &message.UnifiedMessage{
		Platform:      message.PlatformQQ,
		PlatformMsgID: fmt.Sprintf("scheduler-%d-%d", entryID, time.Now().Unix()),
		Author: struct {
			PlatformUserID string
			Username       string
			AvatarURL      string
		}{
			PlatformUserID: "scheduler",
			Username:       "[定时任务]",
		},
		Timestamp: time.Now(),
		IsBot:     false,
		ContentBlocks: []*schema.ContentBlock{
			schema.NewContentBlock(&schema.UserInputText{Text: taskText}),
		},
		Extra: extra,
	}

	select {
	case message.MsgChan <- msg:
		log.Printf("[调度] 任务 %d 已触发，消息已推入 channel", entryID)
	default:
		log.Printf("[调度] 任务 %d 触发失败，channel 已满", entryID)
	}
}

// ListTasks 查询所有任务
func (m *SchedulerManager) ListTasks() []ScheduledTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]ScheduledTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		entry := m.c.Entry(t.ID)
		if entry.ID != 0 {
			t.NextRun = entry.Next
		}
		tasks = append(tasks, t)
	}
	return tasks
}

// RemoveTask 删除任务
func (m *SchedulerManager) RemoveTask(id int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	entryID := cron.EntryID(id)
	if _, ok := m.tasks[entryID]; !ok {
		return false
	}

	m.c.Remove(entryID)
	delete(m.tasks, entryID)
	log.Printf("[调度] 删除任务 ID:%d", id)
	return true
}

// invokeScheduleTask 实现 schedule_task 工具
func invokeScheduleTask(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Name        string `json:"name"`
		CronExpr    string `json:"cron_expr"`
		Description string `json:"description"`
		TaskText    string `json:"task_text"`
		ReplyTo     string `json:"reply_to"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}
	if args.CronExpr == "" {
		return "", fmt.Errorf("cron_expr 不能为空")
	}
	if args.TaskText == "" {
		return "", fmt.Errorf("task_text 不能为空")
	}
	if args.ReplyTo == "" {
		return "", fmt.Errorf("reply_to 不能为空")
	}

	// 解析 reply_to JSON 为 map，用于构建 Extra
	var extra map[string]any
	if err := json.Unmarshal([]byte(args.ReplyTo), &extra); err != nil {
		return "", fmt.Errorf("reply_to JSON 格式错误: %w", err)
	}

	if schedulerMgr == nil {
		return "", fmt.Errorf("调度器未初始化")
	}

	id, err := schedulerMgr.AddTask(args.Name, args.CronExpr, args.Description, args.TaskText, extra)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("定时任务创建成功，任务 ID: %d", id), nil
}

// invokeListTasks 实现 list_tasks 工具
func invokeListTasks(ctx context.Context, argsJSON string) (string, error) {
	if schedulerMgr == nil {
		return "", fmt.Errorf("调度器未初始化")
	}

	tasks := schedulerMgr.ListTasks()
	if len(tasks) == 0 {
		return "当前没有定时任务。", nil
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("当前共 %d 个定时任务：", len(tasks)))
	for _, t := range tasks {
		lines = append(lines, fmt.Sprintf("- [%d] %s | Cron: %s | 下次: %v | %s",
			t.ID, t.Name, t.CronExpr, t.NextRun.Format("2006-01-02 15:04:05"), t.Description))
	}
	return strings.Join(lines, "\n"), nil
}

// invokeDeleteTask 实现 delete_task 工具
func invokeDeleteTask(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}
	if args.ID == 0 {
		return "", fmt.Errorf("id 不能为空")
	}

	if schedulerMgr == nil {
		return "", fmt.Errorf("调度器未初始化")
	}

	if schedulerMgr.RemoveTask(args.ID) {
		return fmt.Sprintf("任务 ID %d 已删除。", args.ID), nil
	}
	return fmt.Sprintf("任务 ID %d 不存在。", args.ID), nil
}
