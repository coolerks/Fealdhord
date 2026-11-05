# Phase 3 技术方案

## 1. 边界与所有权

internal/netpolicy 统一 HTTP 出口；internal/feed 负责发现、解析、规范化与订阅服务；internal/refresh 持有刷新操作和到期调度；internal/media 持有候选、探测、缓存与本地 handler；database 持有事务和参数化 SQL；main 仅装配。沿用既有 app 错误／事件形状，不替换诊断服务或让前端处理不可信 Feed。

## 2. 签名与数据流

网关提供带 context 的有界抓取，返回最终 URL、状态、受限响应 Header 和字节；Feed 上限解压后 10MiB／20 秒。Parser 仅接本地字节和基址，不使用 ParseURL。Discover 返回候选列表，Add 对选中的候选重新校验后提交；前端候选不能作为免检 token。

FeedService 提供 List／Get／Discover／Add／Update／Remove／Restore，具体 snake_case DTO 含 revision，分类 ID 后端核对。RefreshService 提供 Start／Get／Cancel 的具体快照，沿用 operation_id／sequence 与现有事件 envelope，快照可恢复；既有 BootstrapService 诊断 API 保持。每个阶段落地实际签名时同步绑定契约，生成代码不手改。

抓取 → 三格式解析 → URL／时间／身份／正文规范化及首图候选 → 单源事务写文章、别名、FTS、分类继承和 validator → 提交事件 → Query 失效。媒体探测仅在提交后进行，不持有数据库事务。允许扩展 AppError.code，不泄露原始 URL 查询秘密或 Go 错误。

## 3. 网络、解析与正文契约

只允许无 userinfo 的 HTTP(S)；规范化主机、默认端口、fragment，保留 query。解析全部 DNS 候选并拒绝任一危险 IP，Dial 连接已校验 IP 且保留 Host／TLS SNI；每个跳转重新校验，最多 5 跳。不读取环境 HTTP 代理来绕过目标策略；首版本网关不提供 Feed 的私网例外。

Transport 不使用共享 cookie jar／认证；跨源不携带敏感头。限制超时、响应 Header、压缩后及解压后字节；拒绝不支持的压缩，不把压缩炸弹交给 Parser。RSS／Atom 预检深度、条数和 XML 实体规则，HTML 发现同样有界；测试可注入 resolver／dialer，但生产配置不能关闭策略。

gofeed 统一三格式；goquery 选择 HTML 结构；bluemonday 自定义 allowlist 清理 HTML，正文纯文本供现有 Reader。禁止 script、事件、表单、iframe、外部样式及 SVG；媒体候选在清理前提取，前端不消费原始 Feed HTML，不默认访问原文网页。

## 4. 事务、刷新与调度

提取可复用的事务内 article upsert，公开单篇入口保留兼容；新增批量提交方法，在同一个事务提交整源文章、别名、继承分类与 validator。解析或任何文章身份冲突失败时整源回滚。文章状态表和稳定 sort_at 保留；304 只更新源检查／成功时间与错误计数。

刷新协调器维护有界 worker 和每站点限制。同一活动批次的新请求合并目标、去重 in-flight feed，返回共同快照；取消针对该活动批次，保留已提交源。默认每 30 分钟唤醒到期队列，启动只补一次，不为每源保留 goroutine。429／退避由数据库 next_check_at 驱动，时钟与抖动可注入测试；关机取消并等待 worker，之后才关闭 Store。

Feed URL 修改重新验证并保留 feed_id、旧文章和别名；revision 防止过期编辑。归档与提交需在事务内复核，避免刷新完成后恢复归档。重复添加不覆盖已有自定义名称；归档恢复使用原 ID。

## 5. 媒体与兼容迁移

候选优先级及排除规则沿用 article-media，最多探测 5 个候选；默认不请求网页补图。图片限 8MiB／40MP／8 秒，先探测尺寸再有界解码；拒绝 SVG，支持经测试的 JPEG／PNG／WebP，动画不自动播放。Remote Images 的 Always／Subscriptions Only／Never 由后端在探测和加载前检查；旧偏好安全迁移至 Subscriptions Only，网页补图默认关闭。

现有 article_media CHECK 不含 blocked，以新增 goose 迁移补足，不修改已应用迁移。候选、缓存键和必要版本按真实需要扩表。缓存总 256MiB、7 天 TTL、LRU；禁缓存使用有界内存流。handler 仅接已登记媒体 ID，不接受 URL 或磁盘路径，返回明确 MIME／nosniff。旧 cover 与 LibraryService 保持 /media/ 适配，不让 React 直接加载远程图片。

## 6. 功能接线与范围

在现有订阅设置页接入发现／添加、列表、重命名、分类关联、归档／恢复；菜单／工具栏刷新复用同一命令入口，取消和部分成功有反馈。现有 Query 通过 operation／domain 变化失效并重取，状态变更仍用既有持久化路径。功能表单沿用现有组件，不改视觉与动画，不提前实现 Phase 4 全库分页和筛选。

## 7. 验证与回退

网络恶意地址、DNS／重定向、压缩限额测试先过；三格式合法／畸形自制 fixture 与 sanitizer 测试离线可重复。真实 SQLite 验证整源回滚、304、不保存坏 validator、状态保留和并发去重；手动时钟测试调度／429／取消。媒体验证禁图零请求、候选失败回退及任意代理拒绝。功能组件验证真实 DTO 适配和失败状态；只在关键跨层行为无法替代时做最小真实运行。

按子任务职责提交并更新 CURRENT／LATEST，新增迁移前一致快照；不自动降级破坏性 schema 或覆盖数据。RSS 禁用／归档不删除已读收藏。新服务装配或媒体处理失败返回安全错误，不将未实施部分声明完成。
