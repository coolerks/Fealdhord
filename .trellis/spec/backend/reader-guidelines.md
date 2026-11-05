# 安全正文与登记资源

## 1. 范围与触发条件

修改 Reader、订阅 HTML 清理、网页提取、正文图片／链接或自动已读设置时。正文和资源不得让远程 HTML 在有 Wails 绑定的 WebView 内执行或导航。

## 2. 签名

reader.Service.Get{article_id,mode:feed|reader}、Extract{article_id,mode} → Detail{article_id,html,text,mode,has_cached,has_original}；OpenLink{article_id,resource_id?} → ok／error／request_id。空 resource_id 表示原文。媒体路由 `/media/inline/{64位hexID}`，只读取已登记 image；链接 ID 不可当图片使用。

## 3. 契约

00007 给 reader_contents 增 content_hash，reader_resources 按文章 hash／mode 保存 image／link。HTML 先白名单清理并规范化 URL，仅作为 Go 缓存；发送前再改写本地图片 ID 和 href=#reader-link／data-reader-link。外部脚本、事件、样式、表单、嵌入和危险 URL 均不进入展示 HTML；source 地址不直接成为客户端 src／href。每篇正文最多32图、256链接，图加载复用网关／缓存／2个解码槽和实时隐私策略。

提取固定 Readability v2.1.3，统一网关最多5MiB／20秒，DOM最多100000节点／128层，同时最多2次提取；只调用本地 DOM 解析。RPC CancellablePromise 取消贯穿网络和存储，第三方异常返回 ReaderError；解析CPU检查前后取消，不另起不可控解析 goroutine。提取缓存与 RSS 正文分离，保存和资源登记均复核文章 hash；旧资源加载失败，不能覆盖新文。

## 4. 校验与错误矩阵

缺ID／非法mode→ValidationError；无缓存／文章变化→NotFound／Conflict；无法提取→ReaderError且保留订阅正文和旧缓存；危险地址→PermissionError；取消／超时保持明确错误。OpenLink 只打开当前文章登记链接或原文，校验 HTTP(S) 后交给系统浏览器，不接受前端文件路径或任意 URL。

## 5. 良好、基础和异常案例

良好：中文正文、代码、表格离线缓存可读，来回切换不重复抓网页。基础：没有正文时用安全摘要／文字回退。异常：刷新后旧正文图片ID或跨文章链接ID被拒；Never取消在途图片，前端不得把旧提取结果覆盖新revision。

## 6. 必需测试

恶意脚本／事件／SVG／iframe／CSS／data/file URL，安全代码表格、相对链接、本地ID、原文打开；真实 SQLite 缓存／版本CAS／资源隔离；禁图零请求、归档策略、缓存和在途取消；组件StrictMode自动已读单次、取消后迟到响应、缓存切换、revision变化及早到菜单命令。依赖与决策见 ADR-0013。

## 7. 错误与正确做法

错误：把数据库中含远程资源的中间 HTML 直接传给 dangerouslySetInnerHTML，或使用 Readability.FromURL 绕过网关。正确：仅暴露 prepareHTML 的本地ID产物；UI正文 Query 绑定 ArticleSummary.revision，更新时取消旧提取并重建正文子树。自动已读意图在同篇外层持有，不能因revision重挂载又覆盖用户手动未读；默认可关闭。

清理器会把空片段 `href="#"` 规范化为空并移除；登记链接使用非空本地片段 `#reader-link`，并测试最终Go输出的真实href，确保键盘能聚焦和激活，不能只在组件mock里手写一个可用链接。
