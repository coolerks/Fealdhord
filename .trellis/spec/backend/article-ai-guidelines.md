# 文章 AI 的数据库与模型边界

## 范围
适用于分类、精选、试跑及其后台调度。

## 签名
`ArticleAISnapshot(ctx,id,task,preview)` 提取有界输入。 `ReserveArticleAI(ctx,snapshot,preview,estimate,now)` 在调用前核对快照并预留预算。 `CommitArticleAI(ctx,snapshot,scores,matches,input,output,now)` 复核后一次提交。

## 契约
网络请求在事务之外。快照含有序分类、配置版本、模型和 Provider 版本与文章内容哈希。成功仅替换 AI 来源，人工和订阅优先级继续复用有效成员视图。精选游标绑定当前配置和代次。

## 错误
缺启用或模型、预算已满、快照冲突分别拒绝调用或提交。概率缺失或越界、未知或重复分类 ID、截断和额外字段均视为无效输出，格式问题最多重试一次。

## 案例
模型返回期间正文变化，拒绝写入。人工排除已存在时，AI 命中仍不改变有效分类。试跑只消耗预算。

## 检查
使用临时 SQLite 和模拟模型覆盖禁用、缓存、人工排除、过期、非法输出、精选游标与预算窗口。

## 实现位置
数据层在 `internal/database/article_ai.go`，服务在 `internal/articleai`。不得从前端直接构造 AI 成员写入，也不得用精选结果创建普通分类。

预算及展示阈值更新只递增设置 CAS revision，保留 profile_revision。迁移 00012 将既有 revision 复制为 profile_revision，模型配置摘要保留旧字段序列化。精选阈值变更使旧游标过期，预算预留实时读取数据库限额。

## 文章摘要

### 范围

适用于阅读器的手动生成、打开时生成、SQLite 结果复用及短文阈值。

### 签名

`articlesummary.Service.Get(ctx, Source)` 与 `Generate(ctx, Request)` 接收文章 ID、`feed|reader` 正文来源和版本。迁移 00028 增加 `article_summaries`、摘要设置及 `feed_content_explicit`。交互和错误字段见 [桌面接口](../../../docs/05-api/wails-bindings.md#文章-ai)。

### 契约

读写按文章、来源、版本定位。显式订阅正文来源在订阅解析时记录，描述字段回落的内容不取得此标记。结果存为 `generated`、`skipped_model` 或 `skipped_length`。迁移 00031 为前两种记录输出目标语言，读取和并发合并只复用当前语言。第三种匹配本地阈值。模型调用前后复核来源，写入事务保护正文版本以及结果对应的语言或阈值。生成不使用工具，长文必须覆盖全部批次后提交。

### 校验和错误

未确认正文返回 `BodyUnavailable`，正文、目标语言或本地判断阈值变化返回 `Conflict`。模型截断、空生成、过度复述和高置信度的语言冲突按输出异常处理，有限重试后返回 `InvalidModelOutput`。超出完整处理预算返回 `ContextWindowExceeded`。失败保留重试机会。

### 案例

完整的普通短文零模型调用并保存跳过。只有较短的 RSS 摘录时等待网页正文。调低阈值后重新判断旧的短文跳过。同版本已有摘要时离线读取。

### 检查

用临时 SQLite 和模拟模型断言旧库升级、短文零调用、重启复用、阈值变更、并发合并、末段纳入及旧版本拒绝。用阅读器组件测试验证正文就绪后触发和键盘折叠。

### 实现位置

`internal/articlesummary` 负责正文判断与生成，`internal/database/article_summary.go` 负责版本校验与持久化。正文完整性由来源记录和正文状态共同判断。直接依据订阅模式返回的文本长度会把描述字段误当全文。

## 场景：文章机器翻译服务

### 范围

适用于 `internal/translation` 的厂商适配、凭据、正文进度、摘要和后台队列。

### 签名

`SaveProviderConfig(ctx, input)` 和 `TestProvider(ctx, id, draft)` 共用端点校验。`Translate(ctx, req)` 按 `articleId`、`targetLang` 和 `sourceMode` 归属任务。`Clear(ctx, articleID, targetLang)` 的空目标语言表示清除该文章的全部目标语言。

### 契约

已有凭据绑定已保存 HTTPS 来源，默认地址参与同源比较。轮换先写独立凭据引用，再保存配置。保存失败时仅回收新引用。带密钥的请求拒绝跨来源和降级重定向。

摘要完成前逐项验证必需字段。正文 `checkpoint_json` 按原文、引擎和分片计划复用，只有结构校验通过的片段可写入。清除先取消对应任务再删除记录，取消处理只使用最后一次提交的快照。百度的原批次和单段补译共用同一限流器。

### 校验与错误

非法端点返回 `InvalidEndpoint`，跨来源复用旧凭据返回 `CredentialRequired`。缺少必需译文返回厂商错误或 `InvalidModelOutput`。后台 `UnsupportedLanguage` 延后单篇文章，认证、配置或配额错误沿用服务退避。

### 案例

同一 HTTPS 来源变更路径可保留密钥，换端口需重新提供密钥。长段尾片失败后只重发尾片。标题成功且摘录失败时保留已有标题的展示，摘要仍可重试。

### 检查

用官方数字状态码 fixture 验证阿里云解析。用临时 SQLite、内存凭据和模拟 HTTP 验证凭据写入回滚、重定向、旧缓存恢复、限流和取消。用阅读器组件验证错误码对应的配置跳转。

### 实现约束

配置和草稿都通过来源校验后才读取可复用的密钥。厂商响应和正文标记都必须通过校验才推进完成状态。取消的请求沿原上下文停止写入。
