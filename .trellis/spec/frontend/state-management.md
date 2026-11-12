# 状态所有权与恢复

状态：已评审代码契约；Phase 0 已有基础工程，涉及其余领域的规则仍待对应阶段实现。

## 1. 范围与触发条件

新增持久偏好、筛选、缓存或乐观 mutation 时。

## 2. 签名

UI 状态 `articleViewMode/listDensity/listThumbnail/masonryImageMode`；锚点 `{article_id,offset}`；持久设置含 schema_version。

## 3. 契约

SQLite 是事实源；TanStack Query 管领域缓存；Zustand 管 UI 并用 selector；局部状态不泛化为全局。持久偏好由 Go SettingsService 保存。

## 4. 校验与错误矩阵

非法旧枚举→安全默认；mutation 失败→回滚；游标过期→重查保留锚点。

## 5. 良好、基础和异常案例

良好：跨视图收藏一致；基础：System 主题；异常：Zustand 复制 100k 文章导致分叉。

## 6. 必需测试

重启恢复、主题跟随、状态回滚、图片到达锚点偏差、scope 切换不混数据。

## 7. 错误与正确做法

错误：用 scrollTop 作为所有布局唯一锚点。正确：article_id＋offset，布局测量后定位。

权威详细设计：[架构](../../../docs/03-architecture/frontend.md)。

## Phase 1 真实依据

workspace.initialize 仅在首次偏好恢复／显式重载时调用；patch 用 PreferenceWriter 串行保存，并合并等待中的最新版本，SQLite revision 只在成功后推进。saveStatus 区分 saving／saved／error，不把乐观界面当落盘成功。冲突不能自动覆盖，允许显式重载。scroll_anchors 共用最多 32 项，Reader 键为 reader|article_id；布局锚点必须以 ID＋offset 恢复，恢复期间不捕获顶部，退出时冲刷防抖值。

## Phase 2 真实依据

library Query 现在保存 Page={items,hasMore}，生产从 Go 读取，不再是固定空数组；Reader 正文按 article_id 独立缓存，列表不得加载全部正文。文章状态由 article-state 串行落盘，失败只把最新意图回滚到最后保存值，不能回滚到另一个失败请求的乐观值。RFC3339 不同小数精度不能直接字符串排序，比较真实时间。hasMore 必须提示加载范围，不能把当前有界页冒充全库。

## Phase 4 分页缓存

library Query 迁移为 InfiniteData<LibraryPage>；libraryKey 包含 scope／search／read／favorite／sort／limit，cursor 是 pageParam，不是独立键。默认全库时间倒序50条保留基础 articleQueryKey。只能通过 libraryArticles／findCachedArticle／updateCachedArticle 访问跨页缓存；文章状态串行写入同步所有相关文章页，过滤页失效重取，不复制到 Zustand。搜索防抖150ms，signal 取消 Wails CancellablePromise；旧响应留在旧键，不覆盖新查询。命令读取 activeArticleQueryKey 的已加载结果，切换 list／masonry 不变查询键。

分类与订阅选择保存 selected_scope／selected_scope_id；导航 Query=['navigation'] 来自独立全库计数，禁止从分页数据计数。readFilter／favoriteFilter 为短期 UI 状态并进入查询键。滚动锚点键使用 libraryAnchorKey 的有界64位UI哈希，不能把512字搜索 JSON 直接写入≤512字节的偏好键；此哈希不是安全游标签名。

侧栏与系统视图命令共用 `selectScope(scope, id?)`。瀑布流使用铺满阅读布局时，在更新 `selected_scope`／`selected_scope_id` 的同一笔偏好中将 `reader_open` 设为 `false`，使目标查询结果立即可见。

## 阅读工具栏常驻偏好

`reader_toolbar_pinned` 通过既有 SettingsService 和 `workspace.patch` 持久化。偏好包含稳定操作 ID。空数组表示全部收进菜单。读取旧记录时，缺失或无效字段使用默认四项。字段定义、写入校验与测试见[偏好契约](../../../docs/04-specs/preferences.md)。

通知方式和订阅结果 Toast 的自动收起开关由「通知」设置页通过 `workspace.patch` 独立保存。旧记录缺少 `refresh_toast_auto_dismiss` 时按 `false` 读取。Toast 超时是单条通知的参数，全局 Provider 默认值继续供其他通知使用。
