# Agent 运行时

流程：构建最少上下文 → 解析 Skill → 求工具 allowlist 与用户策略交集 → Model → 收集完整 Tool Calls → 校验 → 权限 → 执行 → 结果回传 → 下一轮或结束。

拟定默认限制：max_turns=16（模型请求次数）、max_tool_calls=64（实际执行尝试数）、总时限 180 秒；可配置但有上限。等待用户许可暂停模型／工具活动但总时钟继续，审批过期后运行结束，不无限占资源。每次执行前检查取消、预算和权限。

状态：created → running → waiting_permission → running → succeeded；任意活动态可到 failed／cancelled／timed_out／limit_exceeded。terminal 后不再发新工具。退出应用后的非终态记录标记 interrupted，重开不自动重放写操作。

只读且无依赖的工具可有界并行；写入和破坏性操作按模型顺序串行。重复 call_id 返回既有结果，不重复副作用；外部写入超时标记 outcome_unknown，不自动重试。工具结果即使失败也以统一错误反馈模型，认证和权限失败不通过模型循环反复尝试。

审批绑定 run_id、tool_call_id、规范化参数哈希、目标 revision、策略 revision 和过期时间，任一变化都重新展示。取消不撤销已提交写入；历史清楚显示已完成、未执行和结果未知。

ContextBuilder 按 scope 限制检索：全局、分类、当前文章及选中段落。文章与 MCP 内容标为不可信材料，不能覆盖系统权限；按 token 预算裁剪并保存引用 article_id，禁止默认发送全库、远程图片或密钥。

验收 AGENT-01：16 轮／64 工具／总时限默认边界、取消、审批、重复 call_id、外部结果未知与重启 interrupted 有断言；任何超限均停止新副作用。审批 expires_at 取运行 deadline 和当前时间加 5 分钟的较早值，等待许可计入 180 秒总时限。
