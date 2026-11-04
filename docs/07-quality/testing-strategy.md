# 质量与验收策略

Phase 0 基础调用链检查已有 [真实验收](../09-research/phase0-validation.md)；其余领域与发布门禁仍待执行，规格阶段记录见 [文档核验](../09-research/validation.md)。

## 测试层级

| 层级 | 重点与断言 |
| --- | --- |
| Go 单元 | Parser／去重／分类有效成员／Prompt 缓存／上下文预算／Skill 校验／错误分类 |
| Go 集成 | SQLite 真实驱动、迁移、FTS 同步、事务回滚、备份恢复、并发唯一性 |
| AI 协议 | httptest 三 endpoint，文本、Streaming、多工具、结果续轮、usage、畸形 SSE、取消、超时、错误 |
| 网络安全 | 测试 DNS／dial／redirect 注入，私网、IPv6 映射、重绑定、body／像素限额、秘密重定向 |
| 前端组件 | Testing Library + 可控时间，键盘／Focus、乐观回滚、图像失败、主题和 Reduced Motion |
| Web UI 集成 | Playwright 候选，经 Mock 边界验证列表／瀑布流、Reader 恢复、权限卡、设置 |
| 原生桌面 | 真实 Wails 绑定、系统主题、文件对话框、Keychain／Credential Manager／Secret Service、WebView 差异 |
| 性能 | 1k／10k／100k 可复现数据、冷／热启动、查询、滚动、图片与取消 |

Mock 浏览器不能替代原生桌面验收，跨编译不能替代 launch。UI 截图证明静态外观，语义动画需真实交互断言／录屏。

## Fixture 策略

在 testdata 保存自制、可分发、无秘密的 rss.xml、atom.xml、jsonfeed.json、malformed.xml、duplicate.xml，以及 Media RSS、相对 URL、srcset／data-src、追踪像素、XSS 和 OPML 多分类 fixture。每项测试声明预期 article ID、数量、候选来源和错误，不以“无 panic”作为唯一断言。

## 预期命令

代码创建后提供：`go test ./...`、`go vet ./...`、`gofmt` 检查；frontend scripts `typecheck`、`lint`、`test`、`build` 和 `test:e2e`。Phase 0 已创建工程并执行 test／vet、前端 typecheck／lint／test／build；test:e2e 和其余领域／平台门禁尚未执行。sqlc generate 和 Wails bindings 生成后应无非预期 diff。

## 完成定义

实现存在、可构建、相关测试通过、文档／任务／CURRENT／Handoff 同步、无明显临时代码、涉及媒体与动效时已覆盖对应状态。单纯 UI 截图或编译成功不算功能完成。新发现失败先修复相关层，再重复必要检查，不为凑数量运行无关测试。

验收 DOC-01：对应 AC-D01 至 D05，113 章节映射完整、本地链接有效、关键契约七节结构完整、状态与阶段一致、无模板占位、不把计划写成事实。
