# 正式阅读 Shell 实现契约

## 1. 范围与触发条件

Phase 1 的正式阅读空间、双视图、外观、键盘和偏好恢复。Phase 2 已接真实领域库及状态持久化，RSS 抓取与完整管理界面在后续阶段接入；不注入测试文章。

## 2. 签名

App → PreferencesGate → Shell；lib/preferences 适配生成的 SettingsService.Get／Update。articleQueryKey=['library','articles'] 现在保存有界 Page={items,hasMore}，filterArticles 在已加载范围派生系统 scope／搜索／排序；接入分页后查询键必须包含实际影响后端结果的参数，不能包含 list／masonry。

ArticleCollection 消费同一 Article[]、selectedID 和操作回调；TanStack Virtual 的 lanes 随容器计算，条目按稳定 ID 定位。Reader 的 onAnchor 与文章浏览共享有界保存入口。

## 3. 契约

- 用户后续修订为默认双栏浏览，打开文章默认铺满侧栏右侧区域。可在「阅读排版」选择抽屉或并排，后者仅在宽窗口列表视图下并排。侧栏 220–360px、列表 300–420px；侧栏收起时隐藏整列导航。并排布局遇到窗口≤1080px 或瀑布流时回退为抽屉；不为切换模式复制文章缓存。
- List 支持紧凑／舒适、无／小缩略图；Masonry 原比例／裁剪、动态高度、极长图上限 320px。只渲染 /media/ 本地图片，失败回退文字；远程首图由后续安全媒体服务交付。
- 浏览锚点键为 scope|search|sort|mode，值为 {article_id,offset}；Reader 使用 reader|article_id，offset 为 Reader 内部滚动距离。共用最多 32 项。恢复和尺寸重算期间不把顶部测量覆盖到已保存锚点；180ms 防抖，卸载／切换时冲刷最后值。
- J／K 切换并同步列表焦点，F 收藏，M 切换已读，/ 搜索，Cmd／Ctrl+K 命令面板，Escape 关闭 Reader；表单、菜单、对话框、输入法合成期间不抢单键。分隔条 Arrow±8px、Shift±20px、Home／End 到边界。
- Theme=Light／Dark／System 即时生效；Animation=Off 优先，Follow System＋系统减少动态或 Subtle 使用减弱模式。Off 禁用装饰动画但保留虚拟条目的定位 transform；Subtle 去空间运动／弹簧，短淡入≤100ms。Full Reader 使用有界弹簧；静止空态不持续空转。
- 设置与命令面板按需加载。Dialog 有标题、焦点管理及关闭回焦；诊断移至设置的关于区域。原生菜单与真实窗口材质已由[原生体验契约](native-experience.md)补齐，不依赖私有 macOS API。

## 4. 校验与错误矩阵

偏好读失败→安全错误页和重试，不冒充已恢复；写失败→保留本地编辑并显示未保存，可重试或显式重新载入；revision 冲突→提示重新载入，不覆盖其他窗口。非法持久值由 Go 逐字段恢复默认，未来 schema 拒绝读写。

生产无文章→真实空态。订阅、AI 和刷新尚未交付→清楚说明，不展示假成功。Phase 2 收藏／已读已接 Go 串行持久化与失败回滚；完整分页及全库筛选仍在 Phase 4。

## 5. 良好、基础和异常案例

良好：列表选择文章后切换 Masonry，选择／收藏共享，Reader 滚动恢复。基础：新库 System＋List，无订阅与 Reader 空态。异常：拖动分栏／重挂载时把 scrollTop=0 覆盖到已保存 ID 锚点，属于缺陷。

## 6. 必需测试

19 项前端测试：1 万条数据只挂载视口＋overscan、多列横向位置／逻辑 DOM 顺序、坏图回退、单列／长图上限、锚点防抖冲刷及重挂载恢复、Reader 滚动和焦点、共享操作、主题／动画、空态、读取失败、串行保存／重试及保留的诊断用例。

真实 SQLite 5 项设置测试含重开、8 路 CAS、非法值、旧值恢复、未来 schema 保护；不接触用户库。验收命令和未覆盖平台见 phase1-validation.md。本阶段不运行非必要 E2E；组件几何模拟不等于真实 WebView 性能认证。

## 7. 错误与正确做法

错误：CSS columns 改变逻辑阅读顺序、两套文章状态、localStorage 保存第二套偏好、Off 全局删除 transform。正确：同一 Query 投影、虚拟 lanes 与稳定 ID、Go／SQLite 单一持久事实、只关闭装饰运动。

阅读模式在工具栏提供重新提取图标，复用正文提取与取消机制。提取失败保留当前正文，可从工具栏重试。
