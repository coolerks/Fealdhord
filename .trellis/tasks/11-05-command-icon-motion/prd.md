# 命令面板图标动画修复

## 目标与证据

修复命令面板可用选项图标不播放。用户已确认创建任务并继续；这是已定位的局部缺陷。cmdk 1.1.1 给可用选项设置 data-disabled=false，AnimatedGlyph 的 [data-disabled] 判断将其误判为禁用，阻断 hover 和键盘选择动画。

## 范围与实现

仅修正共享图标的禁用值判断，保留原生 disabled、aria-disabled=true、Radix 空 data-disabled 以及 cmdk true 的禁用语义。使用真实 Command／CommandItem 和真实 Motion 编写回归，不再用缺属性的手写 role 节点替代。保留 StrictMode、按需挂载、自动归位、动态偏好、命令执行和可访问名称；不重做菜单或业务接口。

## 验收与执行

先用真实 cmdk 可用行 hover／键盘选中测试复现失败，再做局部修复；真实 SVG 中间帧变化且完成后归位。真正禁用项仍静态；Off／Subtle 不变。执行相关与全部前端测试、类型、lint、macOS 构建并同步应用包；更新组件规范、CURRENT 与交接，直接提交并归档。

用户体验恢复命令项动效，开发体验准确兼容两种组件属性约定，AI Agent 体验保持语义与禁用状态一致。无产品决策待澄清，PRD 已自检收敛。


## 完成证据

真实 CommandItem 的两项回归在旧代码因 SVG 不运动失败，修正禁用属性判断后通过，覆盖悬停、重播、键盘选择和真正禁用项。全部前端 43 项、类型、lint（既有 1 项提示）、生产前端和 macOS 构建通过，本地应用包与签名已更新。保持 StrictMode 修复，规范记录属性值差异。
