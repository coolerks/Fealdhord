# 折栈 Fealdhord 项目总开发指令

> 版本：v2.0｜设计与动效升级 + 双视图阅读 + RSS 首图
>
> 本文件以原始 108 节开发指令为基础修改，保留 RSS、AI Provider、Agent、Tools、Skills、MCP、Wails、SQLite 和多 Agent 协作规则。新增规范已纳入原有章节及「五十五-A」至「五十五-E」。
>
> 冲突优先级：新增的列表／瀑布流／首图／Motion 细化要求优先于原文中较宽泛的“文章列表”“图片不得主导布局”“最终再做 UI Polish”等表达。UI 交互与视觉应在 Phase 1 开始同步实现。



你现在负责设计并实现一款正式的跨平台桌面应用：

# 折栈 Fealdhord

产品定位：

> Local first AI RSS Reader and Agent Workspace

折栈首先是一款高质量 RSS 阅读器，同时提供 AI 自动分类、内容理解、对话操作、Tools、Skills、MCP 等能力。

这个项目会长期开发，并且可能由多个不同 AI Agent 在不同时间接手或者并行参与。

因此：

1. 代码是项目资产。

2. 架构文档是项目资产。

3. UI 设计文档是项目资产。

4. 当前工作状态是项目资产。

5. 技术决策记录是项目资产。

6. Agent 交接信息也是项目资产。

任何重要信息都不能只存在于当前对话上下文。

所有重要信息必须进入仓库。

你需要直接开始实施项目，不要只输出规划。

每完成一个有意义的开发阶段，都要同步维护文档。

---

# 一、产品原则

## 1. RSS 是主体

即使完全没有配置 AI Provider，用户仍然必须能够完整使用。

阅读展示必须同时支持「列表模式」和「瀑布流模式」，并能从 RSS 内容提取文章首图；这三项属于 RSS 基础体验，不能依赖 AI Provider。

完整基础功能包括：

1. 添加 RSS。

2. 删除 RSS。

3. 编辑 RSS。

4. 刷新 RSS。

5. 阅读文章。

6. 管理未读状态。

7. 收藏文章。

8. 管理分类。

9. 搜索文章。

10. 导入 OPML。

11. 导出 OPML。

12. 使用 Reader Mode。

13. 使用深色和浅色主题。

AI 是增强能力，不能成为基础 RSS 功能的依赖。

---

# 二、目标平台

必须支持：

```text
Windows
macOS
Linux
```

桌面框架：

```text
Go
Wails 3
```

Go 基线：

```text
Go 1.25+
```

开始开发前检查当前 Wails 3 官方文档和本机环境。

使用：

```text
wails3 doctor
```

验证环境。

不得依赖只存在于某一开发机器上的私有工具链。

---

# 三、技术栈

后端：

```text
Go 1.25+
Wails 3 正式版
SQLite
SQLite FTS5
sqlc
goose
gofeed
goquery
Readability compatible Go library
robfig/cron
bluemonday
```

前端：

```text
React
TypeScript
Vite
Tailwind CSS
shadcn/ui
Lucide
Zustand
TanStack Query
TanStack Virtual
motion
```

AI：

```text
OpenAI Go SDK
Anthropic Go SDK
```

Agent：

```text
Agent Runtime
Tool Registry
Skill Registry
Context Builder
Permission Layer
Execution History
```

MCP：

```text
Official MCP Go SDK
github.com/modelcontextprotocol/go-sdk
```

如果某个库已经长期无人维护，实施前寻找维护状态更好的替代方案，并将选择理由写入 ADR。

---

# 四、不要过度依赖 SDK

OpenAI Go SDK 与 Anthropic Go SDK 可以使用。

但 AI Provider 层必须围绕折栈自己的协议抽象设计。

原因是折栈需要连接：

```text
OpenAI
Anthropic
OpenRouter
DeepSeek
Groq
各种 OpenAI Compatible Provider
公司内部 AI Gateway
用户自行部署的服务
其他兼容实现
```

因此：

```text
业务层
↓
Fealdhord Model Interface
↓
Protocol Adapter
↓
SDK 或 HTTP Client
```

禁止业务层直接依赖某家厂商的数据结构。

---

# 五、AI API 协议

折栈完整支持以下三种协议。

## 1. Chat Completions API

协议 ID：

```text
chat_completions
```

标准 Endpoint：

```text
POST /v1/chat/completions
```

这是产品 UI 中所说的：

```text
Completions API
```

在代码和技术文档中建议写为：

```text
Chat Completions
```

避免与旧 \`/v1/completions\` 混淆。

必须完整支持：

```text
messages
system
user
assistant
tool
tools
tool_choice
tool_calls
stream
usage
temperature
top_p
max tokens related parameters
stop
response format when provider supports it
parallel tool calls when provider supports it
provider specific compatible fields
```

必须支持：

### 非流式

```text
POST /v1/chat/completions
stream = false
```

### SSE Streaming

```text
POST /v1/chat/completions
stream = true
```

需要正确合并：

```text
content delta
tool call delta
arguments delta
finish reason
usage
```

Tool Calling 必须支持完整循环：

```text
User
↓
Assistant tool_calls
↓
Execute Tool
↓
role=tool
↓
Assistant
↓
Final Response
```

必须支持一次返回多个 \`tool_calls\`。

不得假设只有一个 Tool Call。

---

# 六、明确不支持旧 Completions API

不要实现：

```text
POST /v1/completions
```

不要为它实现兼容层。

不要通过 Prompt JSON 模拟工具调用。

不要在 UI 中提供该选项。

折栈中所说：

```text
Completions API
```

统一代表：

```text
/v1/chat/completions
```

---

# 七、Responses API

协议 ID：

```text
responses
```

标准 Endpoint：

```text
POST /v1/responses
```

完整支持：

```text
input
instructions
tools
tool calls
function calls
function outputs
streaming
usage
reasoning related events when available
structured output when available
multi turn context
```

需要实现独立：

```text
ResponsesAdapter
```

不能通过 Chat Completions Adapter 模拟 Responses。

Streaming Event 必须转换成折栈统一事件模型。

---

# 八、Messages API

协议 ID：

```text
messages
```

标准 Endpoint：

```text
POST /v1/messages
```

面向 Anthropic Compatible 服务。

必须支持：

```text
system
messages
content blocks
text
tool_use
tool_result
streaming
usage
thinking related content when supported
stop reason
```

Tool Loop：

```text
User
↓
Assistant tool_use
↓
Execute Tool
↓
User tool_result
↓
Assistant
```

实现：

```text
MessagesAdapter
```

---

# 九、统一 Model Interface

业务层只面向自己的接口。

建议：

```go
type Model interface {
    Generate(
        ctx context.Context,
        req *ModelRequest,
    ) (*ModelResponse, error)
    Stream(
        ctx context.Context,
        req *ModelRequest,
    ) (<-chan ModelEvent, error)
}
```

统一请求模型至少包括：

```text
ModelRequest
System
Messages
Tools
ToolChoice
Temperature
TopP
MaxOutputTokens
Stop
ResponseFormat
Metadata
ProviderOptions
```

统一 Message：

```text
Message
Role
Content
ToolCalls
ToolCallID
Metadata
```

---

# 十、统一 Streaming Event

不同 Provider 的 Streaming 格式不同。

内部必须转换成统一模型。

例如：

```text
ModelEvent
```

事件类型至少包括：

```text
message_start
text_delta
reasoning_delta
tool_call_start
tool_call_arguments_delta
tool_call_end
usage
message_end
error
```

前端 AI Chat 只能依赖 \`ModelEvent\`。

前端禁止处理 OpenAI 或 Anthropic 原始 SSE 格式。

---

# 十一、Provider 配置系统

用户可以添加任意多个 AI 服务。

Provider 配置：

```text
id
name
protocol
base_url
credential_ref
enabled
created_at
updated_at
```

协议：

```text
chat_completions
responses
messages
```

UI：

```text
添加 AI 服务
名称
My Provider
API 类型
○ Chat Completions
○ Responses
○ Messages
Base URL
https\://example.com/v1
API Key
••••••••••••••
[测试连接]
```

---

# 十二、兼容第三方 Provider

由于很多服务只是 OpenAI Compatible，配置需要提供高级选项：

```text
Custom Headers
Custom Endpoint Path
Query Parameters
Request Extra Body
Request Timeout
Disable unsupported parameters
```

这些高级设置默认折叠。

禁止要求普通用户理解所有字段。

---

# 十三、Model 管理

Provider 与 Model 分离。

数据库：

```text
ai_providers
ai_models
```

Model：

```text
id
provider_id
model
display_name
capabilities
enabled
created_at
updated_at
```

Capabilities 建议包括：

```text
streaming
tools
parallel_tools
structured_output
reasoning
vision
```

允许：

```text
自动检测
+
用户手动覆盖
```

因为很多 Compatible Provider 的能力声明并不可靠。

---

# 十四、任务可以使用不同模型

不要设置成整个应用只能使用一个模型。

支持：

```text
默认聊天模型
自动分类模型
摘要模型
翻译模型
精选模型
```

例如：

```text
聊天
GPT 系列
自动分类
低成本模型
长文总结
Claude
翻译
其他兼容模型
```

---

# 十五、Agent Runtime

建立独立模块：

```text
Agent Runtime
```

职责：

```text
Context 构建
Model 调用
Tool 注册
Tool 调用
权限判断
Tool Result 回传
多轮 Tool Loop
Skill 注入
MCP Tool 集成
Execution Log
Cancellation
Timeout
Error Recovery
```

结构：

```text
User
↓
Context Builder
↓
Skill Resolver
↓
Tool Resolver
↓
Model
↓
Tool Call
↓
Permission Layer
↓
Tool Executor
↓
Tool Result
↓
Model
↓
Final Answer
```

---

# 十六、Agent 最大循环限制

防止模型无限调用工具。

每次 Agent Run 配置：

```text
max_turns
max_tool_calls
timeout
```

提供合理默认值。

例如：

```text
max_turns = 16
```

超过限制后：

```text
安全终止
保存 Execution History
向 UI 返回明确错误
```

---

# 十七、Tool 是核心能力抽象

建立：

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() json.RawMessage
    Permission() ToolPermission
    Execute(
        ctx context.Context,
        input json.RawMessage,
    ) (*ToolResult, error)
}
```

