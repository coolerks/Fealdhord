# Hook、查询与订阅

状态：已评审代码契约；Phase 0 已有基础工程，涉及其余领域的规则仍待对应阶段实现。

## 1. 范围与触发条件

新增 use*、异步调用或事件订阅时。

## 2. 签名

`useArticles(query)` 共用一份 Query；query key 含 scope/filter/sort/search，不含 list/masonry。

## 3. 契约

所有 Hook 顶层调用；异步通过统一 API；事件卸载清理；context／operation ID 支持取消；不要为 StrictMode 重复 effect 触发副作用。

## 4. 校验与错误矩阵

重复订阅→检查失败；丢事件→重取 snapshot；旧搜索响应→丢弃。

## 5. 良好、基础和异常案例

良好：query key 变更取消旧查询；基础：空 items=[]；异常：重挂载再次执行删除，应移到显式 mutation。

## 6. 必需测试

卸载无泄漏、StrictMode 不重复提交、query 参数隔离、快照恢复和取消。

## 7. 错误与正确做法

错误：用 effect 同步两份文章数组。正确：同一 Query 数据投影两视图，选择器只派生视图状态。

权威详细设计：[架构](../../../docs/03-architecture/frontend.md)。

## Phase 0 真实依据

[基础契约](../../../docs/04-specs/bootstrap.md)记录实际服务、DTO、事件合并、测试与生成命令。生成绑定不能手改；早到乱序事件使用同一个 sequence 合并器，类型与构建测试必须采用兼容的 Vite／Vitest 版本。
