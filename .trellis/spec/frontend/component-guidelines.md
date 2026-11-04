# 组件、视觉与可访问性

状态：已评审代码契约；Phase 0 已有基础工程，涉及其余领域的规则仍待对应阶段实现。

## 1. 范围与触发条件

创建／修改可交互组件、列表、Reader 或图标时。

## 2. 签名

Props 明确类型；article_id 为条目 key；按钮有 type、accessible name、Focus-visible。

## 3. 契约

只消费规范化 DTO；语义 token；Hover／Press／Focus／Disabled 完整，触控可用；关键动作不只 hover 可见。

## 4. 校验与错误矩阵

缺 label／键盘路径→验收失败；图片失败→无图回退；旧响应→不能覆盖新选择。

## 5. 良好、基础和异常案例

良好：收藏星形语义反馈；基础：无图文字卡；异常：动画 Off 后残留遮罩，不可接受。

## 6. 必需测试

键盘／Focus／触控、两主题、无图／失败、Reduced Motion、快速重复点击和真实副作用回滚。

## 7. 错误与正确做法

错误：所有图标统一缩放或 useEffect 为每卡建动画实例。正确：统一语义 primitives，简单状态 CSS，复杂面板 motion。

权威详细设计：[架构](../../../docs/03-architecture/frontend.md)。
