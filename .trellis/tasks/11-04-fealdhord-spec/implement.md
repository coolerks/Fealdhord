# 执行计划

## 本轮规格任务

- [x] 归档原文并保留 SHA-256。
- [x] 核对仓库、工具链和官方技术资料。
- [x] 建立中文产品、设计、架构、功能、API、质量与 ADR。
- [x] 将 113 章节映射到规格、验收 ID 和阶段。
- [x] 建立前后端拟定代码规范，清除空模板与 English 语言冲突。
- [x] 核验链接、追踪、契约结构与实际状态，写交接。
- [x] 已呈现最终规划摘要，用户于后续消息明确要求开始实施。

## 后续实施顺序

1. Phase 0：固定 Go／Wails／Node／生成器；调查 doctor 失败；建立最小 Wails + React／TS／Vite 工程；真实验证窗口、绑定、事件和取消。成功后填写可执行 README 命令。
2. Phase 1：Tailwind／shadcn／Lucide／Query／Zustand／Virtual／motion；正式 Shell、玻璃、双布局、动态 lanes 原型、锚点、主题、分栏和 Reduced Motion。只在测试用 fixture，生产真空态。
3. Phase 2：实际 Go 驱动 FTS5、goose、sqlc、身份／分类／状态／媒体 schema、迁移和恢复实验。
4. Phase 3：统一网络安全网关、发现、三格式 Parser、去重、条件请求／调度、首图候选和异步探测。
5. Phase 4：完整 RSS 阅读器、虚拟双视图与媒体失败、Reader、状态、分类、FTS 中文搜索、OPML、快捷键／Palette。完成零 AI 路径。
6. Phase 5：CredentialStore、多 Provider／Model、独立三 Adapter 和基础 Chat；Mock wire contracts 全覆盖。
7. Phase 6：多分类一次判断、缓存／manual override、Prompt 测试、精选与预算。
8. Phase 7–8：Runtime、Context、许可快照／审计／取消，25 内置工具与真实副作用。
9. Phase 9–11：声明式 Skills，MCP Client 两 transport 与资源／Prompt，默认关闭的认证 Server。
10. Phase 12：五平台原生、1k／10k／100k、无障碍／动效／媒体、安全与打包发布收敛。

每个阶段进入时创建或细分独立可验收实现任务，依顺序执行，主会话独立研究、实现、检查。禁止提前为全部阶段派发子任务／辅助进程。

## 验证命令与门禁

本轮实际验证见 docs/09-research/validation.md。未来工程存在后执行 go test ./...、go vet ./...、gofmt 检查、frontend typecheck／lint／test／build、sqlc generate、Wails bindings 生成与桌面启动。具体 scripts 由 Phase 0 创建并验证，当前不能宣称可执行。

## 高风险区域与回滚

Schema／身份／FTS：修改前备份，迁移故障注入；网络网关：恶意 URL／DNS／限额测试先过；权限：直接绑定和 MCP 越权测试；动态 lanes：缩放／图片到达锚点与顺序；Adapter：流中断绝不执行半参数。配置改变均保持旧数据可恢复。

## task.py start 前

确认最新 PRD／design／implement 完整、无阻塞产品决策、用户后续明确批准最新摘要；加载 trellis-before-dev 与 Phase 1.4 详情，才运行 task.py start。inline 模式不要求 implement.jsonl／check.jsonl，空文件不代表漏派代理。
