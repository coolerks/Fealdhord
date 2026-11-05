# 最近交接：图标动画失效回归修复

日期：2026-11-05。分支 codex/interface-clarity。

已修复 StrictMode 下所有图标首次 hover 后不运动且 busy 卡住的问题。图标按需挂载时父级 useLayoutEffect 抢先启动，子树副作用重放取消播放，完成 Promise 悬置；现在于 useEffect 后排入可取消微任务，保留 StrictMode、官方轨迹及现有模式偏好。

上一轮模拟控制测试不执行真实 Motion 生命周期，目录测试又在预挂载后启动，均绕开了失败路径。本轮真实组合回归先在旧代码失败，再验证 38 个图标至少 3 个不同中间 SVG 状态、收尾和重播；覆盖按钮、菜单、被动图标。新增启动前 Off／卸载取消检查，全部前端 41 项、类型、lint（1 项既有 Virtualizer 提示）、生产前端与 macOS 构建通过。

另在 Chrome 保留 StrictMode 与真实时钟进行针对性核验：Settings 首轮 339、第二轮 338 个不同 SVG 状态，Layers 多段 117 个，每轮自动归位。临时探针与独立服务已清理。未执行完整 E2E、原生 WebKit 帧率或 Windows 实机采样。

组件与质量规范已实际更新，要求真实首次挂载路径及中间帧断言，禁止仅凭 Promise／data-animating 宣称运动。src/templates/markdown/spec 不存在，未创建虚构模板。菜单、业务接口、数据与焦点策略保持；其他未跟踪框架文件保留。

实际状态见[当前状态](../01-status/CURRENT.md)。后续仍为 Phase 3 RSS；本轮直接提交，不推送。

修复提交 c047117；[根因与预防记录](../../.trellis/tasks/archive/2026-11/11-05-icon-motion-regression/research.md)随任务归档。
