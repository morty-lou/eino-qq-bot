# QQ Bot — 基于 agenticark 的智能对话机器人

基于 [botgo](https://github.com/tencent-connect/botgo) QQ 机器人 + [eino agenticark](https://github.com/cloudwego/eino-ext) 豆包-seed LLM 的智能对话 bot，支持工具调用、群聊/私聊、多 worker 并发。

## 架构

```
用户消息 (WS)
    ↓
message/qqadapter.go       # QQ 事件适配器，构建 UnifiedMessage
    ↓
message/msg.go MsgChan      # 统一消息 channel
    ↓
worker.go                  # ReAct 循环消费消息，调用 agenticark LLM
    ↓
agent.go                   # agenticark Model 初始化
    ↓
tools/                     # 工具注册与执行
    ├── registry.go         # Invoke 分发 + GetToolInfos 注册
    ├── weather.go          # 天气查询
    ├── websearch.go        # web_search 工具定义
    ├── duckduckgo.go       # DuckDuckGo 免费搜索实现
    ├── scheduler.go        # 定时任务管理器
    └── scheduler_info.go   # 定时任务工具定义
    ↓
LLM 回复 → 回复用户
```

## 文件结构

```
qqbottest/
├── main.go              # 入口，init() 初始化所有组件
├── agent.go             # chatModel 全局变量 + InitAgent
├── worker.go            # ReAct 循环 + reply 函数
├── message/
│   ├── msg.go          # UnifiedMessage 定义 + Platform 常量 + MsgChan
│   └── qqadapter.go    # QQ 适配器：Init、注册 handler、下载图片、发送消息
└── tools/
    ├── registry.go      # 工具注册与 Invoke 分发
    ├── weather.go       # get_weather 工具
    ├── websearch.go    # web_search 工具定义
    ├── duckduckgo.go   # DuckDuckGo 搜索实现
    ├── scheduler.go    # 定时任务管理器 + 工具执行函数
    └── scheduler_info.go# 定时任务工具定义
```

## 环境变量

在项目根目录创建 `.env` 文件：

```env
# QQ 机器人
QQ_APPID=你的AppID
QQ_SECRET=你的Secret
QQ_SANDBOX=true          # true=沙箱环境，false=生产环境

# 豆包 LLM (agenticark)
ARK_API_KEY=你的ARK密钥
ARK_MODEL=doubao-seed-1-8-251228
```

## 工具

| 工具 | 说明 |
|------|------|
| `get_weather` | 查询城市天气，支持：北京/上海/广州/深圳/香港/东京 |
| `web_search` | DuckDuckGo 搜索获取实时信息（免费，无需 API Key） |
| `schedule_task` | 创建定时任务，到期自动执行任务并回复用户 |
| `list_tasks` | 查询所有已创建的定时任务 |
| `delete_task` | 删除指定的定时任务 |

### 定时任务格式

创建定时任务时 `reply_to` 参数格式：

```json
// 私聊回复
{"type":"c2c","user_id":"用户ID"}

// 群聊回复
{"type":"group","channel_id":"频道ID"}
```

## 启动

```bash
cd D:/code/qqbottest
go run main.go
```

## 工作流程

1. `main.go` init() 依次初始化：QQ 适配器（WS 连接）→ 工具 provider → Agent
2. QQ 消息通过 WebSocket 到达 adapter，构建 `UnifiedMessage` 推入 `MsgChan`
3. Worker 从 `MsgChan` 消费消息，构造 ReAct 循环：
   - 系统提示注入当前时间，消息头注入平台/用户元信息
   - LLM Generate → 检测工具调用 → 执行工具 → 追加结果 → 继续 Generate
   - 无工具调用时提取回复文本，回复对应用户/群组
4. 定时任务触发时，scheduler 向 `MsgChan` 推入消息，流程同上
