# Phase 0 技术设计

官方 Wails v3.0.0 React TS 模板先生成到临时目录，再选择性引入工程，避免覆盖 AGENTS／README／docs。module 使用已存在 Git remote 对应 github.com/coolerks/Fealdhord。

根 main 仅装配窗口与服务。internal/app 提供应用信息与受控诊断操作；领域无厂商类型。前端 lib/api 包装生成绑定，开发诊断区真实使用事件和快照；不伪装为已完成 RSS 功能。

诊断操作为固定时长、本机 timer，无网络副作用；单活动操作与有界最近历史，操作 ID、sequence、明确终态和 context 取消，emit 在锁外按序。关闭应用取消任务并等待有界收尾。

Node、包管理器、Go／Wails 固定；前端生成输出与运行产物忽略，必要绑定源码纳入版本控制。doctor 失败先查固定版本源码与具体系统命令，再决定是否需要受限环境外复核，不改系统配置。

验收采用真正 native 绑定／事件；Go 单测验证竞态／终态，前端测试验证订阅、错误与状态。没有 schema，不需要迁移或数据回滚；工程初始化只新增文件。
