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
