# 列表设计

数据与范围遵循 [双视图](article-views.md)。默认文字优先，Compact 行高候选 64px、Comfortable 88px；固定高度，标题最多两行，来源与发布时间一行，状态图标不挤走标题。缩略图 none／small，small 为 48×48px、object-fit:cover，不改变行高。

每行展示 Title、Feed、Published Time、Unread、Favorite、AI indicator；无时间显示获取时间并说明。AI 未运行不显示假标记。选中与焦点分别呈现，不把 hover 当选择。收藏／已读按钮在 Hover 或 Focus-within 出现；触控模式始终提供更多入口。

TanStack Virtual 限制 DOM，key 为 article_id；J/K 改当前逻辑选择并滚动到可见，不能依据 DOM 顺序数组误跳。标记已读后若被当前过滤排除，保持相邻逻辑文章焦点并提供 Undo；失败回滚并重新获取计数。

验收 LIST-01：两密度和缩略图设置不改变查询结果；10k 数据可键盘遍历、滚动、收藏，失败恢复真实状态；文本缩放仍可读。
