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


## 按需挂载动画的真实组合回归

图标播放 Promise 完成、data-animating=true 或 mock startAnimation 被调用，均不能证明画面运动。AnimatedGlyph 这种交互时新增 Motion 子树的组件，必须用真实图标／Motion，在应用同样的 StrictMode 环境下，从 hover／focus 触发按需挂载，再验证中间帧 SVG transform、路径或透明度确实改变，最终自动归位及再次进入可重播。预先挂载图标后再调用句柄的目录测试，只能检查图标自身，不能替代外层调度回归。

imperative handle 就绪不等于 StrictMode 副作用重放完成。播放不能在新子树尚可能清理时抢先启动；取消必须覆盖未执行调度和已执行播放。保留 StrictMode，不用删除严格模式、固定延时清 busy 或吞错静态回退掩盖根因。手动时钟可验证中间帧及完成，不声称原生帧率达标。


第三方控件兼容必须覆盖真实生成属性：cmdk 可用项含 data-disabled=false，Radix 用空属性表示禁用。只有 role／label 相同的手写节点不覆盖这些约定；command-icon-motion.test.tsx 使用真实 CommandItem 和 Motion 断言悬停／键盘运动、归位重播及禁用静态。

批量脚本必须逐文件读取与写入对应内容，不跨文件复用上一次的文本变量；提交前核对 diff --numstat，非重构出现大段删除必须先查原因。组件／类型检查不会解析独立 CSS，原生／生产构建必须覆盖资源编译，不能据组件通过宣称样式完整。
