# Phase 1 验收记录

日期：2026-11-04，macOS arm64。本阶段为正式 Shell 和偏好恢复；原生窗口往返证据仍引用 Phase 0，不重复非必要 E2E。

## 已执行

| 命令／检查 | 实际结果 |
| --- | --- |
| go test -race ./internal/settings/... | 5 项真实 SQLite 测试通过，包含重开、8 路 CAS、非法输入、旧值与未来 schema |
| wails3 task check | Go 全包 test／vet 通过，前端 typecheck／lint 返回成功，4 文件共 19 项测试通过 |
| wails3 build | 2 Services／6 Methods／12 Models／1 Event，生成绑定、TypeScript、Vite 和 macOS 原生编译成功 |
| git diff --check | 无空白错误 |

组件测试实际验证 10,000 条 fixture 的 DOM 有界（小于 100 个条目）、真实多列横向位置与逻辑 DOM 顺序、坏图／长图／窄宽度、ID 锚点冲刷及恢复、Reader 滚动、键盘／焦点、主题／动效、保存队列与失败重试、真实空态及基础诊断回归。fixture 不进入生产入口。

## 保留提示与边界

- lint 为 0 error、1 warning：react-hooks/incompatible-library 指出 TanStack useVirtualizer 的可变实例不适合自动缓存。项目未启用 React Compiler，组件明确 use no memo，未压制规则；启用 Compiler 前必须重新验证，不能缓存 Virtualizer 方法结果。
- Vite 保留首包体积提示：设置／命令按需分块后，主 JS 629.69 kB（gzip 203.14 kB），CSS 78.72 kB（gzip 13.98 kB）；本地嵌入资源可构建，不等于启动性能已认证。后续按真实平台测量再拆分共享依赖。
- JSDOM 通过测试专用几何模型提供 ResizeObserver／宽高／滚动高度；不模拟真实 WebView 绘制、图片网络和连续拖动帧率。1k／100k、异步媒体到达偏差、五平台、完整触控及原生动效校准仍属后续质量门禁。
- 本阶段未运行浏览器／原生 E2E；未启动或改写用户数据目录中的真实数据库，SQLite 测试使用临时目录。
- 生产文章入口当前空集合；收藏／已读回调只为后续领域 API 预留统一 Query 通路，尚无 RSS 文章持久化。订阅／AI 有明确阶段说明。

## 检查中修复的实际问题

初始化测量曾可能覆盖保存的 ID 锚点，现恢复期间禁止捕获；快速卸载前冲刷防抖值。Reader 关闭／切换布局前冲刷位置，打开时使用 preventScroll 恢复标题焦点。测试几何必须提供 scrollHeight，否则虚拟器正确执行的滚动边界会被 JSDOM 的 0 错误钳制。

曾并行运行前端重建和 Go 全包检查，触发 dist 删除时 go:embed 读取竞争；已改为先完成构建，再顺序执行 wails3 task check 并通过。它不是产品运行故障，也不应通过忽略嵌入资源错误绕过。
