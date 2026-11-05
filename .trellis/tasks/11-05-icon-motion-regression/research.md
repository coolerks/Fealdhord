# 缺陷分析：严格模式下悬停图标播放被取消

## 1. 根因类别

B（跨层契约）：父组件的立即启动与新挂载 Motion 子树的 React 副作用生命周期不匹配。D（测试覆盖缺口）：模拟播放器不执行 Motion 副作用；图标目录测试预先挂载后才播放，绕过真正失败路径。E（隐含假设）：假设取得 imperative handle 就意味着子树生命周期稳定，假设 Promise 完成和 data-animating 能代表可见运动。

证据：main.tsx 保留 React.StrictMode；AnimatedGlyph 在 playing=true 的更新中新增库组件，于 useLayoutEffect 调用 startAnimation。真实组合探针在 StrictMode 下 160–2880ms 采样全为 data-animating=true、transform:none；同探针移除 StrictMode 后通过。Motion useAnimationControls 的 mount 清理会 controls.stop，控制器停止不保证原 start Promise 完成。

## 2. 修复为什么失败

上轮已补充单次结束和取消，但把父级 layout effect 当成稳定启动点。模拟控制测试的 cancel 不会中止真实 Motion，预挂载目录测试也没有经过悬停新增子树，所以两组测试都通过，组合仍坏。吞掉动画错误并回退静态还会降低故障可见性；本次无异常也能卡住，不能只看日志。

## 3. 预防机制

| 优先级 | 机制 | 行动与状态 |
| --- | --- | --- |
| P0 | 回归证据 | 真实 AnimatedGlyph＋Motion＋StrictMode＋首次 hover 已复现失败；非严格模式对照通过 |
| P0 | 调度边界 | 候选 passive effect 后可取消微任务已通过同一探针，产品修复等待最终评审 |
| P0 | 测试门禁 | 必须断言中间帧实际 SVG 变化和最终归位，前端质量规范已实际更新 |
| P1 | 取消安全 | 实施时验证离开、模式切换、卸载和重复进入，不用超时强制清 busy |

## 4. 系统性扩展与置信度

按钮、菜单、设置、阅读器和被动图标均复用同一 GlyphPlayback，因此一个挂载竞态会传播到全部图标。真实应用根入口开启 StrictMode，测试渲染 App 却没有始终复现同一挂载环境。系统减少动态或运行旧产物也可能解释不动，但不能解释保持所有其他条件、只移除 StrictMode 后通过的对照。严格模式挂载链路是已证实缺陷，对本次主要根因置信度高于 90%；不据此声称排除了所有平台问题。

候选只在研究目录临时复制组件，改为 useEffect 并在可取消 queueMicrotask 中启动；同一 StrictMode 探针通过。研究临时文件已移除，生产代码未改。正式回归应在用户批准后直接使用生产组件，先失败再修复。

## 5. 知识沉淀

已更新 .trellis/spec/frontend/quality-guidelines.md：Promise／状态断言不能代替运动；必须覆盖按需挂载、StrictMode、真实中间帧与终态。已建立本任务记录，后续修复同步组件规范、CURRENT 与交接。src/templates/markdown/spec 在本仓库不存在，无可同步的模板源码，不新建虚构目录。
