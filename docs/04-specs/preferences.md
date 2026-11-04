# 界面偏好持久化

## 1. 范围

Phase 1 提前引入最小 SQLite 设置表，以满足主题、双视图和布局重启恢复；不代表 Phase 2 文章数据库已完成。

## 2. 签名

Settings Service：Get(context.Context) Result；Update(context.Context, UpdateRequest) Result。UpdateRequest={preferences,revision}，Result 成功 data={preferences,revision}，失败安全 AppError。

## 3. 契约

migrations/00001_ui_preferences.sql 通过 goose 创建 app_settings，sqlc v1.30.0 生成读取与 CAS 更新。modernc SQLite v1.61.1、goose v3.28.0。数据文件在 os.UserConfigDir()/Fealdhord/fealdhord.db，单写连接，foreign_keys、WAL、busy_timeout=5000。启动失败不假装偏好已保存，Get 返回中文 DatabaseError。

偏好覆盖 Theme、article_view_mode、list_density、list_thumbnail、masonry_image_mode、animation、follow_system_reduce_motion、sidebar_collapsed／width、list_width、reader_open／reader_pinned、selected_scope／article_id、scroll_anchors、Reader 字体／宽度。scroll_anchors 同时保存浏览 ID 锚点和 reader|article_id 的 Reader 垂直偏移，合计最多 32 项。当前只恢复四个系统 scope，后续 Feed／Category 导航再扩展。

## 4. 错误

非法枚举／尺寸／超限锚点→ValidationError，零写入；旧 revision→Conflict，原子拒绝；未来 schema→ConfigurationError；I/O→DatabaseError。读取旧字段分别回退安全默认，不覆盖有效字段；未知未来 schema 不允许旧客户端写回。

## 5. 案例

正常：保存 Dark＋Masonry，关闭数据库重开仍一致。基础：新库默认 System＋List。异常：两个窗口以同 revision 更新，只能一个成功，不静默覆盖。

## 6. 测试

临时真实 SQLite 测试重开、并发 CAS、非法输入、旧字段恢复与未来 schema 拒绝读写；使用 -race 检查，不访问用户数据目录，不执行 E2E。前端保存失败、快速连续更改和初次恢复需组件／单元验证。

## 7. 实现约束

错误：用 localStorage 另存一套持久事实，或用无 revision 的整对象覆盖。正确：SQLite 单一事实、Go 验证、串行保存最新偏好，冲突明确提示并恢复一致性。
