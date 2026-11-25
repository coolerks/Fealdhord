# 安全正文与登记资源

## 1. 范围与触发条件

修改 Reader、订阅 HTML 清理、网页提取、正文图片／链接或自动已读设置时。正文和资源仅渲染清理后的 HTML。原文由系统浏览器打开。

## 2. 签名

reader.Service.Get{article_id,mode:feed|reader}、Extract{article_id,mode} → Detail{article_id,html,text,mode,has_cached,has_original}；OpenLink{article_id,resource_id?} → ok／error／request_id。空 resource_id 表示原文。媒体路由 `/media/inline/{64位hexID}`，只读取已登记 image；链接 ID 不可当图片使用。

`media.API.CopyInlineImage(ctx, id string) CopyImageResult` 复用同一个 64 位小写十六进制资源 ID，结果字段为 `ok`、`error`（可选）和 `request_id`。

## 3. 契约

00007 给 reader_contents 增 content_hash，reader_resources 按文章 hash／mode 保存 image／link。HTML 先白名单清理并规范化 URL，仅作为 Go 缓存；发送前再改写本地图片 ID 和 href=#reader-link／data-reader-link。外部脚本、事件、样式、表单、嵌入和危险 URL 均不进入展示 HTML；source 地址不直接成为客户端 src／href。每篇正文最多32图、256链接，图加载复用网关／缓存／2个解码槽和实时隐私策略。

提取固定 Readability v2.1.3，统一网关最多5MiB／20秒，DOM最多100000节点／128层，同时最多2次提取；只调用本地 DOM 解析。RPC CancellablePromise 取消贯穿网络和存储，第三方异常返回 ReaderError；解析CPU检查前后取消，不另起不可控解析 goroutine。提取缓存与 RSS 正文分离，保存和资源登记均复核文章 hash；旧资源加载失败，不能覆盖新文。

图片复制沿用内联加载、解码槽和隐私策略。JPEG／WebP 转为 PNG。写入前复核当前资源及取消状态，系统写入由桌面主线程处理。

## 4. 校验与错误矩阵

缺ID／非法mode→ValidationError；无缓存／文章变化→NotFound／Conflict；无法提取→ReaderError且保留订阅正文和旧缓存；危险地址→PermissionError；取消／超时保持明确错误。OpenLink 只打开当前文章登记链接或原文，校验 HTTP(S) 后交给系统浏览器，不接受前端文件路径或任意 URL。

复制请求 ID 格式非法时返回 `ValidationError`。资源不存在、属于链接、受策略限制或系统剪贴板不可写时返回 `MediaError`。

## 5. 良好、基础和异常案例

良好：中文正文、代码、表格离线缓存可读，来回切换不重复抓网页。基础：没有正文时用安全摘要／文字回退。异常：刷新后旧正文图片ID或跨文章链接ID被拒；Never取消在途图片，前端不得把旧提取结果覆盖新revision。

缓存中的已登记 JPEG 可以转成 PNG 复制。启用 Never 后，已缓存图片也不得写入剪贴板。

## 6. 必需测试

恶意脚本／事件／SVG／iframe／CSS／data/file URL，安全代码表格、相对链接、本地ID、原文打开；真实 SQLite 缓存／版本CAS／资源隔离；禁图零请求、归档策略、缓存和在途取消；组件StrictMode自动已读单次、取消后迟到响应、缓存切换、revision变化及早到菜单命令。依赖与决策见 ADR-0013。

复制回归检查输出 PNG 的实际尺寸、无效及链接 ID 的拒绝，以及 Never 策略下零额外请求和零写入。

## 7. 错误与正确做法

错误：把数据库中含远程资源的中间 HTML 直接传给 dangerouslySetInnerHTML，或使用 Readability.FromURL 绕过网关。正确：仅暴露 prepareHTML 的本地ID产物；UI正文 Query 绑定 ArticleSummary.revision，更新时取消旧提取并重建正文子树。自动已读意图在同篇外层持有，不能因revision重挂载又覆盖用户手动未读；默认可关闭。

