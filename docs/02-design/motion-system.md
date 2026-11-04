# 统一动效系统与语义矩阵

动态语言：Fold／Stack／Reveal／Glide／Settle，表达折叠、错层、展开与归位。动效从 Phase 1 建立，不能留到发布前统一补。

| token 层级 | 默认时间范围 | 用途 |
| --- | --- | --- |
| duration.fast | 120–200ms | Hover／Focus／Press／图标 |
| duration.normal | 180–280ms | 选择／已读／收藏／Toggle |
| duration.panel | 220–380ms | 侧栏／菜单／面板／Dialog |
| duration.content | 240–450ms | 模式切换／Reader／受控重排 |

统一 ease.standard、spring.gentle、spring.snappy，具体参数通过真实窗口校准。CSS 优先 transform／opacity，复杂组合用 motion；不持续动画 width／height／blur／shadow。布局拖动即时跟手，释放后轻归位。禁止每卡 JS 动画、长队列 stagger 与滚动视差。

| 场景 | Hover／Focus | 激活与结束 |
| --- | --- | --- |
| 导航 | 背景浮现，图标向文字轻移 | 选中指示滑动 |
| 分类 | 箭头／文件夹微开启 | 子项展开并保留焦点 |
| 收藏 | 星形轻倾 | 填充、一次微回弹，可撤销 |
| 已读 | 圆点／信封局部变化 | 圆点与字重过渡 |
| 刷新 | 箭头有限微动 | 真实请求中旋转，成功归位，失败提示 |
| 搜索 | 放大镜轻移 | 输入框展开并获焦 |
| 设置 | 齿轮有限角度转动 | 面板进入 |
| List／Masonry | 布局图标短排列 | 有界视口内重组，保持锚点 |
| 卡片 | 低幅抬升，图轻微缩放 | Reader 连贯打开 |
| 更多 | 三点轻移／变色 | 从触发点展开，关闭回焦 |
| Sidebar 折叠 | 箭头方向提示 | 导航与文字同步过渡 |
| AI 总结 | Sparkles 单次微闪 | 仅真实生成有活动指示 |
| Send／Cancel | 箭头轻上移 | 短反馈，点击区不变 |
| Tool | 当前工作反馈 | 成功／失败／待许可不同图文 |
| Toast／Undo | 短滑入淡入 | 撤销恢复真实状态 |
| DnD | 把手可见 | 浮起、目标线、释放归位 |

每个按钮至少 Hover／Press／Focus-visible／Disabled。图标运动限于容器，命中区不动；Tooltip 延迟建议 500ms，不能遮焦点。快速反向 hover／连点中断当前动画，不残留半状态；仅真实加载／同步／Agent 工作允许持续活动。

## 减弱动态优先级

Animation=Off → 全部装饰动效禁用；否则 Follow System=true 且 prefers-reduced-motion=true → 按 Subtle，移除空间移动／缩放／弹簧，只保留≤120ms淡入或即时状态。显式 Subtle 同样处理。状态和操作不因关闭动画失效；CSS 与 motion 共用同一有效模式，不能只关闭其中一层。

## 验收

MOTION-01：上述矩阵各有演示与交互检查，快速 hover／连点无排队、闪烁或卡遮罩；Focus 与触控功能完整。

MOTION-02：Full／Subtle／Off 和系统优先级生效，滚动不反复播放正文／卡片 Reveal；静止界面无永久空转。静态截图不能证明通过，需交互日志或录屏。