所有 Tool 都进入：

```text
ToolRegistry
```

来源可以是：

```text
Built in Tool
MCP Tool
未来 Plugin Tool
```

模型不需要知道底层来源。

---

# 十八、Built in Tools

至少规划以下工具。

## Feed

```text
feed.list
feed.get
feed.add
feed.update
feed.remove
feed.refresh
feed.refresh_all
feed.discover
```

## Category

```text
category.list
category.get
category.create
category.update
category.remove
category.add_feed
category.remove_feed
```

## Article

```text
article.list
article.get
article.search
article.mark_read
article.mark_unread
article.favorite
article.unfavorite
article.add_category
article.remove_category
```

## Application

```text
app.get_settings
```

敏感设置修改不能随意开放给 Agent。

---

# 十九、Tool 权限系统

Tool 至少划分：

```text
read
write
destructive
external
```

行为：

### read

例如：

```text
article.search
feed.list
article.get
```

默认允许自动执行。

### write

例如：

```text
article.favorite
article.mark_read
```

允许用户配置：

```text
自动执行
每次询问
禁止
```

### destructive

例如：

```text
feed.remove
category.remove
```

默认必须用户确认。

### external

例如 MCP 对外部系统进行写操作。

默认依据 MCP Server 和具体 Tool 权限配置。

---

# 二十、危险操作确认

例如用户说：

```text
把半年没有更新的订阅全部删掉
```

Agent 可以：

```text
feed.list
```

计算结果。

但在执行删除前必须产生：

```text
Pending Tool Action
```

UI 显示：

```text
AI 准备删除 17 个订阅
Feed A
Feed B
Feed C
...
取消
确认删除
```

用户确认后才调用：

```text
feed.remove
```

---

# 二十一、Tool Execution History

记录：

```text
agent_runs
tool_calls
```

Tool Call 保存：

```text
id
agent_run_id
tool_name
tool_source
input
result
status
permission
started_at
finished_at
error
```

Chat UI 可以展示：

```text
使用了 4 个工具
✓ 搜索文章
✓ 查询分类
✓ 收藏文章
✓ 更新已读状态
```

---

# 二十二、MCP

折栈同时支持：

```text
MCP Client
MCP Server
```

使用官方 Go SDK。

---

# 二十三、MCP Client

允许用户添加 MCP Server。

至少考虑：

```text
stdio
Streamable HTTP
```

具体 Transport 根据当前 MCP 官方规范实现。

MCP 配置：

```text
id
name
transport
command
args
url
env
enabled
permission_policy
```

敏感 Environment Variable 不明文进入普通配置文件。

---

# 二十四、MCP Tool Integration

MCP Tool 加载后进入统一：

```text
ToolRegistry
```

命名建议：

```text
mcp.\<server>.\<tool>
```

例如：

```text
mcp.github.search_repositories
mcp.notion.search
```

需要避免不同 Server Tool 名冲突。

---

# 二十五、MCP 权限

配置页面显示每个 MCP Server 提供：

```text
Tools
Resources
Prompts
```

每个 Tool 可以设置：

```text
自动
询问
禁止
```

外部写操作默认：

```text
询问
```

---

# 二十六、MCP Server

折栈自身也需要规划 MCP Server。

通过 MCP 暴露安全的折栈能力。

例如：

```text
search_articles
get_article
list_feeds
list_categories
favorite_article
mark_article_read
```

Resources 可以规划：

```text
Fealdhord://article/{id}
Fealdhord://feed/{id}
Fealdhord://category/{id}
```

第一阶段如果 MCP Server 尚未实现，也必须把架构和接口设计保留清楚。

---

# 二十七、Skill 系统

Skill 与 Tool 分开。

定义：

```text
Tool
\=
原子操作能力
```

```text
Skill
\=
指导 Agent 如何组合 Context 与 Tools 完成某类任务
```

例如：

```text
每日技术简报
订阅整理
深度阅读
文章翻译
```

---

# 二十八、Skill 目录

支持：

```text
skills/
```

一个 Skill：

```text
skills/
└── daily-tech-brief/
    ├── SKILL.md
    └── assets/
```

SKILL.md：

```markdown
---
name: daily-tech-brief
title: 每日技术简报
description: 根据最近 RSS 内容生成技术简报
version: 1
tools:
  \- article.search
  \- article.get
---
# Instructions
读取过去 24 小时的文章。
重点关注：
1. Go
2. AI
3. Developer Tools
降低营销、融资和重复新闻权重。
最多选择十篇。
```

---

# 二十九、Skill 安全

第一版 Skill 只允许：

```text
Prompt
Metadata
Tool allowlist
Assets
```

不要允许 Skill 任意执行：

```text
shell
JavaScript
Python
binary
```

未来需要代码 Skill 时必须设计独立 Sandbox。

---

# 三十、Skill UI

设置区域提供：

```text
Skills
```

支持：

```text
查看
创建
编辑
启用
禁用
删除
```

展示：

```text
名称
描述
允许工具
更新时间
来源
```

---

# 三十一、AI Chat

