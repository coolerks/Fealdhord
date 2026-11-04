# 安全验证清单

实现前使用 [安全架构](../03-architecture/security.md)；以下全部待应用验证。

- NET-01：HTTP(S) 以外、URL userinfo、本地／私网／保留 IP、IPv4-mapped IPv6、DNS 重绑定、逐跳重定向与代理绕过被拒；Provider／MCP 的指定 origin 例外不扩散。
- NET-02：解压后字节、图片像素、SSE／工具参数、时间与并发上限有效；跨域不带认证。
- HTML-01：script／事件／iframe／form／style／SVG fixture 不执行；外部原文只经系统浏览器；本地媒体 handler 不能读任意文件或代理任意 URL。
- SECRET-01：API Key、敏感 Header／env／query 不出现在 SQLite、日志、Query、Zustand、localStorage、错误与诊断导出。
- PERM-01：后端重复验证 scope、参数、策略、revision 和审批期限，直接绑定／MCP 调用也无法绕过；双击确认不重复执行。
- SKILL-SEC-01：manifest／assets 穿越、symlink、超限与代码执行拒绝；Prompt injection 不改变权限。
- MCP-SEC-01：默认关闭、loopback、认证、Origin、工具风险不信任远端自述、断线外部写不自动重放。
- DATA-01：迁移失败不继续写库，备份包含一致快照，恢复不覆盖唯一可用副本；导出不含凭据。
- LOG-01：结构化级别正确，秘密脱敏和隐私清除有效，生产默认不记录全文模型上下文。

每项记录测试文件／命令／平台／结果；不能把存在文档当作安全检查通过。
