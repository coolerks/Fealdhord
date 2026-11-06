# 全库文章查询与游标

## 1. 范围与触发条件

修改 ArticleQuery、分页、FTS 或索引修复时；普通查询与全文查询均由 database.QueryArticles 执行。

## 2. 签名

ArticleQuery={scope:{kind,id?},search?,read?,favorite?,sort,cursor?,limit?}；kind 为 all／unread／favorites／featured／feed／category，read 为 all／read／unread，sort 为 published_desc／published_asc／relevance。limit 默认 50，1..200。QueryPage 提供 Items、NextCursor、SnapshotID、SearchLimited；LibraryService.Query 投影纯文本摘要、FeedID／URL、UTF-16 TitleMatches。

## 3. 契约

输入先 NormalizeArticleQuery；搜索最长 512 个 Unicode 字符，字面编译，SQL 所有数据参数绑定。00006 的 search_state 为索引 generation；article_query_birth 记录出生代次，解决删除最高 rowid 后新文复用 rowid 的快照污染。普通游标绑定 rowid 上界和出生代次，以 sort_at／id 键集继续；全文额外比较 rank 和 generation。读取版本／边界／结果在同一短读事务，服务超时 5 秒。

游标 base64url＋HMAC，包含查询摘要、边界、位置、代次及 30 分钟有效期，进程内随机密钥，重启自然失效。签名不能作为前端可编辑分页参数。索引 rebuild 与 generation 递增同事务；常规 FTS 更新由触发器同步。

## 4. 校验与错误矩阵

非法 scope／枚举／limit／超长查询→ValidationError；篡改／过期／跨查询／全文索引变化→CursorExpired；取消／超时保持对应错误，不退化无界 LIKE。非空纯标点查询为空结果。中文连续片段必须复核，单次最多扫描 1000 候选；达到上限后提供继续游标与 SearchLimited，不假报穷尽。

## 5. 良好、基础和异常案例

良好：530 篇持续分页，无固定 500 上限，期间插入旧日期文章不混入旧快照。基础：空库和真正空结果。异常：删除最高 rowid 再插入，新文 rowid 复用仍由出生代次排除；索引重建后旧全文 rank 游标拒绝。

## 6. 必需测试

真实 SQLite 测连续分页、相同排序键、插入／删除与 rowid 复用、Feed／Category／状态过滤、中英混合、语法字符、游标签名和查询绑定、排名翻页／重建失效、候选上限后继续。DTO 测 UTF-16 命中偏移与安全错误。生成 sqlc／Wails 绑定，不手改输出。

## 7. 错误与正确做法

错误：从最新 500 篇本地筛选冒充全库，或把 max(rowid) 单独当永不复用的快照序号。正确：数据库查询与键集分页，出生代次防复用，候选截断明确提供继续路径。索引变动必须更新 generation，不能只重建 FTS 不使旧 rank 游标失效。

## 旧开发库兼容

迁移记账与实际结构可能因开发阶段脚本演进而不同。旧版 00006 已记账但缺少 article_query_birth 的库由 00008 补表并回填。已有记录保留出生代次，插入触发器统一为写入出生代次的版本。对历史结构修复使用新迁移，并验证真实旧库副本及已包含预期结构的库。
