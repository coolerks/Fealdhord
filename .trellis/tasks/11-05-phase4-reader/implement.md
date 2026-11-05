# Phase 4 执行计划

状态：规划完成，按本轮用户授权直接实施。

1. library-query：先迁移／数据库查询和游标，真实数据边界测试；接 LibraryService.Query／绑定；前端分页、搜索防抖、状态缓存与命令适配；检查、同步规格、提交。
2. library-navigation：分类元数据／CAS／删除预览、全库导航与计数；接侧栏／管理／文章成员／继承范围；相关测试、检查与提交。
3. reader-content：提取器版本／契约探针、安全 HTML／离线缓存／受控取消；前端阅读、原文入口、自动已读／字体；相关测试与提交。
4. opml-transfer：对话框能力／解析预览／事务导入／安全导出；菜单与管理操作接线；fixture／文件失败回归与提交。
5. 联验 SEARCH／CAT／READER／OPML／API；运行 Go test／vet／相关 race、sqlc／绑定、前端类型／lint／测试、Wails build 与 macOS 调试包；文档检查，CURRENT／LATEST、归档、日志。

主会话 inline；每次切换职责读取对应 PRD／设计和 .trellis/spec。构建写 dist 时不并行 Go 全包 embed 检查。每部分直接提交，不推送；无关框架文件保留。