清理器会把空片段 `href="#"` 规范化为空并移除；登记链接使用非空本地片段 `#reader-link`，并测试最终Go输出的真实href，确保键盘能聚焦和激活，不能只在组件mock里手写一个可用链接。

复制接口接收登记图片 ID，由 Go 复用 `loadInline` 获取字节。前端提交的 URL、路径和图片字节都没有复制入口。

代码块工具栏使用清理后的正文 `pre` 内容。复制保留高亮前的文本和换行，语法高亮沿用有界检测与字符预算。设置的 `reader_code_theme` 只保存已验证格式的样式标识。主题 CSS 从已安装的 highlight.js 样式按需加载，选择器限定为当前阅读正文。

## 文章正文翻译

### 1. 范围

适用于正文分批、译文落库、模型重试、阅读器进度以及后台全文完成判定。

### 2. 接口与存储

`reader.Detail.source_version` 对订阅正文使用文章内容哈希，对阅读模式使用清洗原网页正文和提取版本计算的摘要。翻译请求可带 `sourceVersion`、`startBlockId`。快照和 `translation:progress` 的 `Record` 返回 `sourceVersion` 与 `updatedAt`。迁移 00027 增加 `reader_contents.source_version`、`article_translations.source_version` 和 `checkpoint_json`，完整译文继续存于 `blocks_json`。

### 3. 执行契约

模型预算按实际 JSON、提示、标题和上下文估算，输出独立预留。同文章、语言、来源一次串行执行。短文单批，交互长文首批优先可见块。片段检查点按原块、拆分和模型提示快照匹配。父块全部片段校验后，在同一写入中合成完整译文并清理片段。进度按完整原块计数。

### 4. 错误与重试

缺段、空译文、重复标识、标记不符或输出截断，保留有效项并对缺失项安排一轮较小批次。临时网络错误允许短退避重试。连续两个批次没有有效结果、限流或预算错误时暂停。版本变化返回 `Conflict`，取消保留最后提交的记录，迟到响应不能落库。

### 5. 兼容

旧译文按当前正文的段落哈希和内联结构重新核对。重新提取相同正文保留版本，改动后只复用仍匹配的块。后台全文完成只接受当前原网页正文版本。

### 6. 验证

使用真实临时 SQLite 与模拟模型验证 66 段首批、单段拆分合成、漏段后继续、取消、片段跨重启、版本冲突和 v26 升级备份。

### 7. 更新顺序

先检查来源版本，在短事务写入完整段和片段状态。提交后推送进度。前端在当前来源与版本内只合并不早于已知 `updatedAt` 的结果。

### 8. 行内格式恢复

等效粗体和斜体标签、属性顺序及换行标记的自闭合写法可互认。模型输出须保持链接登记 ID、属性值、标签序列和代码占位符。通过核对后使用原标签。格式校验失败且有模型译文时，在现有一次局部补发中用文本标记替换 HTML 标签。返回标记要齐全、唯一、顺序一致，再恢复源标签。补发输出额度受相同模型预算约束。

只在非代码文本节点额外还原非 ASCII 可显示数字实体，并重新进行 HTML 转义。原文有意展示的实体保留。与当前正文版本及段落哈希匹配的已存译文可在读取时处理，因此无需重新请求模型。回归覆盖标签写法与属性顺序、链接标识变化、无效标记、旧译文读取及截断补发。

### 9. 摘要语言与正文触发

有摘要时优先复用当前文章内容版本、目标语言下的摘要结论。摘要没有最终结论时，可对已有摘要做本地判断。摘要已判定与目标语言一致时，正文读取返回 `skipped_same_lang`，正文请求在解析原文和查找模型之前结束，后台全文候选也排除这篇文章。摘要为空时不使用该规则。显式 `Force` 可越过摘要判断；正文仍按原有批次处理。测试覆盖摘要本地命中、已有摘要结论、不同目标语言、后台无提取、空摘要与手动覆盖。
