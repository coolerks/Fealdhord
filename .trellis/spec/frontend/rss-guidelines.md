# RSS 功能接线

## 1. 范围与触发条件

修改订阅管理、刷新命令、进度事件、媒体偏好或领域 Query 时。生产界面只消费 Go 服务，不注入示例文章或直接抓远程 URL。

## 2. 签名

`rssAPI` 适配生成 FeedService／refresh.API／media.API；`feedsQueryKey=['rss','feeds']`，文章复用现有 `articleQueryKey`。`startRefresh(client, ids=[])` 的空数组表示所有活动订阅，`cancelRefresh()` 使用当前 operation_id。`useRefresh.accept(Snapshot)` 比较 generation／sequence；快照不写入偏好。

## 3. 契约

先订阅 refresh／library-changed，再请求 Current；同代次旧 sequence、旧 generation 和已关闭代次均丢弃。活动任务每 5 秒 Get 快照恢复丢失事件；清理订阅、计时器并拒绝卸载后的回调。领域事件 80ms 合并失效重取，不复制文章集合。

网页先 Discover，多个源明确选择后 Add；编辑保留打开时 revision，归档明确保留文章与状态，恢复使用当前 revision。图片请求仅使用 `/media/{articleID}`；Never 立即隐藏封面，真实隐私检查仍归 Go。刷新通过工具栏、命令面板、系统／自绘菜单共用命令 `refresh`，macOS Cmd+R／Windows Ctrl+R。

## 4. 校验与错误矩阵

多候选未选→禁止添加；发现／添加失败→显示安全错误并保留输入；CAS 冲突→保留编辑，不自动覆盖。刷新错误→可重新刷新；取消中→禁用重复取消。partial 即使全部失败也按失败数量展示，不写“刷新完成”。保存媒体偏好失败沿用既有保存状态和重试，不冒充已落盘。

## 5. 良好、基础和异常案例

良好：刷新事件先于 Start 返回，界面保留更新的快照。基础：空订阅、无图、正常空源仍可使用。异常：后台订阅 revision 变化后，旧编辑表单不能用新 revision 掩盖冲突。

## 6. 必需测试

FeedSettings 真实组件覆盖多源选择、失败重试、CAS、归档确认和恢复；store 覆盖乱序和关闭代次；useRSSSync 覆盖 StrictMode 清理、合并重取和丢失终态恢复；RefreshStatus 覆盖取消与全部失败；App 覆盖 Never 不渲染已有封面。运行类型、lint、相关组件回归与原生编译，不默认 E2E。

## 7. 错误与正确做法

错误：各按钮分别调用绑定、在 settings 中复制订阅数组、把 500 篇局部筛选当全库。正确：统一 API／命令、Query 是领域缓存、局部表单仅保留编辑草稿；Phase 4 才扩展全库分页／筛选和分类 CRUD。生成绑定不得手改；安全错误由后端生成，UI 不显示含凭据的原始网络异常。

订阅侧栏与设置列表共用 `FeedIcon` 显示已保存的 `icon_data`。缺图或远程图片设为「从不加载」时使用默认订阅图标。
