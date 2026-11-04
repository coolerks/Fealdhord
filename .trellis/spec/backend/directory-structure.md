# 后端目录与依赖边界

状态：已评审代码契约；Phase 0 已有基础工程，涉及其余领域的规则仍待对应阶段实现。

## 1. 范围与触发条件

新增 Go 包、服务或跨模块调用时。

## 2. 签名

应用入口逻辑形如 `List(ctx context.Context, query ArticleQuery) (ArticlePage, error)`；对外绑定使用具体 DTO。

## 3. 契约

internal/app 负责装配；feed/article/category/search/reader 负责领域；database 负责 SQL；ai 的 Adapter 包持有 SDK 类型。生成代码单独目录，不手改；目录详细方案见 docs/03-architecture/backend.md。

## 4. 校验与错误矩阵

循环 import／业务直接 import 厂商模型类型：设计检查失败，调整接口；新增万能 Manager 不接受。

## 5. 良好、基础和异常案例

良好：Tool 经小接口调用 ArticleService；基础：main 只组合依赖；异常：ArticleService 调 Agent 再递归调用自己，应拆界。

## 6. 必需测试

依赖检查无循环；最小真实绑定往返；错误包边界有单元断言。

## 7. 错误与正确做法

错误：React → SQL 或业务返回 SDK 对象。正确：绑定 → 应用服务 → 仓储／Adapter，稳定 DTO。

权威详细设计：[架构](../../../docs/03-architecture/backend.md)。

## Phase 0 真实依据

[基础契约](../../../docs/04-specs/bootstrap.md)记录实际服务、DTO、事件合并、测试与生成命令。生成绑定不能手改；早到乱序事件使用同一个 sequence 合并器，类型与构建测试必须采用兼容的 Vite／Vitest 版本。
