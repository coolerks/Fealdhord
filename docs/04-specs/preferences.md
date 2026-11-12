# 界面偏好持久化

## 1. 范围

Phase 1 提前引入最小 SQLite 设置表，以满足主题、双视图和布局重启恢复；不代表 Phase 2 文章数据库已完成。

## 2. 签名

Settings Service：Get(context.Context) Result；Update(context.Context, UpdateRequest) Result。UpdateRequest={preferences,revision}，Result 成功 data={preferences,revision}，失败安全 AppError。

## 3. 契约

migrations/00001_ui_preferences.sql 通过 goose 创建 app_settings，sqlc v1.30.0 生成读取与 CAS 更新。modernc SQLite v1.61.1、goose v3.28.0。数据文件在 os.UserConfigDir()/Fealdhord/fealdhord.db，单写连接，foreign_keys、WAL、busy_timeout=5000。启动失败不假装偏好已保存，Get 返回中文 DatabaseError。

偏好覆盖 Theme、article_view_mode、list_density、masonry_image_mode、animation、follow_system_reduce_motion、sidebar_collapsed／width、list_width、reader_open／reader_layout、selected_scope／article_id、scroll_anchors、Reader 字体／宽度。scroll_anchors 同时保存浏览 ID 锚点和 reader|article_id 的 Reader 垂直偏移，合计最多 32 项。旧版 list_thumbnail 字段保留读写兼容，列表按图片策略显示可用首图。当前只恢复四个系统 scope，后续 Feed／Category 导航再扩展。

`preferences.reader_toolbar_pinned` 保存阅读工具栏常驻操作 ID 数组。允许 `refresh`、`favorite`、`translate`、`ai`、`categories`、`read`、`view`、`original`。默认是前四项，`[]` 表示全部放入「更多」。设置只控制常驻项，排列顺序由界面统一定义。字段由既有 Get／Update 契约和 CAS 持久化，无需数据库迁移。

`reader_image_wheel` 允许 `zoom`（默认缩放）或 `navigate`（切换图片）。布尔字段 `reader_image_backdrop_close` 默认开启，控制点击图片周围空白处是否退出看图。 `reader_outline` 允许 `ticks`（默认刻度）、`auto`、`fixed`、`hidden`。旧设置缺少字段时按默认值读取。更新时非法枚举返回 `ValidationError`。

通知方式 `preferences.notification_mode` 接受 `toast` 或 `native`，默认 `toast`。布尔值 `preferences.refresh_toast_auto_dismiss` 默认 `false`，旧记录缺少该字段时同样为 `false`。两个值由现有 Get／Update 契约和 CAS 保存，无需迁移。

`preferences.developer_mode` 是「关于」页的布尔开关。默认及旧记录缺字段均为 `false`，沿用 Get／Update 与 CAS。macOS 菜单读取已提交偏好。Windows／Linux 菜单读取工作区当前偏好。

## 4. 错误

非法枚举／尺寸／超限锚点→ValidationError，零写入；旧 revision→Conflict，原子拒绝；未来 schema→ConfigurationError；I/O→DatabaseError。读取旧字段分别回退安全默认，不覆盖有效字段；未知未来 schema 不允许旧客户端写回。常驻项含未知 ID、重复 ID 或超过八项时拒绝写入，旧记录中的无效值和缺失值分别恢复默认。系统通知授权被拒或请求失败时保留当前方式，发送失败时回退到应用内通知。开发者模式保存冲突不触发原生菜单更新。

## 5. 案例

正常：保存 Dark＋Masonry，关闭数据库重开仍一致。基础：新库默认 System＋List。异常：两个窗口以同 revision 更新，只能一个成功，不静默覆盖。常驻项全部取消后重启，仍只显示「更多」。订阅刷新结果默认保留，开启自动收起后，新到达的结果采用普通 Toast 的 5 秒计时。开启开发者模式后，「帮助」出现「开发者工具」，关闭后隐藏。

## 6. 测试

临时真实 SQLite 测试重开、并发 CAS、非法输入、旧字段恢复与未来 schema 拒绝读写；使用 -race 检查，不访问用户数据目录，不执行 E2E。前端保存失败、快速连续更改和初次恢复需组件／单元验证。阅读工具栏覆盖默认项、搜索勾选、动作移动、CAS 保存、空数组读回和无效 ID 拒绝。通知覆盖设置保存与重开、旧记录默认值、订阅终态专用超时和系统发送失败回退。开发者模式覆盖默认关闭、保存读回、CAS 冲突及两次切换的菜单通知。

## 7. 实现约束

错误：用 localStorage 另存一套持久事实，或用无 revision 的整对象覆盖。正确：SQLite 单一事实、Go 验证、串行保存最新偏好，冲突明确提示并恢复一致性。

`reader_layout` 默认为 `fill`，可选 `drawer` 或 `split`。读取缺少该字段的旧记录时，仅将已开启 `reader_pinned` 的记录映射为 `split`。列表视图固定并排，瀑布流读取此偏好，兼容值 `split` 按抽屉显示。


## 订阅刷新与通知

设置的「订阅」页保存自动刷新间隔 `refresh_interval_minutes`，默认为 30 分钟。「通知」页用分段控件切换应用内通知与系统通知，并提供独立的订阅更新通知自动收起开关。系统通知由 Wails 请求授权。刷新进度和可重试的表单错误仍显示在操作处。
