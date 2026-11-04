# 列表／瀑布流与虚拟化契约

`articleViewMode=list|masonry`、`listDensity=compact|comfortable`、`listThumbnail=none|small`、`masonryImageMode=original_ratio|cropped`。两视图共享 Query、排序、过滤、未读和收藏逻辑；只改变投影。

## 瀑布流

候选最小列宽 260px、间距 12px；`columns=max(1,floor((available_width+gap)/(min_width+gap)))`。有限最大卡宽防止超宽单卡；窄容器自动一列且无横向滚动。图片显示高度上限 320px，原比例限制在 0.65–2.0 范围以避免极长图占屏；cropped 默认 16:10。标题最多三行、摘要四行，完整内容在 Reader 中。无图为自然文字卡，不随机补图。

使用 TanStack Virtual lanes 与动态 measureElement／ResizeObserver 候选；article_id 稳定键，DOM 顺序始终是查询顺序，显式 x/y 定位。最短列放置允许错落，但同一索引前项的顶部位置不得晚于后项，时间可见、逻辑导航有明确顺序；不使用 CSS columns 或 dense 重排。J/K 依 article index，方向键的几何导航若实现需单独说明，不能混淆。

图片已知尺寸先预留区域，未知先保守估高，解码后测量；同一轮 ResizeObserver 批量修正，避免同步反复测量。列数变化时保存顶部可见 article_id 和相对 offset，失效旧宽度测量后重新定位，禁止继续使用过时 lane 缓存。追加分页不重新分配已显示卡片，列表数据变化使用稳定排序与快照。

## 锚点与阅读

每个 query_hash + view 保存 `{article_id,offset,selected_article_id,reader_scroll}`，不只存 scrollTop。切换模式／窗口缩放／图片到达前记录锚点，重排后恢复；首版目标偏差≤24px，失败须记录测试结果。锚点消失按相邻排序项回退，不无故跳顶。Reader Drawer 关闭返回触发卡片并恢复焦点。

虚拟项卸载时保留焦点对应 ID，下一次挂载恢复，不让键盘进入未挂载节点。可访问结构以 list/listitem 和可聚焦标题按钮为主；aria-posinset／setsize 仅在总数准确时使用。禁止添加错误的 grid 角色却不实现其键盘模型。

## 验收

- VIEWS-01：切换 List⇄Masonry 保持结果、排序、选择、过滤、已读和收藏，重启恢复偏好。
- VIEWS-02：动态宽高、未知尺寸、列数变化、追加分页、快速滚动均无重叠；DOM 只随视口和 overscan 增长。
- VIEWS-03：键盘和读屏顺序等于查询逻辑顺序，关闭 Reader 恢复焦点和锚点；1k／10k／100k 数据必须分别实测。
