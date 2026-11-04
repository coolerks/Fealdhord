# 前端目录与边界

状态：已评审代码契约；Phase 0 已有基础工程，涉及其余领域的规则仍待对应阶段实现。

## 1. 范围与触发条件

创建 feature、组件或绑定适配时。

## 2. 签名

`frontend/src/features/<feature>`；`lib/api` 是唯一绑定适配入口，具体生成路径 Phase 0 确认。

## 3. 契约

app 放组合，features 按领域，components/ui 放基础控件，stores 只放 UI，styles 放 token；无产品代码时不得引用虚构现有组件。

## 4. 校验与错误矩阵

跨组件直接解析厂商 SSE／RSS／SQL→边界检查失败。

## 5. 良好、基础和异常案例

良好：Reader 用 api.getArticle；基础：本地表单状态；异常：组件持久保存 API Key，必须移入 Go 凭据服务。

## 6. 必需测试

模块依赖、实际绑定集成、生成 DTO 序列化与基础 build。

## 7. 错误与正确做法

错误：所有业务塞 App.tsx。正确：按 feature 分层，小组件按职责组合。

权威详细设计：[架构](../../../docs/03-architecture/frontend.md)。

## Phase 0 真实依据

[基础契约](../../../docs/04-specs/bootstrap.md)记录实际服务、DTO、事件合并、测试与生成命令。生成绑定不能手改；早到乱序事件使用同一个 sequence 合并器，类型与构建测试必须采用兼容的 Vite／Vitest 版本。
