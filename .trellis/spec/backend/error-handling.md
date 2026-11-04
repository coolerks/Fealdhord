# 错误与取消

状态：已评审代码契约；Phase 0 已有基础工程，涉及其余领域的规则仍待对应阶段实现。

## 1. 范围与触发条件

所有 I/O、绑定和长任务。

## 2. 签名

`AppError={code,message,retryable,operation_id?,details?}`；所有内部 I/O 首参数 context.Context。

## 3. 契约

内部包装错误保留 cause，边界映射中文消息与安全细节；cancel 是明确终态，不吞错误。完整错误矩阵见 docs/05-api/wails-bindings.md。

## 4. 校验与错误矩阵

输入错→ValidationError；认证→AuthenticationError；限流→RateLimitError；超时→Timeout；取消→Cancelled。

## 5. 良好、基础和异常案例

良好：失败回滚缓存并保留旧内容；基础：空结果成功；异常：EOF 无协议终态不能当成功。

## 6. 必需测试

取消贯穿 HTTP／模型／等待事务；错误分类可 errors.Is/As；前端不收到 secret 或原始 Go stack。

## 7. 错误与正确做法

错误：`_ = save()` 或全部返回“失败”。正确：处理、分类、记录安全诊断，并明确重试语义。

权威详细设计：[架构](../../../docs/03-architecture/backend.md)。

## Phase 0 真实依据

[基础契约](../../../docs/04-specs/bootstrap.md)记录实际服务、DTO、事件合并、测试与生成命令。生成绑定不能手改；早到乱序事件使用同一个 sequence 合并器，类型与构建测试必须采用兼容的 Vite／Vitest 版本。
