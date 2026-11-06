# 日志与执行审计

状态：已评审代码契约；Phase 0 已有基础工程，涉及其余领域的规则仍待对应阶段实现。

## 1. 范围与触发条件

后台操作、模型、MCP、凭据或数据库记录时。

## 2. 签名

结构化字段 `level,component,operation_id,run_id?,code?,duration_ms`；级别 debug/info/warn/error。

## 3. 契约

拟采用 Go slog；生产默认 info。debug 不记录原始凭据、完整上下文与内容；工具审计与诊断日志分开，用户可清历史。

## 4. 校验与错误矩阵

密钥／Authorization／Cookie／secret query→必须脱敏；外部写超时→OutcomeUnknown，不能写 success。

## 5. 良好、基础和异常案例

良好：记录安全 code 和耗时；基础：普通完成 info；异常：认证失败只记服务 ID 与错误类别。

## 6. 必需测试

对嵌套 header／env／body 的日志脱敏断言；并发日志含正确 run_id；清历史不删文章。

## 7. 错误与正确做法

错误：输出整个请求结构。正确：显式白名单字段与敏感值脱敏。

权威详细设计：[架构](../../../docs/03-architecture/security.md)。

Chat 只读工具执行状态由 00013 持久化，使用助手消息 ID 与调用编号摘要定位。开始记录成功才读文章，终态记录失败时结束模型续轮。原始编号、工具参数及返回正文不写入活动记录。恢复事务同时关闭遗留消息与工具活动。
