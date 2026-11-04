# 最近交接：原生阅读体验修订

日期：2026-11-04，分支 codex/native-reading-experience。用户要求先修复菜单、材质、配色与动效，再自动开始 Phase 2；无需再次询问。

## 本批交付

macOS 原生菜单栏与标准编辑／窗口 role；Windows／Linux 应用内 Menubar；全局上下文菜单与剪贴板目标快照；中性灰白／深色、透明标题栏与真实窗口模糊材质、双栏浏览和可选并排 Reader。Motion 图标改为 lucide-animated 按需源码，父按钮 Hover／Focus／Press 触发，保留 MIT，减少动态不挂载内部动画实例。

## 证据与边界

原生构建成功，设置 race 通过；本机系统菜单切换 Masonry、全局自定义右键、设置回焦和灰白窗口已实际观察。完整命令和交互边界见[原生体验契约](../04-specs/native-experience.md)。未做 Windows 实机、逐帧／录屏认证。原生材质受 OS 减少透明度与背景影响，不能以一张截图保证所有环境表现。保留既有 Virtualizer Compiler 与首包体积提示。

## 后续

立即由独立任务承接 Phase 2 数据基础，已有设置迁移编号 00001 不重写；补核心领域 schema、FTS5／索引一致性、身份与分类来源约束、迁移失败及备份恢复。RSS 抓取属于 Phase 3，不能注入生产假文章。
