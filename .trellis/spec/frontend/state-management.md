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
