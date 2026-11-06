# 数据库模型与迁移

状态：Phase 2 核心表已落地，AI 与扩展表仍待后续阶段。SQLite + database/sql、modernc v1.61.1、goose v3.28.0、sqlc v1.30.0；00001 设置、00002 核心领域、00003 FTS。实际边界见 data-foundation.md。

## 通用约定

领域 ID 是 Go 生成的不透明字符串，排序不依赖 ID 代表时间；文章另有内部 INTEGER rowid 供 FTS 关联。时间持久化 UTC Unix 毫秒，API 输出 RFC3339；原始发布日期可为空。布尔用 INTEGER CHECK IN (0,1)，概率 REAL CHECK 0..1，JSON 写前校验。用户可见名不作主键。

每条连接启用 foreign_keys；WAL、busy_timeout=5000ms，当前单连接串行写入与有界读取，未来按实际并发证据扩展读池。设置必须逐连接生效。事务内不进行网络或模型调用；取消前已提交的事务不自动回滚。

## 核心表

| 表 | 关键字段与约束 |
| --- | --- |
| feeds | id PK，title，url，normalized_url UNIQUE，site_url，description，icon_url，custom_icon_url，etag，last_modified，last_checked_at，last_success_at，next_check_at，error_count，last_error，created_at，updated_at，archived_at，sort_position，revision |
| articles | rowid INTEGER PK，id UNIQUE，feed_id FK，guid nullable，title，url，normalized_url nullable，author，summary，content_html，content_text，cjk_terms，published_at nullable，updated_at nullable，fetched_at，sort_at，content_hash，dedup_key，revision；UNIQUE(feed_id,dedup_key) |
| article_identities | article_id FK，feed_id FK，kind，value；UNIQUE(feed_id,kind,value)；保留历次 GUID／URL 别名 |
| categories | id PK，parent_id nullable FK，name，normalized_name，icon，color，description，ai_enabled，ai_prompt，ai_threshold，created_at，updated_at，sort_position，revision；同父级名称唯一，根级使用独立唯一索引 |
| feed_categories | feed_id + category_id 复合 PK，created_at |
| article_categories | article_id + category_id + source 复合 PK，source CHECK manual/feed/ai，state CHECK included/excluded，confidence nullable，reason nullable，model_ref nullable，created_at，updated_at |
| reading_states | article_id PK FK，is_read，read_at nullable，updated_at，revision |
| favorites | article_id PK FK，created_at |
| article_media | article_id PK FK，url nullable，source nullable，status，width nullable，height nullable，alt，checked_at，retry_after，cache_key nullable |
| reader_contents | article_id PK FK，sanitized_html，content_text，source_url，fetched_at，extractor_version |
| app_settings | key PK，value_json，schema_version，updated_at |

`sort_at=published_at ?? fetched_at` 写入后稳定，不因重复刷新改变无日期文章顺序。索引 `(feed_id, sort_at DESC, id DESC)` 与 `(sort_at DESC,id DESC)`，分类反向索引、reading_states 状态索引。全文使用 article rowid 对应外部内容 FTS，title／author／summary／content_text／cjk_terms；新增、更新、删除必须同事务维护，提供重建与一致性检查。

## AI 与扩展表

| 表 | 最少字段与约束 |
| --- | --- |
| ai_providers | id，name，protocol CHECK 三种，base_url，credential_ref，enabled，options_json，revision，created_at，updated_at |
| ai_models | id，provider_id FK，model，display_name，capabilities_json，overrides_json，enabled，created_at，updated_at；UNIQUE(provider_id,model) |
| model_assignments | task_type PK，model_id FK；chat/classification/summary/translation/featured |
| classification_results | article_id，content_hash，category_set_hash，provider_revision，model_ref，options_hash，classification_version，result_json，created_at；上述缓存键唯一 |
| article_scores | article_id + scoring_profile_hash 复合 PK，importance，relevance，model_ref，created_at |
| conversations / messages | conversation id、scope_type、scope_id；message id、conversation_id、role、content_json、adapter_state_ref nullable、created_at |
| agent_runs | id，conversation_id，status，max_turns，max_tool_calls，deadline_at，usage_json，error_code，started_at，finished_at |
| tool_calls | id，agent_run_id，call_id，tool_name，tool_source，input_json，result_json，status，permission_json，started_at，finished_at，error_code；UNIQUE(agent_run_id,call_id) |
| pending_actions | id，run_id，tool_call_id，target_revision，args_hash，policy_revision，expires_at，status；单次消费 |
| mcp_servers | id，name，transport，config_json，credential_refs_json，enabled，permission_policy_json |
| skills | id，name UNIQUE，path，version，manifest_hash，enabled，source，updated_at |

历史输入和输出可能含用户数据，默认保存脱敏摘要；完整内容仅本机按用户选择保存，可清除。密钥、敏感 headers 和 env 不能因 JSON 扩展字段进入库。Skill 文件是内容源，表是索引，变化后重扫并校验。

## 去重与删除

同 Feed 优先 GUID，其次安全规范化 URL，最后正文指纹；已有任一稳定别名匹配则更新同 article_id，不重置收藏、阅读或手动排除。URL 不盲删 query；GUID 冲突但内容／URL 显著不一致隔离并记录，不覆盖旧文。跨 Feed 相同 URL 保留两条，不破坏来源。

删除订阅采用归档：移出订阅区并停止抓取，已有文章、收藏和搜索可保留，添加相同 normalized_url 恢复原 Feed。永久清理数据不在首版范围。删除分类只删除分类及关联，不删除文章；有子分类时提示迁移到父级并在一事务完成。

## 迁移与恢复

启动获取 gofrs/flock 进程锁后，待升级旧库使用 VACUUM INTO 一致快照创建备份，不能只复制活跃 WAL 数据库主文件。检查 schema 版本后运行嵌入 goose；失败禁止写入并保留原库／备份。发布后回退采用恢复兼容备份或前向修复；不依赖破坏性 Down。备份不包含 OS keyring 明文；导入后失效 credential_ref 要求重新配置。

Phase 2 必须验证：空库、旧版升级、迁移故障、外键、唯一约束、FTS 同步、并发刷新、备份恢复；schema 正文、迁移和 sqlc 输出保持一致。

验收 DB-01：真实 Go 驱动支持 FTS5，迁移／外键／唯一／事务／身份别名／来源优先级与一致备份恢复均有测试；未建 schema 前不得标通过。

## Phase 2 实际交付

[实现契约](../04-specs/data-foundation.md)记录真实方法、错误和有界读取；[验证记录](../09-research/phase2-validation.md)区分已验证的数据行为与仍未交付的完整分页／搜索 UI、RSS 和其他平台。Core 表、sqlc 与 FTS 特殊固定 SQL 均已存在。
