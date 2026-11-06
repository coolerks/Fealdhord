# 受控网络与 Feed 解析

## 1. 范围与触发条件

Feed、网页发现及媒体发起 HTTP 请求或消费不可信源时。

## 2. 签名

internal/netpolicy.Fetcher.Fetch(ctx,rawURL,Request) 返回 Response{URL,Status,Header,Body}；New 创建生产网关。feed.Parse(ctx,bytes,base) 返回规范化 Document；Discover 只返回明确候选，不提交订阅。

## 3. 契约

只允许无 userinfo 的 HTTP(S)，规范化 IDNA、默认端口和 fragment，保留 query。逐条校验 DNS 候选。域名解析返回的 `198.18.0.0/15` 可用于本机代理的 Fake-IP 路由。剔除其他私网与保留地址，剩余为空时拒绝。Dial 只使用筛出的固定 IP，Transport 保留 Host／TLS SNI，禁止环境代理绕过；每次跳转重检，最多 5 跳。跨源不继承条件请求或敏感头。响应最多解压后 10MiB／20 秒，图片调用可收紧；原始字节同样受限，仅支持 identity／gzip。

Parser 仅消费网关字节，禁止 ParseURL；XML／JSON 检查深度、token 数、条目数及指令，拒绝 DTD。HTML 自定义 allowlist 清理，纯文本供现有 Reader，不返回可执行原文；JSON Feed 的 content_text 与 title 不能被误当 HTML。图片只登记候选，不在解析事务发请求。

## 4. 校验与错误矩阵

非法协议／userinfo→ErrInvalidURL；私网／特殊 IP／危险跳转→ErrBlocked；原始或解压超限→ErrTooLarge；未知压缩→ErrEncoding；畸形源／复杂度超限→feed.ErrParse；无发现候选→ErrNoFeed。服务边界映射中文安全错误，不回传含秘密 URL 的原始 url.Error。

## 5. 良好、基础和异常案例

良好：网页两个 alternate 返回两个候选待选择；基础：合法空 Feed 成功；异常：首跳公网、重定向私网必须在目标连接前拒绝。304 由刷新层处理，不交 Parser。

## 6. 必需测试

httptest 加内部 resolver／dial 注入验证真正拨号目标；混合 DNS 只拨公网、仅有私网时拒绝、公网拨号失败时不能回退到私网、映射 IPv6、重定向、压缩炸弹、原始超限和取消。RSS／Atom／JSON 自制 fixture、坏日期／缺标题、JSON 纯文本与脚本清理；测试不访问真实站点。

## 7. 错误与正确做法

错误：仅在 UI 校验 URL 后调用 gofeed.ParseURL／http.Get。正确：统一网关校验实际连接，只解析有界本地字节。测试专用 resolver／dial 注入不暴露生产关闭安全策略的配置。


共享 netpolicy.Client 在 Fetch 外层限制全局 4 个请求，Transport 限制每主机 2 个连接且不启用 HTTP/2 合并；媒体另有 2 个并行探测／解码槽。媒体策略从后端事实读取，不能只靠 React 隐藏图片。回写复核 generation；本地 handler 只接登记 ID，不能接原始 URL／文件路径。

Phase 4：Feed.SanitizeBody 可保留规范化 HTTP(S) img／a 地址，属于 Go 缓存的中间 HTML，不是可直接展示的 DTO；Reader 发送前必须二次清理和登记本地资源。旧“清理器一律删除 img”的约束已由此更严格的展示边界替代，测试保留恶意事件／URL拒绝。
