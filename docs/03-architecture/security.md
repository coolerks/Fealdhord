# 安全与隐私边界

## 不可信输入

RSS／网页／图片、模型输出、OPML、Skill 文件和 MCP 内容均不可信。数据库已有内容也可能来自远端。所有入口限制大小、深度、时限；不把不可信文本提升为指令或权限。

## 网络网关

Feed、发现、Reader、封面探测只允许 HTTP(S)，默认拒绝 URL userinfo、loopback、私网、链路本地、组播、未指定、保留地址及 IPv4-mapped IPv6 绕过。解析 DNS 后检查全部候选，dial 连接已检查 IP，保留 Host／TLS SNI；每次重定向重新检查。最多 5 次重定向，跨主机不转发认证／cookie。拒绝本地文件和任意协议；代理同样必须执行目标策略，不能用代理绕过。

用户显式配置 Provider／MCP 的指定本机服务可单独授权 origin；该例外不用于 Feed／图片。网络限额拟定：Feed 解压后 10 MiB、Reader 5 MiB、图片 8 MiB 与 40MP、单请求 20 秒、图片 8 秒；超限流式中止。防解压炸弹和 DNS 变化必须有测试。

## HTML 与媒体

Go 使用自定义 bluemonday allowlist 清理正文，禁 script、事件属性、form、object、embed、危险 iframe、外部样式与可执行 SVG。链接只经安全外部浏览器入口；原网页不加载到带 Wails 权限的主 WebView。正文图片改写成本地受控媒体 URL／ID，前端不直接发远程请求；禁图时不发探测或下载请求，包括正文和卡片。

本地媒体 handler 只接受已登记媒体 ID，不接受任意路径或任意代理 URL，返回正确 MIME 与 nosniff，默认不渲染 SVG。CSP 限制脚本／连接／图片来源，开发与生产配置分开。

## Secret 与审计

API Key、敏感 Header、MCP env 存 OS keyring；SQLite 仅引用。凭据编辑 UI 是短暂输入通道，不负责管理和持久化。日志不记 Authorization、Cookie、secret query、完整用户内容；记录 code、operation_id 与安全摘要。执行历史可包含隐私，提供本机清除入口；没有账号或遥测上传前提。

工具审批是数据约束，不是 UI 装饰；直接调用后端也无法绕过。Skill allowlist 不能扩权，模型和 MCP 提示不能批准自身调用。见 [安全检查清单](../07-quality/security-checklist.md)。

验收 SEC-01：网络、HTML、Secret、权限、Skill 与 MCP 的安全检查全部由直接边界测试验证；安全文档存在本身不算通过。
