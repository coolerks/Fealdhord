# 凭据与操作系统集成调研

核验日期：2026-11-04。

## 官方事实

[go-keyring](https://github.com/zalando/go-keyring)提供 Windows Credential Manager、macOS Keychain 与 Linux Secret Service 的跨平台接口。Linux 环境是否存在可用密钥服务需要实际验证，不能假定所有发行版都有。

## 项目决定

CredentialStore 包装库，SQLite 只存引用。无密钥服务时允许本次会话内存凭据并提示无法持久保存；生产版不能无声降为明文。开发 fallback 仅在显式开发模式使用内存 store，不写文件。Header、MCP env 和敏感查询参数同样进入凭据域。

## 待验证

锁屏、拒绝授权、服务不可用、密钥更新中断与卸载残留分别进平台测试；不在本轮读写用户实际凭据。跨 SQLite 与 OS keyring 没有分布式事务，采用新引用先写、DB 提交后再清旧引用并回收孤儿的补偿策略。
