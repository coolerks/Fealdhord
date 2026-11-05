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

## 原生体验修订

默认中性灰白，不再使用绿色主题；默认双栏＋Reader 聚焦，reader_pinned 才三栏。AnimatedGlyph 只在完整动态交互中挂载库实例，fallback 与动画 SVG 形状一致，完整父按钮触发；异步多段动画必须可中断。全局右键不可禁用文本选择；文本菜单使用打开前快照，异步剪贴板返回时核对整段原值。具体签名见[原生体验契约](../../../docs/04-specs/native-experience.md)。

## 界面减负与三方体验

遵守 AGENTS 两条规则：删除重复标语／视图解释／开发阶段说明，以操作层级与状态表达含义；错误、真实加载范围和不可用状态仍要明确。标题、图标按钮及命令名称必须匹配实际行为，保留语义角色、label 和焦点策略；不得为了少文字删掉无障碍名称。空态由 LibraryEmpty 统一处理，避免 Shell 多层条件及无效 CSS。窄分栏工具区通过容器查询重排，不用说明弥补拥挤布局。

cmdk 的 CommandInput 自带 aria-labelledby，可覆盖孤立 aria-label；使用 Command 的 label 属性给生成标签赋值，并用 getByRole('combobox', {name:'搜索命令'}) 验证真实可访问名称。仅在 JSX 中看见 aria-label 不能证明 Agent 能识别控件。文案与角色语义变化同步命令面板／原生菜单，CommandID 与数据接口保持稳定。
