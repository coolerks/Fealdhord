# 事件、快照与恢复契约

## 1. 范围与触发条件

Go 后台任务、Agent、刷新和 Query 缓存之间的通知。事件不是持久事实源。

## 2. 签名与信封

事件逻辑名 `fealdhord:event`；载荷 `{schema_version:1,event_id:string,operation_id?:string,run_id?:string,sequence:number,type:string,occurred_at:string,payload:object}`。Wails 事件订阅与注销的具体 API 在 Phase 0 验证。

## 3. 契约

| type | payload |
| --- | --- |
| operation.progress | status、completed、total?、feed_id? |
| domain.changed | entity_type、ids、revision、scope_ids? |
| run.model_event | 统一 ModelEvent |
| run.tool_state | call_id、tool_name、status、safe_summary |
| run.permission_required | action_id、tool_name、targets、expires_at |
| operation.terminal | status、error?、snapshot_revision |

sequence 在同一 operation 内递增；前端按 event_id／sequence 去重。订阅必须在 Start 前建立；早到事件按 run_id 有界缓存。token 流按短帧合并，保留顺序，不能为每字符使全应用重渲染。

快照是权威：`GetOperation/GetRun` 返回状态、完成调用、待审批和累积消息。事件缺号、窗口重载或重连时重新获取快照；domain.changed 只触发 Query 失效，不通过事件拼成数据库镜像。快照 revision 之前的事件丢弃，之后继续消费。内存队列有界；落后消费者可丢弃非终态进度并发送 resync_required，终态状态须持久保存。

## 4. 错误矩阵

未知 schema_version → 停止解释并重取；重复事件 → 忽略；乱序 → 按序处理或重取；run 不存在 → NotFound；应用退出 → 重启记录 interrupted。终态后到达的旧 text_delta 不恢复活动状态。

## 5. 案例

良好：stream 结束后快照和 UI 同为 succeeded；基础：空库刷新 total=0 后结束；异常：漏掉 sequence=3，UI 获取快照，不重发之前的工具请求。

## 6. 必需测试

早到／重复／丢失／乱序、订阅清理、快速切换 run、背压、终态只保存一次、窗口重载和取消后旧事件不污染当前阅读。

## 7. 错误与正确做法

错误：仅凭最后一条 token 判断完成。正确：明确协议终态 + 持久运行快照，通知可以丢，事实可以重查。
