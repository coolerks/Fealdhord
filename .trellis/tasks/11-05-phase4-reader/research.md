# Phase 4 完整基础阅读器：规划草案

日期：2026-11-05。用户授权规划完成后直接执行，不要求逐次规划确认。用户已明确同意创建任务；规划完成后直接实施，采用本轮授权，不重复请求规划确认。主会话独立完成，不使用子代理。

## 目标

消除最新 500 篇的功能上限，支持全库查询和真实订阅／分类导航，并完成安全阅读与 OPML 迁移。暂停视觉调整，沿用已交付菜单、AnimatedGlyph、布局与偏好保存。

## 已确认源码事实

- internal/library/service.go 的 List 仅 limit，最大 500；Summary 缺 feed_id／原文 URL；Get 仅 content_text。
- frontend/src/lib/library-api.ts 固定请求 500；Shell 使用本地 filterArticles；搜索未使用 FTS。
- article-state.ts 和 commands.ts 读取单一固定 articleQueryKey；接分页时必须共同调整缓存操作，不能仅改 Shell。
- internal/database/search.go 具备 CJK 编译／连续文本校验，但最多扫描 1000 候选，只有 id/title，没有分页／过滤／索引 generation。
- 分类层已有 Create／Move／Delete、来源优先级和墓碑；缺公开管理服务、完整元数据更新、导航计数和操作 UI。
- reader_contents 表已存在；Reader 当前分段显示纯文本，无安全 HTML／提取／原文入口／自动已读偏好。
- Wails 3.0.0 本地源码提供 OpenFileDialog／SaveFileDialog 的 PromptForSingleSelection；授权路径必须留在 Go。
- Phase 3 已提供受控网络、图片策略和本地媒体 ID 路由，后续阅读抓取必须复用。

## 顺序与独立交付

1. 全库查询与分页：ArticleQuery／游标、FTS／过滤／排序、摘要与计数；前端统一 Query 与分页加载，状态写入覆盖相关缓存，键盘与锚点保持。先完成可用纵向路径并提交。
2. 分类与订阅导航：分类 CRUD／移动／排序／元数据、删除预览与冲突、真实未读数、Feed／Category scope；文章手动加入／排除／恢复自动；Feed 默认分类影响范围明确。直接提交。
3. 阅读增强：清理后的订阅 HTML、显式 Reader Mode／取消／缓存／失败保留正文、系统浏览器原文入口、首次正文成功后可配置标已读、字体设置。HTML 中远程资源不能绕开 Go 媒体策略。直接提交。
4. OPML：原生对话框授权 token，受限解析预览／确认／事务导入，树形与多分类导出，覆盖已有文件采用安全写入，不自动无限并发刷新。菜单与命令接线，直接提交。
5. 联验与收尾：真实 SQLite、关键安全与状态回归、适当数据规模探针、绑定／类型／lint／原生构建、中文规格／CURRENT／交接，归档任务并记录日志。

## 关键契约

遵循 docs/05-api/wails-bindings.md：ArticleQuery.limit 默认 50、1..200；cursor 绑定查询哈希和快照上界，时间排序使用 sort_at/id 键集；relevance 额外绑定 rank／generation；过期返回 CursorExpired 并允许重查恢复锚点。过滤必须在数据库生效，不能在分页后本地过滤冒充全库。全文输入最长 512 字符、安全编译，中文连续性校验不得导致越过候选上限后假报全量结果。

TanStack Query 管领域缓存，Zustand 仅保存 UI scope／选择／短期状态；scope/filter/search/sort 必须进入键。状态修改复用串行写入与失败回滚，覆盖相关缓存，不在收藏或未读视图遗留错项。

分类深度最多 5、禁止环、同父级重名拒绝；删除不删文章／订阅，子项迁移原子化，冲突先改名。manual excluded 优先，恢复自动删除 manual 来源。父分类不默认包含后代。

Reader 抓取经统一 netpolicy，受限 HTML 字节再交给提取器，禁止库自行联网。安全 HTML 白名单必须禁止脚本／事件／iframe／表单／任意远程子资源，链接通过受控动作交给系统浏览器；正文媒体只登记本地 ID。失败或取消保留订阅正文；不绕登录／付费墙。

OPML 最大 10MiB／深度 16／10000 Feed；不解析外部实体、不在解析时访问网络；预览零写入；明确确认后单事务；大小写归一 host 但保留 URL query／路径语义。文件能力一次性、有到期与用途限制，前端不接任意绝对路径。

## 不包含

AI／Agent／Skills／MCP、动态活跃度刷新调度、视觉重做、永久删除订阅／历史、发布签名、公证和未实际验证的平台性能承诺。

## 研究依据

- 仓库 docs/04-specs/search.md、categories.md、reader.md、opml.md 和 docs/05-api/wails-bindings.md。
- https://www.sqlite.org/rowvalue.html ：行值比较支持滚动窗口键集查询。
- https://github.com/go-shiori/go-readability ：官方标记归档／弃用并指向 codeberg.org/readeck/go-readability/v2；实施前核对后继版本、许可证和离线 fixture。现有项目尚未引入旧提取器，迁移不是运行数据兼容变化。
- Wails 3.0.0 本地 pkg/application/dialogs.go；公开文档路径本轮未能访问，不以不可访问页面作为签名依据。

## 验收与回退

每部分运行必要的单元／真实 SQLite 集成、格式／类型／lint、生成绑定和构建；相关并发／取消测试运行 race。禁止默认 E2E，不使用用户生产库测例，不依赖真实付费／用户网络。涉及 schema 时新增迁移，不修改已应用文件；保留自动一致快照，失败前向修复或恢复兼容备份，不破坏性降级。每部分为清晰 Git 回退点，不自动推送。