聊天不能成为整个应用唯一入口。

提供三种主要 Context。

### Global Chat

可以访问整个折栈。

### Category Chat

自动包含当前分类 Context。

### Article Chat

自动包含：

```text
当前文章
当前选中文字
当前 Feed
```

---

# 三十二、Reader AI Panel

阅读文章时允许右侧打开 AI Panel。

示例：

```text
✨ AI
总结这篇文章
解释选中的内容
翻译全文
找出类似文章
这篇内容为什么值得读？
把它加入 Go 分类
收藏这篇文章
```

最后两种属于 Tool 操作。

---

# 三十三、Context Builder

不要把整个数据库发送给模型。

统一建立：

```text
ContextBuilder
```

上下文来源：

```text
System Prompt
Conversation
Current Article
Current Category
Selected Text
Search Results
Skill Instructions
Tool Definitions
User Preferences
```

根据请求选择最少必要 Context。

---

# 三十四、RSS 数据结构

至少规划：

```text
feeds
articles
categories
feed_categories
article_categories
reading_states
favorites
```

建议最终由 sqlc 管理查询。

---

# 三十五、Feed

保存：

```text
id
title
url
site_url
description
icon_url
etag
last_modified
last_checked_at
last_success_at
next_check_at
error_count
last_error
created_at
updated_at
```

---

# 三十六、Article

保存：

