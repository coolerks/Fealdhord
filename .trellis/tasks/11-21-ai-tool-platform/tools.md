# 应用工具清单

状态：本任务交付清单。逻辑名采用点分隔，由注册表生成模型和 MCP 各自允许的 wire 名并维护稳定映射。Chat 只装入当前会话有机会使用的工具，MCP 仅发布当前客户端获准的工具与资源。两端执行进入同一服务层。表中「授权」指用户或客户端已明确授权对应范围；文章正文、批量数据、计费调用单独核验。

| 领域 | 工具 ID | 主要参数及返回 | 默认权限 / 复用 |
| --- | --- | --- | --- |
| 文章 | `article.list`、`article.search` | 分类/订阅、关键词、已读/收藏、精选、排序、游标；摘要、匹配片段、稳定文章 ID | 授权读；Library Query/FTS |
| 文章 | `article.get`、`article.get_metadata` | 文章 ID；分别返回有界纯文本正文和详情元数据，带引用、版本与截断提示 | 元数据授权读，正文限当前文章/本轮选中/用户授权命中；Reader、Library |
| 文章 | `article.extract` | 文章 ID 与 revision；按已存原文地址重新提取，返回提取状态和正文引用 | 外部写确认；Reader 受控网络 |
| 文章 | `article.mark_read`、`article.mark_unread` | IDs 和待确认的目标；返回逐项结果 | 写确认；Library 状态 |
| 文章 | `article.favorite`、`article.unfavorite` | 文章 ID；返回实际收藏状态 | 写确认；Library 状态 |
| 文章 | `article.add_category`、`article.remove_category` | 文章 ID、分类 ID；返回实际成员关系 | 写确认；Navigation，复用手动覆盖 |
| 订阅 | `feed.list`、`feed.get` | 订阅 ID／游标／归档状态／活跃度；返回标题与可观察的刷新状态 | 授权读；Feed。区分最后刷新成功和最近文章发稿时间，缺失字段按未知或补查询 |
| 订阅 | `feed.discover` | 公共网页 URL；返回候选订阅 | 仅允许用户授权的公共 URL；Feed Discover |
| 订阅 | `feed.add`、`feed.update` | URL、标题、分类、可安全编辑的图标选项、revision；返回保存的订阅 | 写确认，网络走原受控网关；Feed |
| 订阅 | `feed.remove`、`feed.restore` | ID、revision；归档或恢复订阅，返回真实状态 | 归档始终确认，恢复默认确认；Feed |
| 订阅 | `feed.refresh`、`feed.refresh_all` | 指定 ID 或全部；返回操作 ID | 外部写确认；Refresh |
| 订阅 | `feed.refresh_status`、`feed.cancel_refresh` | 操作 ID；返回进度／取消结果，只允许调用者有权查看的操作 | 读／写确认；Refresh |
| 分类 | `category.list`、`category.get` | ID、父级、分页；返回可见树、计数或单项 | 授权读；Navigation |
| 分类 | `category.create`、`category.update` | 名称、父级、说明、revision；返回实际修改 | 写确认；Navigation |
| 分类 | `category.preview_remove`、`category.remove` | 分类 ID；预览子项、订阅和文章关系的影响，确认后删除分类关系 | 预览授权读，删除始终确认；Navigation CAS |
| 分类 | `category.add_feed`、`category.remove_feed` | 分类 ID、订阅 ID、revision；返回订阅继承关系 | 写确认；Feed CAS，保留其他分类 |
| 分类 | `category.article_memberships` | 文章 ID；返回有效成员以及可见来源 | 授权读；Navigation |
| 智能视图 | `view.list`、`view.get_counts` | 返回现有全部/未读/收藏/精选与可见计数 | 授权读；Navigation、Library；文章数据仍经 article 查询 |
| 文章 AI | `article_ai.preview`、`article_ai.evaluate` | 文章 ID、分类/精选用途；返回预算和结果状态 | 两项均涉及模型调用时显式确认，后者可能写入；复用 ArticleAI 缓存、预算和版本 |
| 导入导出 | `opml.preview_import`、`opml.import`、`opml.export` | 前者要求桌面文件授权，导入确认预览范围，导出要求桌面选择保存位置；返回结果摘要 | 桌面授权，导入/导出确认；Transfer，MCP 仅在确有本地授权时允许 |
| 应用 | `app.get_settings`、`app.update_reading_settings` | 只读/修改显示、主题、阅读排版白名单；写入须校验 revision | 授权读／写确认；Settings |
| 应用 | `app.get_refresh_settings`、`app.update_refresh_settings` | 刷新间隔及可公开的调度状态；修改刷新间隔经设置校验 | 授权读／写确认；Settings |
| 应用 | `app.get_version`、`app.get_diagnostics` | 版本与脱敏状态，例如待刷新数量、最近错误码 | 授权读；App、Refresh |

以上含既有 25 项规格与 19 项扩展，共 44 个逻辑工具。上下文、Skills 和 MCP 的管理由用户操作；运行时只消费用户已授权的 Skills／MCP 工具。自然语言总结和解释可用文章搜索、读取和引用直接完成。相同业务能力同时采用按工具和资源的授权，例如 `fealdhord://article/{id}` 与 `article.get` 必须执行相同正文范围校验。配置维护和文件选择通过安全的桌面交互，工具结果不回传凭据、本地文件路径或其他会话内容。

## 共通验收

- 每个工具必须记录实际实现、Schema 样例、正常/非法/越权回归；结构化结果含业务引用及状态。列表默认 50、最大 200，正文按 token 和字节双限，批量 ID 最大 500 并要求明确上限。
- 真实库测试覆盖添加／更新／归档／恢复订阅、操作取消、分类关系、已读和收藏。审批显示具体实体、影响和费用，用户确认后单次执行。失败、取消、过期、重复及目标版本变更均有独立断言。
- 每次网络路由到真实服务前遵守受控 URL 和认证策略。外部写操作失败且结果不明时显示「结果未知」，保留审计；外部客户端不能决定自身权限。
