# 风险与处理门禁

状态均为开放，除非另标“已澄清”或“已关闭”；风险不是已发生缺陷的同义词。

| ID | 风险 | 证据与影响 | 缓解／验证阶段 |
| --- | --- | --- | --- |
| R-01 | doctor 沙箱限制，已关闭 | sysctl 沙箱拒绝；沙箱外 doctor ready=true，native 启动与调用已验收 | 证据见 phase0-validation.md，其他平台仍需各自检查 |
| R-02 | Go 下界冲突，已澄清 | 当前 Wails 要求 1.26+；原文 1.25+ 不足单独指导构建 | ADR-0001，后续固定 1.26+ 工具链 |
| R-03 | WebView／CSS 差异 | 玻璃、Tailwind 新 CSS、motion 未实际验证 | Phase 0 固定最低版本，Phase 1 实色 fallback，五平台实测 |
| R-04 | 瀑布流顺序与锚点 | lanes／异步尺寸不自动满足可访问顺序 | Phase 1 原型，Phase 4 规模测试 |
| R-05 | 中文搜索召回与体积 | unicode61 与 trigram 有边界，二字片段方案未 Go 验证 | Phase 2／4 候选检索与连续文本校验、100k 测量 |
| R-06 | sqlc／FTS 与驱动兼容 | 系统 sqlite 可用不等于 modernc 可用 | Phase 2 实际 FTS5 与生成器探针 |
| R-07 | 抓取 SSRF／资源耗尽 | RSS、Reader、媒体均能触发网络 | 网络统一策略、逐跳检查、字节／像素限额，Phase 3 |
| R-08 | 凭据服务不可用 | Linux Secret Service 不一定存在 | 仅内存 fallback，不落明文，Phase 5 平台测试 |
| R-09 | Provider 部分兼容 | 多工具、SSE、参数、usage 差异 | 独立 Adapter、capability、Mock contracts，Phase 5 |
| R-10 | Prompt injection／误写 | 外部内容与工具组合可能误导执行 | 后端审批快照、最小 allowlist、revision，Phase 7–11 |
| R-11 | MCP 协议演进 | 2026-07-28 与旧版本差异 | 锁定 SDK、协商矩阵、当前与旧版本互操作，Phase 10 |
| R-12 | 迁移／恢复损失 | 活跃 WAL 文件直接复制不一致 | 在线备份、故障注入、恢复演练，Phase 2 |
| R-13 | 五平台证据不足 | 当前仅本机命令检查 | 无实机证据保留未测，阻止全平台发布声明 |
| R-14 | 默认值未体验验证 | 颜色、阈值、尺寸、预算为首版建议 | 最终评审与阶段实测，变更留 ADR／设计记录 |
| R-15 | 依赖维护与许可证 | README 能力不证明维护质量 | 每阶段锁版本时检查 release／commit／公告／许可证，不预先认定所有库健康 |

Owner：当前主会话；下次会话接手时以 LATEST 和任务文件确认负责人。风险缓解需提供实验结果，单写文档不关闭。