```text
id
feed_id
guid
title
url
author
summary
content_html
content_text
published_at
updated_at
fetched_at
content_hash
cover_image_url
cover_image_source
cover_image_status
cover_image_width
cover_image_height
cover_image_alt
```\`

首图元数据应根据实际需求决定保存在 articles 字段或独立 article_media 表；不要为每篇文章强制下载图片原图。原始文章中的图片应按来源与优先级解析，详情见「五十五-B」。

需要可靠处理重复文章。

不能只依赖 URL。

---

# 三十七、RSS 格式

必须支持：

```text
RSS
Atom
JSON Feed
```

优先使用成熟 Parser。

---

# 三十八、Feed Discovery

用户输入：

```text
https\://example.com
```

应用可以尝试：

```text
\<link rel="alternate">
```

寻找：

```text
RSS
Atom
```

并允许用户选择发现到的 Feed。

---

# 三十九、Feed 更新机制

支持：

```text
ETag
Last Modified
If None Match
If Modified Since
304 Not Modified
```

更新状态：

```text
last_checked_at
last_success_at
next_check_at
error_count
last_error
```

调度策略可以根据 Feed 活跃程度动态调整。

---

# 四十、自动分类

Category 包含：

```text
id
name
icon
color
description
ai_enabled
ai_prompt
ai_threshold
created_at
updated_at
```

Feed 可以设置默认 Category。

Article 可以属于多个 Category。

---

# 四十一、分类来源

\`article_categories\` 至少包含：

```text
article_id
category_id
source
state
confidence
reason
model
created_at
```

source：

```text
manual
feed
ai
```

state：

```text
included
excluded
```

用户手动操作优先级最高。

---

# 四十二、Manual Override

如果 AI 加入：

```text
Article → AI Category
```

用户随后手动移除。

记录：

```text
source = manual
state = excluded
```

以后重新执行 AI 分类时不能再次自动添加。

---

# 四十三、AI Classification

一次请求尽可能同时判断多个分类。

避免：

```text
文章数量 × Category 数量
```

的模型调用模式。

模型返回：

```json
{
  "categories": [
    {
      "id": "xxx",
      "confidence": 0.94,
      "reason": "文章主要讨论 Go 的并发设计"
    }
  ],
  "importance": 0.87,
  "relevance": 0.91
}
```

---

# 四十四、AI Classification Cache

缓存判断至少考虑：

```text
article_content_hash
category_prompt_hash
provider
model
classification_version
```

没有变化时禁止重复调用模型。

---

# 四十五、精选

精选是：

```text
Smart View
```

不要设计成普通 Category。

依据：

```text
importance
relevance
user preference
featured prompt
```

进行筛选。

---

# 四十六、Prompt 测试

AI Category 设置页面提供：

```text
测试 Prompt
```

从最近文章选择样本。

显示：

```text
匹配
confidence
reason
```

例如：

```text
✓ Building Agents in Go      94%
✓ New LLM Architecture       89%
✕ Startup Funding News       21%
```

---

# 四十七、搜索

SQLite 开启：

```text
FTS5
```

至少索引：

```text
title
author
summary
content_text
```

第一阶段优先实现高质量全文搜索。

不要因为 AI 存在就忽略传统搜索。

---

# 四十八、Reader

支持：

```text
RSS Content
Reader Mode
Original Web Page
```

文章正文内原始插图与「列表／瀑布流的首图」分别处理：前者以阅读排版、安全清理和原文忠实度优先，后者以快速预览、加载效率和清晰裁剪优先。禁止为了制作卡片封面而修改或破坏原文。

HTML 必须 sanitize。

禁止 RSS Content 执行：

```text
script
event handler
危险 iframe
```

---

# 四十九、阅读体验

正文宽度建议：

```text
680px 到 760px
```

支持设置：

```text
Font Size
Line Height
Reader Width
Font Family
```

正文排版比装饰效果更重要。

---

# 五十、主界面

宽屏使用三栏：

```text
┌────────────────┬──────────────────────┬───────────────────────────────┐
│                │                      │                               │
│    Sidebar     │     Article List     │            Reader             │
│                │                      │                               │
└────────────────┴──────────────────────┴───────────────────────────────┘
```

Sidebar：

```text
220px 到 280px
```

Article List：

```text
300px 到 420px
```

Reader：

```text
剩余区域
```

支持拖动调整区域宽度。

文章中栏提供「列表／瀑布流」切换。宽屏时瀑布流可在中栏扩展或进入更宽的浏览视图；打开文章可展示阅读抽屉，或切回三栏阅读。两种模式必须共用相同的 Feed、Category、过滤器、排序、阅读状态和搜索结果。

保存布局、视图模式、阅读面板状态与各视图的浏览位置。

---

# 五十一、Sidebar 毛玻璃效果

侧栏必须具有明确的 Frosted Glass 视觉。

这是正式设计要求，不是可选效果。

视觉组成：

```text
透明背景
Backdrop Blur
低对比边框
非常轻微的高光
背景层次
```

Tailwind 方向可以参考：

```tsx
bg-background/65
backdrop-blur-xl
supports-[backdrop-filter]:bg-background/55
border-r
border-border/50
```

具体数值需要通过实际视觉测试调整。

要求：

1. Light Mode 能明显看到玻璃层次。

2. Dark Mode 仍然保持玻璃质感。

3. 文字对比度必须足够。

4. Sidebar 不能像一个普通 Card。

5. 避免夸张阴影。

6. 避免高饱和渐变。

7. 不支持 backdrop filter 时提供正常实色 fallback。

8. Windows、macOS、Linux 都要验证视觉。

9. 不依赖 macOS 私有材质才能成立。

---

# 五十二、视觉风格

方向参考：

```text
Linear
Raycast
Reeder
现代 macOS 桌面应用
```

只能作为设计方向。

禁止像素级复制任何现有产品。

整体关键词：

```text
克制
轻
高信息密度
清晰
现代
安静
桌面感
有生命力的交互
```

UI 视觉语言以 Linear、Raycast、Reeder 和现代桌面应用的精致排版为参考。Telegram 只作为「微交互动效的丰富度、反馈及时性、动画连贯性」的参照，绝不模仿 Telegram 的蓝白配色、聊天气泡、聊天列表密度、导航组织、图标风格或整体界面。

避免：

```text
大面积 Hero
营销网站风格
Dashboard 卡片堆积
粗重边框
巨大圆角
大量 Gradient
```

---

# 五十三、Theme

支持：

```text
Light
Dark
System
```

主题必须基于 Design Tokens。

CSS Variables 至少包含：

```text
background
foreground
card
popover
muted
muted-foreground
border
input
accent
accent-foreground
primary
primary-foreground
destructive
sidebar
sidebar-foreground
sidebar-accent
reader-background
```

不要在组件中大量硬编码 Tailwind 色值。

---

# 五十四、Sidebar 信息架构

系统区域：

```text
全部
精选
未读
收藏
```

分类：

```text
分类
AI
Go
开发工具
设计
...
```

订阅：

```text
订阅
Hacker News
OpenAI
Anthropic
...
```

底部：

```text
AI
设置
```

使用 Lucide Icon。

支持：

```text
展开
折叠
Context Menu
Drag and Drop
Keyboard Navigation
Unread Count
```

---

# 五十五、Article List

支持两种 Density：

```text
Compact
Comfortable
```

列表至少展示：

```text
Title
Feed
Published Time
Unread
Favorite
AI indicator
```

列表模式采用文字优先布局；可根据设置显示小缩略图，但图片不得主导列表。

瀑布流模式采用图文卡片，优先展示 RSS 提取的首图，允许不同图片比例形成自然错落的视觉节奏；缺少首图时使用纯排版卡片，不使用夸张的随机占位图片。

文章数量很多时使用：

```text
TanStack Virtual
```\`

列表和瀑布流都要限制可见 DOM 数量。瀑布流需要在动态图片高度、列数变化、窗口缩放和无限滚动时保持稳定定位；不得依赖简单 CSS columns 导致的视觉顺序与键盘顺序冲突。

避免渲染全部 DOM。

---

# 五十五-A、双阅读视图：列表与瀑布流

**两种视图都是正式的一等功能，不是占位演示。**

## 1. 列表模式（List View）

- 适合高信息密度、快速扫描、键盘阅读。
- 保留 Compact / Comfortable 两种密度。
- 行内容至少包含标题、订阅源、发布时间、未读状态、收藏状态、AI 标记。
- 支持用户配置「不显示图片／小缩略图」。缩略图统一限定尺寸和裁剪规则，不打乱行高。
- Hover 时出现有意义的快捷操作，键盘 Focus 时也能访问同样功能。
- 行选中、已读状态、收藏状态与阅读面板的变化要有自然过渡。

## 2. 瀑布流模式（Masonry View）

- 以文章为卡片，顶部优先展示提取到的 RSS 首图；卡片随后展示标题、来源、发布时间、节选与必要状态。
- 不强制所有卡片等高；图片可保留合理的原始宽高比，卡片高度由实际内容决定。
- 同时限制图片最大展示高度、标题行数与摘要长度，避免单篇文章异常占据整屏。
- 无图文章使用整洁的文字卡片；不要使用随机图库、重复的品牌大图或假封面。
- 应有清晰的 Hover、Focus、Selected、Unread、Favorite 状态。
- 用户可选按时间排序，视觉排布不能导致阅读顺序混乱；键盘导航及辅助技术读取应遵循可预测的逻辑顺序。
- 允许在瀑布流直接打开 Reader；可以使用阅读抽屉、详情面板或回到三栏阅读，过渡需连贯。
- 随窗口和容器宽度自适应列数。可参考每列最小宽度 240px 至 300px，具体值由真实视觉测试确定；不要硬编码只支持某个显示器。
- 窄容器自动降低列数。不要为了保持多列而产生横向滚动。
- 对大量文章必须使用支持动态测量高度的虚拟化策略。测量用 ResizeObserver 或等效机制；切换过滤和窗口尺寸后不得出现卡片重叠、跳跃和大面积空洞。

## 3. 统一数据与视图状态

```text
articleViewMode = list | masonry
listDensity = compact | comfortable
listThumbnail = none | small
masonryImageMode = original_ratio | cropped
```

- 视图切换入口放在文章工具栏，使用易辨识的 List / LayoutGrid 类图标及 Tooltip。
- 不允许分别维护两套文章数据、已读逻辑和过滤查询；不同视图共享 Query 数据源。
- 记住视图模式，重启应用后恢复。
- 切换模式时尽可能保持当前选中文章、筛选条件和浏览锚点，避免每次都跳回文章顶部。
- 支持鼠标、触控板和键盘操作；滚动不得被装饰性动画阻断。
- 数据不足时，使用真实的空状态和引导；不要用模拟内容伪装产品已具备的功能。

---

# 五十五-B、RSS 首图提取与媒体管理

文章首图是完整 RSS 体验的一部分，不依赖 AI、付费 Provider 或云端识别。

## 1. 提取优先级

按以下顺序尝试，并允许根据真实 RSS 格式做适配：

1. 明确声明的图片媒体字段，例如 `media:content`、`media:thumbnail`、符合图片类型的 `enclosure`，以及 JSON Feed 的相关图片字段。
2. RSS / Atom 条目的正文 `content:encoded`、`content` 中的有效 `<img>`；必要时检查 `srcset`、`data-src` 等常见延迟加载字段。
3. 条目摘要 `description`、`summary` 内的有效图片。
4. 文章原网页的 Open Graph `og:image` 或 `twitter:image`；这一步属于可选的额外网络请求，需受用户设置、站点策略、请求频率和超时控制。
5. 均未找到时使用无图卡片，并可以显示站点 favicon 或文字来源标识；**favicon 不能冒充文章首图**。

多张候选图片应做基本筛选，尽量排除追踪像素、极小图片、常见头像和明显的装饰性图标。不要把第一张任意 `<img>` 无条件认定为封面。

## 2. 数据与处理边界

- 在采集层完成候选提取和 URL 规范化，React 只消费规范化后的图片元数据。
- 记录 `cover_image_url`、`cover_image_source`、`cover_image_status`、可用的宽高和替代文字；可采用单表或独立媒体表，关键是迁移可追踪。
- 根据条目与站点基准 URL 正确解析相对路径；拒绝 `javascript:`、`data:` 等不适合远程首图的 URL scheme。
- 采用异步图片探测，避免阻塞 Feed 更新主流程。
- 优先保留原始图片地址与元数据；本地缩略图缓存是可选优化，必须有容量限制、过期/清理策略。
- 禁止将远程图片 URL、站点认证信息或用户隐私数据交给 AI 处理作为默认流程。
- 遵守站点访问策略和请求限制，避免因首图发现产生大量额外访问。

## 3. 安全与隐私

- 抓取远端页面或图片的 Go HTTP Client 必须对 SSRF 做防护：限制协议、重定向、私网与本地地址访问，必要时防范 DNS 解析变化。
- 不把外站 HTML 或 SVG 作为可信可执行内容注入界面；原文和图片地址分别清理。
- 允许用户关闭外部图片加载，或选择仅对已订阅源加载图片。
- 对超时、404、403、无效图片、超大文件、格式不支持等情况降级为无图，不能造成文章列表崩溃。

## 4. 展示与加载

- 使用原始宽高比生成占位区域，减少图片加载造成的布局位移（CLS）。
- 使用 `loading="lazy"`、异步解码或等效策略，优先加载视口附近图片。
- 优先缩略图尺寸而非每次下载原图；必要时使用有界缓存。
- 图片进入视口时可做很轻的透明度过渡，不能对每次滚动触发重复渐变。
- 默认禁止自动播放图片中的视频或音频。
- 卡片首图需要适当的替代文本或装饰图标语义，不得为了视觉效果丢失标题可访问性。

---

# 五十五-C、统一 Motion Design System

**折栈必须具有丰富、细腻且与操作含义匹配的动画。** 参考 Telegram 的交互反馈密度与流畅感，但**不使用 Telegram 的视觉设计**。

设计关键词：`Fold`、`Stack`、`Reveal`、`Glide`、`Settle`。核心动态语言是折叠、轻微错层、展开和归位，与「折栈」品牌对应。

## 1. 动画层级

```text
Tier 1  Micro Interaction  120–200ms  Hover / Focus / Press / Icon response
Tier 2  State Transition   180–280ms  Selected / Read / Favorite / Toggle
Tier 3  Surface Transition 220–380ms  Sidebar / Menu / Panel / Dialog
Tier 4  Content Transition 240–450ms  View switch / Masonry reflow / Reader
```

时长为默认设计范围，不作为不可修改的魔法数字。通过真实交互调整缓动与 Spring 参数。

- Hover 动画触发及时且能反向恢复，不允许鼠标快速掠过时留下半完成状态。
- 使用物理感适中的 Spring 动画处理图标微弹、卡片位移、列表选择和面板展开；避免强烈弹跳与反复震荡。
- 同一操作涉及多个元素时可做轻微 stagger，但列表和瀑布流大批量出现时禁止逐条长时间排队播放。
- 短时间重复操作应可打断或衔接当前动画，不得出现闪烁、瞬移或重复动作排队。
- 只有加载、同步、Agent 工作中等真实进行的状态允许持续动画；空闲时不得永久旋转图标或持续呼吸闪烁。

## 2. 实现方式

- 使用既定前端技术栈里的 `motion` 处理复杂编排和布局过渡。
- 简单 Hover / Focus / Press 优先使用 CSS transition 与 transform，避免给每个列表项创建额外 JS 动画实例。
- 图标可基于 Lucide SVG 做局部旋转、位移、路径描画或可读的形态变化，但不得在组件树中大量复制整套 SVG 动画逻辑。
- 建立统一 `Motion Tokens`，如 `duration.fast`、`duration.normal`、`duration.panel`、`ease.standard`、`spring.gentle`、`spring.snappy`，禁止每个组件随意写不一致的参数。
- 动画优先使用 `transform` 和 `opacity`；避免大范围持续修改 `width`、`height`、`top`、`left`、`box-shadow` 或高成本 `backdrop-filter` 属性。
- Wails 使用的 Windows WebView2、macOS WebKit、Linux WebKitGTK 都必须检查动效表现和兼容性。

## 3. 不允许的设计

- 直接复制 Telegram 聊天界面和配色。
- 所有图标统一套用同一个弹跳、缩放或旋转动画。
- 大范围滚动视差、长时间模糊、夸张发光、粒子飞舞、强烈渐变。
- 文章正文因阅读滚动频繁触发入场动画。
- 让动画持续阻塞点击、键盘切换、搜索输入或阅读操作。
- 为了达到动效数量而增加没有操作意义的动画。

---

# 五十五-D、Hover 与图标语义动效矩阵

所有可点击的图标按钮至少有 Hover、Press、Focus-visible 和 Disabled 状态。**有明确语义的操作必须提供差异化动画**，不能全部做统一放大。

| 场景 | Hover 反馈 | 触发操作后的反馈 |
| --- | --- | --- |
| 侧栏导航项目 | 背景柔和浮现，图标轻微向文本方向位移 | 选中指示在相邻项目间平滑移动 |
| 分类文件夹 | 文件夹外形或展开箭头轻微开启 | 子项展开与高度过渡，保留焦点 |
| 收藏 / 取消收藏 | 星形轻微倾斜或弹动 | 星形填充与轻微 Spring 回弹，状态可撤销 |
| 已读 / 未读 | 状态圆点或信封图标局部变化 | 圆点消退或恢复，文字权重自然变化 |
| 手动刷新 | 环形箭头微动，提示可刷新 | 请求期间旋转，成功结束并归位；失败明确提示 |
| 搜索 | 放大镜短距离位移或局部描画 | 搜索框展开，焦点平滑进入 |
| 设置 | 齿轮有限角度轻微旋转 | 设置面板/页面自然进入 |
| 列表 / 瀑布流切换 | 对应布局图标做短暂排列反馈 | 卡片在两种排布中平滑重组 |
| 文章卡片 | 低幅抬升或表面明暗变化，图片轻微缩放 | Reader 打开并保留上下文 |
| 更多菜单 | 三点轻微移动或变色 | 菜单从图标附近展开，结束时返回触发点 |
| Sidebar 折叠 | 箭头方向与位移提示操作 | 导航宽度和文字渐隐同步过渡 |
| AI 总结 | Sparkles 轻微闪动一次 | 流式生成状态有克制的活动指示 |
| AI Send / Cancel | 发送箭头轻微上移 | 点击后短促反馈，发送和取消状态转换 |
| Tool 执行 | 图标展示正在工作的状态 | 成功、失败、待用户许可分别显示清晰反馈 |
| Toast / Undo | 进入时轻微滑入和淡入 | 撤销后直接恢复对应真实数据状态 |
| Drag & Drop | 可拖动行出现把手或轻微抓取提示 | 被拖动元素浮起，目标区域显示位置指示 |

补充要求：

- Hover 必须绑定真实可交互区域，避免用户在图标附近反复移入移出导致动画闪烁。
- 图标建议在图标容器内部运动，触控区域始终足够大，动画不影响点击热区。
- Tooltip 必须在适当延迟后显示，动画只作轻微淡入和位移。
- Focus-visible 与键盘操作应提供同样清晰的状态提示；禁止只用 Hover 呈现关键操作。
- 触控和无 Hover 环境保证功能可用。触控场景不要伪造鼠标 Hover。

---

# 五十五-E、动画、图片与瀑布流的性能和测试

## 1. 用户设置

在 `Settings > Appearance` 中允许设置：

```text
Animation = Full | Subtle | Off
Follow System Reduce Motion = true | false
Default Article View = List | Masonry
List Density = Compact | Comfortable
List Thumbnail = None | Small
Masonry Image Ratio = Original | Cropped
Load Remote Images = Always | Subscriptions Only | Never
```

系统开启 `prefers-reduced-motion` 时默认遵从系统；动画关闭后必须保留功能和状态变化，不得留下不可操作的遮罩。

## 2. 性能目标

- 在常见桌面设备上以 60fps 为交互体验目标；连续动画避免显著卡顿。
- 大批量文章时虚拟化和按需图片加载必须正常工作。
- 滚动期间不要无差别触发所有可见卡片的 Hover、Reveal 或重排动画。
- 远程图片不可达、网络断开、RSS 数据不完整时，列表和瀑布流仍可使用。
- 图片异步到达时不要大幅移动视口中的阅读锚点。
- 使用真实测试数据至少模拟 1,000、10,000、100,000 篇文章时的搜索、切换视图、滚动、筛选和窗口缩放。
- 验证高 DPI、多显示器、窗口最小尺寸、浅色与深色主题。

## 3. 必须覆盖的测试

```text
Feed 首图来源优先级与相对 URL 解析
media:content / media:thumbnail / enclosure
正文 img / srcset / 延迟加载字段
无首图 / 404 / 超时 / 非法格式
禁止访问本地或私网地址的图片/页面请求
图片加载导致的布局位移
List ⇄ Masonry 切换与状态保持
瀑布流动态高度与虚拟滚动
Hover / Focus / Press / Selected / Disabled
动画中途快速重复点击
Reduced Motion / 动画关闭
跨平台 WebView 动效
Reader 打开与关闭后的视图恢复
```

UI 截图只能证明静态外观；关键动画需通过组件测试、自动化交互或屏幕录制验证真实执行情况。

---

# 五十六、状态管理

不要把所有数据塞入 Zustand。

Zustand 负责 UI State：

```text
selected category
selected article
article view mode (list / masonry)
list density and thumbnails
scroll anchor / transient layout state
sidebar collapsed state
layout sizes
reader settings
temporary UI state
theme
```

TanStack Query 管理：

```text
feeds
articles
categories
search result
unread counts
async Wails calls
```

SQLite 是真实持久化数据源。

---

# 五十七、Wails Boundary

React 不直接访问数据库。

React 不直接解析 RSS。

React 不直接管理 API Key。

结构：

```text
React
↓
Wails Bindings
↓
Application Services
↓
Repository / Runtime
↓
SQLite / Network / AI
```

---

# 五十八、Go Backend Modules

建议：

```text
internal/
├── app/
├── feed/
├── article/
├── category/
├── search/
├── reader/
├── database/
├── ai/
├── agent/
├── tools/
├── skills/
├── mcp/
├── settings/
├── credentials/
└── scheduler/
```

实际结构可以根据 Go Package 设计调整。

但职责必须清晰。

---

# 五十九、数据库 Migration

使用：

```text
goose
```

Migration 必须进入版本控制。

禁止应用启动时随意执行不可追踪的 Schema 改动。

---

# 六十、Secret Storage

API Key 不进入普通 SQLite 字段。

优先使用：

```text
Windows Credential Manager
macOS Keychain
Linux Secret Service
```

建立：

```text
CredentialStore
```

接口。

数据库只记录：

```text
credential_ref
```

需要提供开发环境 fallback，但生产版不得默认明文存储 Secret。

---

# 六十一、OPML

必须规划：

```text
Import
Export
```

Import 时：

```text
解析 Folder
映射 Category
检测重复 Feed
显示导入结果
```

Export 保留 Category 结构。

---

# 六十二、错误体验

不要把 Go Error 原样扔给用户。

内部 Error 分类至少考虑：

```text
NetworkError
FeedParseError
DatabaseError
ProviderError
AuthenticationError
RateLimitError
ToolError
MCPError
PermissionError
ConfigurationError
```

UI 展示用户能够理解的信息。

日志保留技术细节。

---

# 六十三、Cancellation

以下操作需要支持取消：

```text
AI streaming
Agent run
Feed refresh
MCP request
长时间全文抓取
```

Go 使用：

```text
context.Context
```

贯穿调用链。

---

# 六十四、Logs

建立结构化 Logging。

至少区分：

```text
debug
info
warn
error
```

敏感信息禁止进入日志：

```text
API Key
Authorization Header
Credential
完整用户 Secret
```

---

# 六十五、多 AI 协作文档体系

必须创建：

```text
docs/
```

并按照下面的结构维护。

```text
docs/
├── README.md
│
├── 00-product/
│   ├── vision.md
│   ├── scope.md
│   ├── terminology.md
│   └── product-principles.md
│
├── 01-status/
│   ├── CURRENT.md
│   ├── ROADMAP.md
│   ├── BACKLOG.md
│   ├── RISKS.md
│   ├── COMPLETED.md
│   └── active/
│
├── 02-design/
│   ├── design-system.md
│   ├── app-shell.md
│   ├── sidebar.md
│   ├── article-list.md
│   ├── reader.md
│   ├── ai-chat.md
│   ├── settings.md
│   ├── interaction.md
│   ├── motion-system.md
│   ├── article-views.md
│   └── article-media.md
│
├── 03-architecture/
│   ├── overview.md
│   ├── frontend.md
│   ├── backend.md
│   ├── database.md
│   ├── ai-provider.md
│   ├── agent-runtime.md
│   ├── tools.md
│   ├── skills.md
│   ├── mcp.md
│   ├── security.md
│   └── data-flow.md
│
├── 04-specs/
│   ├── rss.md
│   ├── categories.md
│   ├── ai-classification.md
│   ├── smart-views.md
│   ├── search.md
│   ├── chat.md
│   ├── providers.md
│   ├── tools.md
│   ├── skills.md
│   ├── mcp.md
│   └── opml.md
│
├── 05-api/
│   ├── wails-bindings.md
│   ├── model-contract.md
│   ├── tool-contract.md
│   └── events.md
│
├── 06-decisions/
│   └── ADR-XXXX-*.md
│
├── 07-quality/
│   ├── testing-strategy.md
│   ├── platform-matrix.md
│   ├── performance.md
│   ├── security-checklist.md
│   └── release-checklist.md
│
├── 08-handoffs/
│   ├── LATEST.md
│   └── history/
│
└── 09-research/
```

---

# 六十六、docs/README.md

这是整个文档系统入口。

必须解释：

```text
每个目录负责什么
新 Agent 应该先读什么
文档更新规则
ADR 如何创建
Task 如何创建
Handoff 如何创建
```

新 Agent 打开仓库后应该能通过这个文件理解项目。

---

# 六十七、00-product

负责长期稳定的产品定义。

### vision.md

回答：

```text
折栈是什么
解决什么问题
面向什么用户
长期方向是什么
```

### scope.md

记录：

```text
当前版本范围
明确支持
明确暂缓
明确不支持
```

### terminology.md

统一术语：

```text
Feed
Article
Category
Smart View
Provider
Model
Tool
Skill
MCP
Agent Run
```

避免多个 AI 使用不同词表示同一个概念。

---

# 六十八、01-status

这是当前开发状态的唯一权威来源。

## CURRENT.md

必须保持短而准确。

包含：

```text
当前阶段
当前可运行状态
正在实现什么
当前已知问题
最近的重要变化
下一批工作
最后更新时间
```

不要写成开发日记。

---

# 六十九、ROADMAP.md

维护大的阶段规划。

例如：

```text
M0 Bootstrap
M1 RSS Core
M2 Reader
M3 Categories
M4 AI Providers
M5 Classification
M6 Agent Runtime
M7 Skills
M8 MCP
M9 Polish
M10 Release
```

每个阶段写：

```text
目标
依赖
完成标准
状态
```

---

# 七十、BACKLOG.md

记录还没有进入 Active 状态的任务。

建议：

```text
ID
Priority
Area
Description
Dependencies
```

不要把已完成任务长期留在 Backlog。

---

# 七十一、RISKS.md

记录：

```text
技术风险
架构风险
跨平台风险
数据迁移风险
AI Provider 兼容风险
安全风险
性能风险
```

风险解决后更新状态。

---

# 七十二、active Task 文档

每个正在进行的较大任务创建：

```text
docs/01-status/active/TASK-XXXX-short-name.md
```

模板：

```markdown
# TASK-XXXX
Status: active
Owner:
AI Agent identifier
Started:
YYYY-MM-DD
Area:
frontend / backend / ai / rss / mcp / docs
## Goal
本任务解决什么问题。
## Scope
本任务包含什么。
## Out of Scope
本任务不包含什么。
## Files Expected To Change
列出预计修改区域。
## Dependencies
依赖哪些任务或模块。
## Progress
当前完成情况。
## Decisions
本任务产生的重要决定。
## Tests
已经执行哪些测试。
## Remaining
尚未完成内容。
## Handoff Notes
其他 Agent 接手时必须知道什么。
```

---

# 七十三、多 AI Soft Ownership

并行 Agent 工作时使用 Task 文档表达当前修改范围。

例如：

```text
TASK-0032
Files Expected To Change:
internal/ai/*
docs/03-architecture/ai-provider.md
```

其他 Agent 看到以后应避免同时大范围修改这些文件。

这是一种协作约定。

不要实现复杂的锁系统。

---

# 七十四、COMPLETED.md

Task 完成后：

1. 从 \`active/\` 移出。

2. 重要历史保留到 Git。

3. 在 \`COMPLETED.md\` 追加简短记录。

4. 更新 CURRENT。

5. 更新相关 Design / Architecture / Spec。

COMPLETED 不要复制几十页实现细节。

---

# 七十五、02-design

负责 UI 和交互设计依据。

任何重要 UI 都应该有设计理由。

例如 \`sidebar.md\` 必须写：

```text
Sidebar 的信息架构
为什么采用毛玻璃
Light Theme 行为
Dark Theme 行为
宽度
折叠方式
Hover
Selected
Unread Count
Drag and Drop
Context Menu
Keyboard Navigation
Accessibility
```

---

# 七十六、03-architecture

记录系统为什么这样设计。

不要只画目录树。

例如：

```text
ai-provider.md
```

必须解释：

```text
为什么按 Protocol Adapter 设计
三种协议如何统一
Streaming 如何统一
Tool Calling 如何统一
Provider Capability 如何表示
兼容第三方 Provider 时如何处理
```

---

# 七十七、ADR

重大技术决定创建：

```text
docs/06-decisions/ADR-0001-use-sqlite.md
```

模板：

```markdown
# ADR-0001: Title
Status:
accepted
Date:
YYYY-MM-DD
## Context
为什么需要这个决定。
## Decision
采用什么方案。
## Alternatives
考虑过哪些方案。
## Consequences
带来什么影响。
```

适合 ADR 的内容：

```text
SQLite
sqlc
Model Adapter
Credential Store
MCP SDK
Skill 格式
FTS5
数据迁移策略
```

---

# 七十八、08-handoffs

每次一个 AI 完成较大的工作阶段时更新：

```text
docs/08-handoffs/LATEST.md
```

内容：

```text
刚刚完成了什么
修改了哪些主要文件
当前项目是否能构建
测试结果
当前存在的问题
下一步建议处理什么
有哪些容易误解的设计
```

历史版本进入：

```text
docs/08-handoffs/history/
```

例如：

```text
2026-09-11-ai-provider.md
```

---

# 七十九、09-research

研究记录放这里。

例如：

```text
wails-v3-notes.md
mcp-sdk-notes.md
openai-compatible-notes.md
sqlite-fts5-notes.md
```

研究记录必须区分：

```text
官方文档事实
项目决定
个人推测
```

禁止把未经验证的内容当事实记录。

---

# 八十、根目录 AGENTS.md

创建：

```text
AGENTS.md
```

供后续 AI Agent 阅读。

内容必须简短明确。

至少要求所有 Agent：

```text
开始工作前阅读 docs/README.md
阅读 docs/01-status/CURRENT.md
检查 active Tasks
阅读相关 Architecture 和 ADR
开发过程中更新 Task
修改架构时同步文档
结束工作时更新 Handoff
执行测试
不要删除不知道用途的代码
不要覆盖其他 Active Task 的工作
```

---

# 八十一、新 Agent 工作协议

每个新 AI Agent 接手项目时，按照以下流程：

```text
1
读取 AGENTS.md
2
读取 docs/README.md
3
读取 docs/01-status/CURRENT.md
4
检查 docs/01-status/active/
5
根据工作内容读取相关：
Design
Architecture
Spec
ADR
6
确认当前代码状态
7
创建或更新 Task
8
开始编码
9
运行测试
10
更新文档
11
更新 CURRENT
12
写 Handoff
```

---

# 八十二、文档不得脱离代码

以下情况必须同时改文档：

```text
数据库 Schema 改变
Provider API 改变
Tool Contract 改变
Wails API 改变
UI 信息架构改变
权限系统改变
MCP 设计改变
Skill 格式改变
```

代码和文档必须在同一个工作阶段更新。

---

# 八十三、测试

Go：

```text
go test ./...
```

核心业务优先写 Unit Test。

重点：

```text
Feed Parsing
Feed Deduplication
Category Rules
AI Adapter
Tool Loop
Permission
Skill Parsing
Provider Config
Database
```

前端：

```text
TypeScript typecheck
lint
component tests
```

对关键交互增加测试。

---

# 八十四、AI Adapter Contract Tests

三种 AI Protocol 都必须有 Adapter Test。

使用 Mock Server。

禁止单元测试直接依赖真实收费 API。

测试：

```text
普通对话
Streaming
Tool Call
多个 Tool Calls
Tool Result
Usage
Provider Error
Malformed SSE
Cancellation
Timeout
```

协议分别覆盖：

```text
/v1/chat/completions
/v1/responses
/v1/messages
```

---

# 八十五、RSS Fixture Tests

在：

```text
testdata/
```

保存合法测试 Fixture：

```text
rss.xml
atom.xml
jsonfeed.json
malformed.xml
duplicate.xml
```

保证 Parser 和 Normalize 逻辑可回归测试。

---

# 八十六、跨平台检查

维护：

```text
docs/07-quality/platform-matrix.md
```

至少覆盖：

```text
Windows
macOS Intel
macOS Apple Silicon
Linux amd64
Linux arm64
```

记录：

```text
build
launch
database
credential store
notifications
file dialog
theme
glass sidebar
list and masonry layouts
hover and motion behavior
RSS cover image behavior
```

---

# 八十七、性能

重点关注：

```text
大量 Feed
数十万 Article
FTS 搜索
Article List 与 Masonry View
首图提取、图片懒加载和本地缓存
图片到达时布局稳定性
动效的帧率、输入响应与中断能力
启动速度
数据库 Migration
AI Streaming
```

Article List 必须使用虚拟化。

避免每篇文章产生大量 React component rerender。

---

# 八十八、Accessibility

至少支持：

```text
Keyboard Navigation
Focus Visible
ARIA Labels
Screen Reader friendly controls
Color Contrast
Reduced Motion
Keyboard Focus 与 Hover 行为一致
瀑布流的逻辑阅读顺序
图片替代文本
```

不要只依赖颜色表达：

```text
Unread
Selected
Error
AI Match
```

---

# 八十九、键盘操作

预留快捷键体系。

例如：

```text
J / K
上一篇 / 下一篇
M
已读切换
F
收藏
R
刷新
/
搜索
Cmd/Ctrl + K
Command Palette
```

具体快捷键写入 Interaction Design。

---

# 九十、Command Palette

规划 Command Palette。

允许执行：

```text
搜索 Feed
搜索 Category
打开设置
刷新全部
添加订阅
切换主题
打开 AI
```

以后 Tool 也可以与 Command 系统产生关联。

---

# 九十一、Settings

至少包含：

```text
General
Appearance
Reader
Article View (List / Masonry)
Media & Image Loading
Animation / Reduced Motion
Feeds
AI Providers
Models
AI Classification
Skills
MCP
Tools and Permissions
Data
About
```

---

# 九十二、项目目录建议

可以调整，但保持职责清晰。

```text
Fealdhord/
├── AGENTS.md
├── README.md
├── docs/
├── frontend/
├── internal/
├── migrations/
├── skills/
├── testdata/
├── main.go
├── go.mod
└── Taskfile.yml
```

---

# 九十三、README

根 README 面向开发者和项目使用者。

包含：

```text
项目简介
当前状态
技术栈
开发环境
如何启动
如何测试
如何构建
文档入口
```

不要把几十页架构说明塞到 README。

详细内容进入 docs。

---

# 九十四、Git Hygiene

生成：

```text
.gitignore
```

禁止提交：

```text
API Keys
.env secrets
数据库运行数据
缓存
build artifact
本机 IDE 临时文件
```

---

# 九十五、代码质量

Go：

```text
gofmt
go vet
```

保持 Package 小而职责明确。

避免：

```text
巨大 service.go
巨大 utils.go
万能 Manager
```

TypeScript：

```text
strict
```

不要随意：

```text
any
```

---

# 九十六、错误处理

禁止：

```go
\_ = doSomething()
```

忽略重要 Error。

禁止为了让编译通过吞掉错误。

Context cancellation 要正确传递。

---

# 九十七、不要提前复杂化

第一阶段暂时不要实现：

```text
Embedding Database
复杂向量数据库
多人云同步
Web Account System
移动 App
任意代码 Skill
Plugin Marketplace
复杂 Workflow Designer
```

但是架构不能把未来可能性封死。

---

# 九十八、第一阶段实施顺序

收到本指令后直接开始实施。

第一阶段按照以下顺序推进。

## Phase 0

环境和仓库基础：

```text
检查 Go
检查 Wails 3
创建项目
创建 React + TypeScript
集成 Tailwind
集成 shadcn/ui
集成 Lucide
创建 AGENTS.md
创建完整 docs 目录
建立初始文档
```

## Phase 1

应用 Shell：

```text
Window
App Shell
Glass Sidebar
List / Masonry view switch with real layouts
Semantic Hover and icon motion primitives
Article List placeholder
Reader placeholder
Theme
Resizable layout
Routing / View State
```

达到可运行状态。

## Phase 2

数据库：

```text
SQLite
Migrations
sqlc
Repository
Feed
Article
Category
Reading State
```

## Phase 3

RSS：

```text
Add Feed
Fetch Feed
Parse
Store
Refresh
Scheduler
ETag
Last Modified
RSS cover image extraction & metadata
```

## Phase 4

Reader：

```text
Article List
Virtualized Masonry View
Cover image lazy loading and fallback
Reader
Read State
Favorite
FTS
Reader Mode
```

## Phase 5

AI Provider：

实现完整：

```text
Chat Completions Adapter
Responses Adapter
Messages Adapter
```

包括 Streaming 与 Tool Calling。

## Phase 6

AI Classification。

## Phase 7

Agent Runtime。

## Phase 8

Built in Tools。

## Phase 9

Skills。

## Phase 10

MCP Client。

## Phase 11

MCP Server。

## Phase 12

Polish 和跨平台验证。

---

# 九十九、第一次执行必须做什么

收到本 Prompt 后，不要只生成架构说明。

立即执行以下工作：

```text
1
检查仓库当前状态。
2
如果项目不存在，初始化 Wails 3 + React + TypeScript 项目。
3
检查当前官方 Wails 3 文档。
4
创建 AGENTS.md。
5
创建 docs 完整目录。
6
填写最初版本：
docs/README.md
docs/00-product/*
docs/01-status/CURRENT.md
docs/01-status/ROADMAP.md
docs/02-design/*
docs/03-architecture/overview.md
docs/03-architecture/ai-provider.md
docs/03-architecture/agent-runtime.md
docs/03-architecture/mcp.md
docs/03-architecture/skills.md
7
创建第一批 ADR。
8
初始化 Tailwind CSS 和 shadcn/ui。
9
完成应用主 Shell。
10
实现 Light / Dark / System。
11
实现毛玻璃 Sidebar。
12
确保项目能够成功启动。
13
运行能够执行的测试。
14
更新 CURRENT 和 Handoff。
```

---

# 一百、第一批 ADR

至少考虑创建：

```text
ADR-0001-use-wails-v3.md
ADR-0002-use-sqlite.md
ADR-0003-use-sqlc.md
ADR-0004-ai-protocol-adapters.md
ADR-0005-tool-registry.md
ADR-0006-skill-format.md
ADR-0007-official-mcp-go-sdk.md
ADR-0008-local-first-data-model.md
```

不要为了数量创建空洞 ADR。

每份都必须有实际决策价值。

---

# 一百零一、UI 第一版验收

第一版 App Shell 至少达到：

```text
应用可以正常启动
Sidebar 存在
Sidebar 毛玻璃效果完成
列表和瀑布流视图均能实际切换
瀑布流图片卡片和无图卡片完成
Hover / Press / Focus 动效样例覆盖关键图标
动效符合折栈视觉，未复制 Telegram UI
Reduced Motion 可以生效
系统主题可跟随
Light / Dark 可切换
Sidebar 有全部、精选、未读、收藏
存在分类区域
存在订阅区域
存在 Article List
存在 Reader
布局看起来像正式桌面应用
窗口缩放正常
没有明显横向溢出
```

不要使用大量假 Dashboard Card 填页面。

---

# 一百零二、代码与文档完成定义

一个任务只有同时满足下面条件才算完成：

```text
代码已经实现
项目能够编译
对应测试通过
相关文档已更新
Task 状态已更新
CURRENT 已更新
Handoff 已更新
不存在明显遗留临时代码
与本任务相关的动效和图片状态已验证
```

只有 UI 截图完成不算完成。

只有代码能编译但没有文档，也不算完成。

---

# 一百零三、设计修改规则

在开发过程中发现原设计不合理，可以修改。

但需要：

```text
1
判断是否属于重大决策。
2
重大决策创建 ADR。
3
更新 Architecture 或 Design。
4
更新 CURRENT。
5
实现代码。
```

不要因为旧 Prompt 写过某件事，就保留明显错误设计。

技术事实优先。

---

# 一百零四、外部信息验证规则

涉及以下内容时查看当前官方文档：

```text
Wails 3
OpenAI API
Anthropic API
MCP
Go SDK
第三方库 API
```

不要依据过时记忆编写 API。

优先级：

```text
官方文档
官方 GitHub
正式 Release Notes
源码
第三方资料
```

---

# 一百零五、AI Provider 命名规则

技术层统一：

```text
ChatCompletionsAdapter
ResponsesAdapter
MessagesAdapter
```

UI 显示：

```text
Chat Completions
Responses
Messages
```

可以在帮助文字解释：

```text
Chat Completions 对应 /v1/chat/completions
Responses 对应 /v1/responses
Messages 对应 /v1/messages
```

绝对不要实现：

```text
/v1/completions
```

---

# 一百零六、最终产品体验目标

用户可以像普通 RSS Reader 一样使用折栈：

```text
订阅
列表阅读
瀑布流图文浏览
RSS 自动首图
动态交互
搜索
分类
收藏
```

同时也可以直接告诉 AI：

```text
帮我看看今天最值得看的技术文章。
找出最近一个月关于 Go 1.XX 的文章。
把这些文章加入 Go 分类。
找出半年没有更新的订阅。
检查哪些 Feed 已经失效。
总结这篇文章。
对比我之前收藏的相关文章。
把最近三天 AI 分类中的重要内容做成简报。
```

Agent 通过：

```text
Context
Tools
Skills
MCP
```

完成操作。

---

# 一百零七、最终架构目标

整体关系保持：

```text
                    React UI
                       │
                       │
                 Wails Bindings
                       │
                       ▼
                Application Layer
                       │
       ┌───────────────┼────────────────┐
       │               │                │
       ▼               ▼                ▼
   RSS Domain      Agent Runtime      Search
                       │
             ┌─────────┼─────────┐
             │         │         │
             ▼         ▼         ▼
           Model      Tools     Skills
             │         │
      ┌──────┼─────┐   │
      │      │     │   ├──── Built in
      ▼      ▼     ▼   │
    Chat Responses Msg └──── MCP
      │      │     │
      └──────┼─────┘
             │
         Providers
                       │
                       ▼
                     SQLite
```

任何模块不得通过大量跨层调用破坏这个边界。

---

# 一百零八、开始执行

现在开始实际处理项目。

先读取现有仓库。

如果仓库为空，从 Phase 0 开始。

如果已有部分代码：

```text
不要盲目重建
不要删除未知实现
分析当前状态
建立文档
记录实际完成度
从正确阶段继续
```

你的工作结果必须是：

```text
可执行代码
+
可维护文档
+
清晰项目状态
+
下一位 AI 能直接继续工作的仓库
```
