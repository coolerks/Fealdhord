# 首图与媒体契约

Phase 3 已实现候选、受控首图加载与现有列表／瀑布流展示；Phase 4 继续完整阅读器内媒体体验。不依赖模型。

## 候选优先级

1. 条目 media:content／media:thumbnail／image enclosure／JSON Feed image 或 banner_image。
2. content:encoded／content 中有效 img，检查 src／srcset／data-src 等。
3. description／summary 中图片。
4. 用户开启“网页补图”后，从受控原文抓取的 og:image／twitter:image 获取。
5. 无有效候选使用无图卡，favicon 只标识来源。

同优先级按可用尺寸与出现顺序筛选。明确 1×1、尺寸低于 64px、常见头像／图标和追踪路径降权或排除；未知尺寸不能在抓取主事务中等待。srcset 按目标缩略尺寸选择有界资源，不盲取最大原图；相对 URL 根据条目链接、xml:base 与 Feed 基址确定，记录最后规范化 URL。剥离危险 scheme；正文原始内容不因裁剪改变。

## 元数据与状态

article_media：url、source、status、width、height、alt、checked_at、retry_after、cache_key。source 为 media_content/media_thumbnail/enclosure/json_image/content/summary/og/twitter。status 为 none/pending/ready/failed/blocked；未知尺寸为 null，不填假 0。用户禁图时不发探测请求，保留元数据但展示 blocked 状态；重新启用可按需重试。

Feed 提交后低优先级异步探测，失败依序回退下一候选（最多 5 个），全部失败为 failed；不从正文无穷提取并抓取每张图。支持 JPEG／PNG／WebP 等已验证解码格式；SVG 默认拒绝进入卡片渲染，动画图不自动播放。尺寸／解码像素和响应体均受安全网关限制。

## 加载与隐私

`Load Remote Images=Always|Subscriptions Only|Never`；默认 Subscriptions Only，含当前订阅条目中明确引用的 CDN 资源，不等于只允许同域；Always 也仅加载已登记内容的图片，不能成为任意代理。Never 对探测、正文、封面、favicon 全部生效，零后台媒体请求。网页补图默认关闭，显式开启才额外访问原网页。

前端使用本地媒体 ID endpoint 按需加载，Go 管请求与有界缓存；首版候选缓存总量 256MiB、LRU、7 天 TTL，可清除，不缓存认证页面或秘密 URL 的明文日志。禁缓存时使用有界内存流，不强制保存原图。离线缓存可用，失败无图不崩溃。

## 验收

MEDIA-01：优先级、相对 URL、srcset、懒加载、JSON Feed 与假像素 fixture 选择正确。

MEDIA-02：404／403／超时／解码失败／超大文件／私网重定向降级；禁图无探测与下载；favicon 不充当封面。

MEDIA-03：尺寸预留、懒加载、异步解码、缓存上限与清理有效；图片到达锚点偏差符合质量目标，滚回不反复播放入场。


## Phase 3 实现依据（2026-11-05）

00005_article_media 增加 blocked、候选表与 generation。媒体完成前复核文章内容和 generation，策略改 Never 会取消在途请求并阻止旧响应回写。磁盘最多 192MiB、短期内存流最多 64MiB，合计不超过 256MiB；磁盘 TTL 为 7 天，访问不续命。私密 URL／private／no-store 响应不进入缓存；禁磁盘缓存用短期有界内存流。JPEG／PNG／WebP 经配置和完整解码校验，拒绝 SVG／动画格式及超过 40MP 的内容。

Wails media.API 挂载 /media/ 服务路由，ServeHTTP 为内部方法，不暴露生命周期或文件路径；前端只能使用已登记文章媒体 ID。首图、隐私、TTL、取消和界面接线已通过相关测试。PNG／WebP 动画块被拒绝；开启网页补图会重新排入无图记录，单次扫描最多 500 条、工作队列 64，余下记录随后续刷新／按需请求处理，不承诺一次预载全库。媒体清理操作和三个偏好已接订阅设置。

## Phase 4 正文资源

正文图片扩展为登记的 `/media/inline/{id}` 路由，不把任意 URL 交给客户端或当代理参数。复用首图网关、解码限制、缓存、归档／Never策略；图片只在正文展示按需请求，不在提取过程中预载。链接同样登记，由Go校验后交系统浏览器。00007绑定文章内容hash，旧版本资源不能继续加载。
