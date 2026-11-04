# 前端架构

拟定 `frontend/src/app` 放启动与布局；`features/feed/articles/reader/categories/ai/settings` 放领域界面；`components/ui` 放 shadcn 基础组件；`components/motion` 放语义反馈；`lib/api` 放绑定适配；`stores` 放 UI 状态；`styles` 放主题和动效 token。禁止预创建无功能模块。

## 状态流

统一 ArticleQuery 包含 scope、筛选、排序、搜索词与 cursor。列表与瀑布流消费同一 `useArticles`；视图模式不加入数据查询键。Query 键必须包括所有影响后端结果的参数。收藏／已读可乐观更新相关缓存，失败回滚并提示；后台事件失效重取计数，不能由各行自行计算全局未读数。

Zustand 只存选择、布局、锚点、主题与偏好；禁止复制文章全集。持久偏好通过 SettingsService 写 SQLite。首屏从 Go 读取版本化偏好再恢复；旧／非法枚举回退默认并记录可诊断错误。瞬时 hover、对话框开关留在组件。

## 渲染与边界

业务组件只调用 lib/api；不解析 RSS、厂商 SSE 或 API Key。生成绑定 DTO 经窄适配映射领域类型；外部和版本化持久数据在边界验证。虚拟条目键为 article_id；图片和布局测量走统一组件。订阅事件在卸载时清理，运行状态断线后重查。凭据输入提交后清空，不进入 Query／Zustand／localStorage。

## Phase 0 实际结构（历史）

现有 src/App.tsx 为明确标注工程初始化的入口，src/lib/api.ts 是唯一绑定包装，operation-state.ts 统一合并 sequence，bindings 是固定生成输出。当前不引入尚无领域数据的 Query／Zustand；Phase 1 随正式 Shell 接入，不将五秒诊断当 RSS 刷新。

## Phase 1 实际结构

App 只组合 Query／Tooltip 与偏好恢复门；app/Shell、Sidebar、Toolbar 和按需 CommandPalette 负责阅读空间。features/articles／reader／settings 承载已实现界面，components/ui 为官方 shadcn Radix 组件，semantic-button／splitter 是项目交互封装，style.css 统一主题与动效 token。

lib/api 保留基础绑定，lib/preferences 包装设置绑定并复用 unwrap；stores/workspace 是 UI 状态及串行偏好保存协调器，lib/preference-writer 不持有领域文章。测试 fixture 与几何测量只在 src/test 内，生产 lib/library 当前为空数据入口。详细行为与失败边界见[Shell 契约](../04-specs/reader-shell.md)及[偏好契约](../04-specs/preferences.md)。

## Phase 2 数据接线

library Query 从 libraryAPI／LibraryService 读取 Page={items,hasMore}，不是固定空集合；列表不载入完整正文，Reader 按 ID Query 获取文本。article-state 按文章＋字段串行保存并维护最后成功值，旧失败不会回滚新意图。metadata 数组不进 Zustand；全库分页／后端筛选与完整搜索界面仍属 Phase 4。
