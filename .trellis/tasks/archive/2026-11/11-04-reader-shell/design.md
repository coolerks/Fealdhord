# Phase 1 技术设计

以既有设计为基线，最小差距是将工程诊断页换成正式阅读 Shell；保留诊断代码但移动到设置的开发信息区域，不作为主界面。

前端 app/Shell、features/articles／reader／settings、共享语义图标、shadcn 基础组件；Query 管异步空数据入口和设置快照，Zustand 管当前选择／过滤／面板。TanStack Virtual 列表与 lanes 瀑布流，逻辑 DOM 顺序、ResizeObserver、ID＋offset 锚点。禁止两套文章状态和 CSS columns。

Go settings 包与 database 包负责最小 app_settings：modernc SQLite、嵌入 goose 迁移、sqlc 两个参数化查询；SettingsService.Get／Update 以 revision 做乐观并发。UI 写请求按序合并，失败提示并保留真实持久化状态。启动读库失败通过安全中文错误页，不 silently 用内存冒充已保存。

选择、分栏、视图及设置恢复用有界结构；文章 ID 锚点在无文章时为空。Shell 空态给明确阶段说明，设置中的未交付区域解释而非无效开关。数据库在平台用户数据目录，测试用临时目录，不触碰真实凭据。

重要调整：最小设置表提前到本阶段，以满足原有 SQLite 约定；其他 schema、FTS 与备份完整策略仍由 Phase 2 交付。
