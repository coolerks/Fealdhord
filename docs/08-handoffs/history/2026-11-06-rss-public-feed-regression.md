# 公开 RSS 与 Atom 订阅被误拒

本机系统 DNS 为三个用户提供的域名同时返回了 198.0 网段的可用 IPv4 和 fd27 开头的私有 IPv6。原网关只要发现一个不合规候选就拒绝整个域名。修复前，三个地址经生产 Feed 服务 Discover 均返回 PermissionError。

网关现在逐条筛选 DNS 候选，只向通过校验的固定公网 IP 拨号。仅有私网时仍拒绝，公网连接失败时也不尝试私网。原有地址字面量、跳转及重绑定防护继续适用。

| 实际地址 | Discover | Add | 临时库文章查询 |
| --- | --- | --- | --- |
| https://kuizuo.me/blog/rss.xml | 1 个候选 | 成功 | 查到文章 |
| https://blog.skk.moe/atom.xml | 1 个候选 | 成功 | 查到文章 |
| https://innei.in/feed | 1 个候选 | 成功 | 查到文章 |

实际请求使用生产网关、Feed 服务和临时 SQLite。测试库已清理。固定回归覆盖混合 DNS、仅私网、公网失败及重绑定，无需自动测试依赖外部服务。
