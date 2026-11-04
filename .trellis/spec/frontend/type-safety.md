# TypeScript 与边界数据

状态：已评审代码契约；Phase 0 已有基础工程，涉及其余领域的规则仍待对应阶段实现。

## 1. 范围与触发条件

任何 DTO、外部配置、事件或持久状态变更。

## 2. 签名

strict=true；外部入口 unknown，经统一 validator 解码；事件使用 type 判别联合。

## 3. 契约

生成 DTO 不手改，不用 any 绕过类型；ID 字符串、时间 RFC3339，snake_case wire 到局部映射只在 api 边界。

## 4. 校验与错误矩阵

未知 schema→重新获取或提示不支持；未知枚举→校验错误；unsafe HTML→不能注入。

## 5. 良好、基础和异常案例

良好：switch 穷尽事件类型；基础：可选 usage=null；异常：把未知对象 cast 成成功结果。

## 6. 必需测试

typecheck、序列化 roundtrip、非法 payload、未知事件和旧 settings 迁移。

## 7. 错误与正确做法

错误：组件各自 as 同一原始事件。正确：单一类型定义与 decoder，渲染只消费 typed projection。

权威详细设计：[架构](../../../docs/03-architecture/frontend.md)。

## Phase 0 真实依据

[基础契约](../../../docs/04-specs/bootstrap.md)记录实际服务、DTO、事件合并、测试与生成命令。生成绑定不能手改；早到乱序事件使用同一个 sequence 合并器，类型与构建测试必须采用兼容的 Vite／Vitest 版本。
