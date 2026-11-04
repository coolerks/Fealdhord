# 内置工具规格

Phase 8，依赖 Runtime；只规划工具，不允许绕过权限直接调用。JSON 输入拒绝未知字段，分页 limit 默认 50、最大 200，批量 IDs 最大 500。Tool Result／审批遵循 [契约](../05-api/tool-contract.md)。

| 工具 | 主要输入 | 输出／效果 | 风险 |
| --- | --- | --- | --- |
| feed.list | cursor?,limit?,inactive_since? | Feed 摘要页，含 last_success_at／last_error | read |
| feed.get | id | Feed 详情 | read |
| feed.add | url,category_ids? | 新建或恢复 Feed | write |
| feed.update | id,revision,patch | 仅名称／URL／分类白名单 | write |
| feed.remove | id,revision | 归档，不删历史文章 | destructive |
| feed.refresh | id | operation_id | write+external |
| feed.refresh_all | 空对象 | operation_id | write+external |
| feed.discover | url | 候选 Feed | read+external |
| category.list | parent_id?,cursor?,limit? | 分类页 | read |
| category.get | id | 分类详情 | read |
| category.create | name,parent_id?,description? | 新分类 | write |
| category.update | id,revision,patch | 分类白名单更新 | write |
| category.remove | id,revision | 删分类关系，不删文章 | destructive |
| category.add_feed | category_id,feed_id | 建立继承关系 | write |
| category.remove_feed | category_id,feed_id | 移除继承关系 | write |
| article.list | ArticleQuery | 文章摘要页 | read |
| article.get | id | 有界正文和引用 | read |
| article.search | ArticleQuery | 搜索页 | read |
| article.mark_read | ids | SetRead(true) | write |
| article.mark_unread | ids | SetRead(false) | write |
| article.favorite | id | SetFavorite(true) | write |
| article.unfavorite | id | SetFavorite(false) | write |
| article.add_category | article_id,category_id | manual included | write |
| article.remove_category | article_id,category_id | manual excluded | write |
| app.get_settings | 空对象 | 主题／阅读／布局安全白名单 | read |

“半年未更新”需区分 last_success_at 和文章最近发布时间；错误源不等于停止更新源，Agent 结果须解释判断依据。删除批次通过运行时生成具体审批清单，不引入不受控任意 SQL／shell 工具。允许用户撤销已读／收藏，撤销必须写真实数据。

## 验收

- TOOL-01：以上 25 个工具均有 Schema／风险／结果与 fixture，调用正确应用服务；app.get_settings 不暴露密钥／路径秘密。
- TOOL-02：read 默认自动，write 可配置 auto/ask/deny，destructive 默认确认；Skill 与 MCP 不可提升策略。
- TOOL-03：目标不存在／revision 变化／批量超限无隐藏副作用；历史记录工具来源、参数摘要、权限、结果、耗时与错误。
