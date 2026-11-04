# 执行清单

- [x] 根据既有数据库／分类／搜索契约收敛范围，读取前后端规范与官方 SQLite 文档。
- [x] 00002／00003 DDL／约束／视图／索引／FTS 触发器及 sqlc 生成。
- [x] 实例锁、一致快照、升级前备份和新目标恢复，真实故障测试；完成部分直接提交。
- [ ] 核心仓储、身份合并、分类来源、CJK／FTS、LibraryService 与真实前端入口。
- [ ] Go 集成／race／vet、前端组件／type／lint、原生构建，更新中文契约与追踪，提交归档。

边界：internal/database／library、migrations、sqlc 配置、main 装配和前端 library／commands 适配。原生菜单与视觉维持刚完成的修订，不回退。验证全在临时目录，恢复测试禁止覆盖用户数据；不做额外 E2E。
