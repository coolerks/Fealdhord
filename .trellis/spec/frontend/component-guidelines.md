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

## 图标单次播放

AnimatedGlyph 仅交互期间挂载 lucide-animated；startAnimation／stopAnimation 返回完成 Promise，cancelAnimation 负责卸载与模式切换。不得以固定延时猜结束、mouseleave 才归位或 pointerdown 重复启动。多 controls 必须等待所有阶段；异步续段核对代次。静态 SVG 由 npm run icons:generate 从同份已安装库源生成，禁止另外选择不匹配的 fallback 图形；生成字符串仅来自仓库已审阅源，不接收外部 HTML。图标模式 Provider 覆盖全局菜单，Portal 继承同一偏好。


菜单项使用 Group／Separator 做语义分组，CommandPresentation／TextActionPresentation 消费 command-presentation 的图标与真实绑定，快捷键末尾对齐且 aria-hidden，条目自身提供 aria-keyshortcuts。右键目标与当前文章不同，不能显示 F／M 冒充该目标快捷键。结构图标也使用 AnimatedGlyph，不遗留静态 X／勾号／箭头。cmdk 焦点留在输入框，选项的键盘动画观察 data-selected，不能只监听 focusin。动画 CSS 不得叠加覆盖库的 transform。


StrictMode 下按需挂载的图标，播放在 useEffect 内排入可取消 queueMicrotask，等本轮 passive effects／副作用重放后才调用句柄。cleanup 先使 current 失效，再取消实际 controls；微任务执行前核对 current。不要在父 useLayoutEffect 抢先启动新子树，不要用固定延时或取消 StrictMode 绕过。真实组合回归见 animated-glyph.integration.test.tsx，必须有多个不同中间帧及最终静态／重播断言。


禁用属性必须读取值约定：原生 :disabled、aria-disabled=true、Radix 空 data-disabled、cmdk data-disabled=true 均阻止播放；cmdk 可用行带 data-disabled=false，不能用 [data-disabled] 存在性一律禁用。命令面板图标变更用真实 Command／CommandItem 验证 hover、键盘选中和禁用，不以手写 role=option 替代真实 DOM。

## Reader 版本与命令调度

Reader 的安全 HTML 仅来自 readerAPI，图片／链接均为登记 ID。当前正文 Query 包含 article revision，变化时重挂载内部正文、取消旧提取；外层自动已读 ref 保留同篇已处理意图，避免刷新后又覆盖人工未读。挂载前到达的菜单请求使用可取消 queueMicrotask 消费，cleanup 先失效，严格模式重放后才发请求；不能先清请求再启动一个马上被 StrictMode cleanup 取消的操作。保留真实 StrictMode 单次执行回归。

## OPML 预览与原生授权

transferAPI 是唯一文件调用边界，Go空items归一为数组；组件不拼宿主路径。Settings data分区持有短期预览和选择，Zustand仅保存一次性transferIntent；菜单早到意图仍用可取消微任务消费，StrictMode不得重复打开原生对话框。活动操作ref防重复提交、AbortSignal中止请求，卸载／旧授权返回后Discard能力；成功预览不能立即Discard其origin token，否则会误删同源私有预览。确认前零Import调用，取消在途提交提示检查列表，不能承诺已提交回滚。预览每页最多200行，跨页选择保留；标签使用data-animated-trigger供整行悬停及输入聚焦播放既有图标动画。

## shadcn/ui 表单控件

业务表单从 `@/components/ui` 导入 SearchableSelect、Switch、Checkbox、Input 和 Textarea。SearchableSelect 支持单选与多选，使用可访问名称关联搜索框，按选项名称和可选别名进行模糊搜索。展开后焦点进入搜索框，键盘可选择和关闭；清空或回到顶层使用独立非空选项值，在提交给业务层前映射为原有空字符串。服务与模型启用使用 Switch，清除密钥和选择导入项使用 Checkbox，确保显示状态、可访问名称与实际保存值一致。新增组件沿用 radix-nova 注册表及 AnimatedGlyph。系统文件对话框等原生能力按 AGENTS.md 的例外处理。

## macOS 侧栏材质

Wails 3.0.0 在 macOS 开发和发布构建中均需启用 `private_mac_apis`，否则 WebView 的不透明底色会盖住系统模糊。主窗口以 `MacBackdropTransparent` 创建，首次显示前用 AppKit 在 WebView 下放置 `NSVisualEffectMaterialSidebar`，混合模式为 `BehindWindow`。侧栏祖先保持透明，CSS 只叠加主题底色并处理对比度与减少透明度回退。阅读工作区保持不透明。验证时在 macOS 上用有纹理的窗后背景对比侧栏与正文。

macOS 侧栏交互使用半透明状态色。以 `aria-current=page` 统一阅读视图、分类和订阅的选中强度。阅读视图的 Motion 背景继承同一个填充色，避免同时绘制两层。折叠按钮保留 shadcn 的键盘焦点样式。

侧栏分隔条通过 `Splitter.onCollapse` 启用拖过下限收起，保留上次保存的宽度。前端宽度 220–360px 与 Go 偏好校验及读回范围保持一致。分隔条取消拖动时只清除临时宽度。

固定单选项使用 `SegmentedControl` 配合 `ToggleGroupItem`，保留单选键盘语义，每项带语义图标。下拉选项保持单行，菜单按内容取宽，受窗口限制时允许横向滚动。分类树展开与收起对分支高度做动画，离场分支立即退出键盘导航。关闭动画时直接显示或隐藏。

### Windows 原生材质

`[data-platform=windows]` 下，`.application-menu` 与 `.glass-sidebar` 共用 `--window-chrome-surface`。半透明颜色由 CSS 提供，桌面模糊由 DWM 提供。文章工作区使用实色背景，保留浮层的语义颜色。降低透明度或高对比度媒体查询命中时统一换成实色，强制颜色使用 `Canvas`。检查深浅主题、侧栏收起以及窗口按钮交互。
