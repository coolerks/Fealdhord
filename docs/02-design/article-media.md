# 首图与媒体契约

Phase 3 提取元数据，Phase 4 完整加载与展示；不依赖模型。

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
