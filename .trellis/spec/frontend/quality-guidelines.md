# 前端质量与性能

状态：已评审代码契约；Phase 0 已有基础工程，涉及其余领域的规则仍待对应阶段实现。

## 1. 范围与触发条件

组件修改、布局／动效和交付前。

## 2. 签名

预期 scripts：typecheck/lint/test/build/test:e2e；Phase 0 已提供并通过前四项；E2E 待后续阶段。

## 3. 契约

两视图虚拟化，动态尺寸／顺序／锚点受测；真实空态，不以模拟数据冒充已实现；截图不能证明动画。

## 4. 校验与错误矩阵

100k 全 DOM→不通过；Keyboard 顺序错→不通过；跨 WebView 未测→不得标全平台通过。

## 5. 良好、基础和异常案例

良好：DOM 仅视口加 overscan；基础：一列无横溢；异常：未知图片到达导致卡片重叠。

## 6. 必需测试

组件行为、真实 Wails 集成、1k/10k/100k、Hover/Focus/Press/Selected/Disabled、动画 Off／系统减少动态。

## 7. 错误与正确做法

错误：只截屏就宣布完成。正确：行为断言／录屏／原生平台证据与文档一致。

权威详细设计：[架构](../../../docs/03-architecture/frontend.md)。

## Phase 0 真实依据

[基础契约](../../../docs/04-specs/bootstrap.md)记录实际服务、DTO、事件合并、测试与生成命令。生成绑定不能手改；早到乱序事件使用同一个 sequence 合并器，类型与构建测试必须采用兼容的 Vite／Vitest 版本。

Taskfile 的工作目录放在任务级 dir，不能给单条 cmd 写 dir 并假定生效；frontend:check 是真实复核入口。

## Phase 1 真实依据

按用户约束，非必要不做 E2E；本阶段采用 19 项组件／单元用例、真实设置 SQLite 测试和原生编译，不能把未执行的平台／帧率／触控门禁写成通过。JSDOM 几何必须同时提供 clientHeight 与 scrollHeight，避免虚拟器滚动范围被错误钳制到 0。构建写 dist 时不并行运行读取 go:embed 的 Go 检查；先 build，再 task check。实际提示和证据见[阶段验收](../../../docs/09-research/phase1-validation.md)。
