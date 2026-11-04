# 统一术语

| 标识 | 中文 | 含义 |
| --- | --- | --- |
| Feed | 订阅源 | 一个可刷新的 RSS／Atom／JSON Feed 地址 |
| Article | 文章 | 订阅源中的一个稳定条目，正文和用户状态分离 |
| Category | 分类 | 用户创建的组织维度，可多对多关联订阅和文章 |
| Smart View | 智能视图 | 查询或评分生成的结果集；精选不是普通分类 |
| Reader | 阅读面板 | 展示清理后的订阅正文或提取正文 |
| Reader Mode | 阅读模式 | Go 抓取原网页并提取、清理正文 |
| Provider | AI 服务 | 协议、地址、认证引用和兼容设置的组合 |
| Model | 模型 | Provider 下的具体模型及能力声明 |
| Tool | 工具 | 输入／输出有 Schema 的原子操作 |
| Skill | 技能 | 组织上下文和工具的声明式指导，不执行代码 |
| Agent Run | Agent 运行 | 有限轮次、可取消、有权限与审计的执行过程 |
| MCP Client | MCP 客户端 | 连接外部服务并接入工具／资源／提示词 |
| MCP Server | MCP 服务端 | 折栈向外部客户端暴露受限能力 |
| 首图 | 文章预览图 | 根据源数据提取的卡片图片，不等于 favicon 或正文所有图片 |
| 取消 | cancellation | 停止后续工作，不保证撤销已经提交的写入 |

技术协议名统一 `chat_completions`、`responses`、`messages`；界面显示 Chat Completions、Responses、Messages。
