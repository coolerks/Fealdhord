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
