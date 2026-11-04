# 关键数据流

## 刷新与首图

到期队列／用户刷新 → 注册 operation_id → 受控 HTTP 条件请求 → 有界 Parser → Normalizer（时间／URL／身份／图片候选）→ 单事务 upsert Article／identities／FTS／分类继承 → 提交 → domain.changed → Query 失效。首图探测作为可取消低优先级任务，在提交后更新媒体；失败不能回滚文章入库。

304 不解析正文、不重复入库，更新检查与成功时间；只有完整成功解析并提交后才能更新持久 ETag／Last-Modified，防止解析失败后一直 304 丢数据。

## 阅读与状态

选择 article_id → 获取已清理内容 → Reader 呈现成功 → 幂等 SetRead(true) → 提交 → 列表／瀑布流／计数同步。切换条目时取消旧内容请求，旧响应不能覆盖新选择。取消收藏或已读失败恢复乐观缓存。

## AI 与权限

UI 订阅事件 → StartRun 返回 run_id → ContextBuilder → Model → 完整工具参数 → Schema／权限 → Pending Action → 用户确认特定目标 → revision 复核 → 应用服务事务 → tool result → Model。事件丢失通过快照恢复，不重放副作用。

## 取消与重启

CancelOperation 是幂等请求，context 贯穿 HTTP／模型／数据库等待。只停止未提交工作，已提交写入记录为完成。应用重启把非终态 run 标为 interrupted，不自动继续危险操作。
