# 最近交接：首版需求与技术规格

日期：2026-11-04。负责人：当前主会话。当前阶段：planning，规格编写完成待评审；应用尚未实现。

## 本次完成

归档 v2.0 原文并保留哈希；逐章追踪 108 节和五项补充；核对官方技术资料及本机环境；写入中文产品、设计、架构、功能、API、12 份 ADR、质量与实施计划；将前后端 Trellis 空模板替换为明确标注“拟定”的代码契约。

## 主要文件

入口 docs/README.md；状态 docs/01-status/CURRENT.md；需求 docs/00-product/requirements-traceability.md；边界 docs/05-api/；研究 docs/09-research/；任务 .trellis/tasks/11-04-fealdhord-spec/{prd,design,implement}.md。根 AGENTS 原约束保留，仅追加中文和文档工作协议。

## 实际验证

已运行环境命令、官方网页核对和离线 SQLite 语义探针；文档检查输出见 docs/09-research/validation.md。无 go.mod／package.json，应用构建／测试／启动均未执行，五平台和产品性能未测。

## 容易误解的设计

- 原文“立即实现”由本轮用户需求调研范围收敛，本次未编应用代码。
- Wails v3 稳定基线需要 Go 1.26+；xcodebuild 失败是可选 iOS 项检查，不能据此断言必须安装完整 Xcode。
- manual/feed/ai 分别存行；手动排除是墓碑，不能因 AI 重分类消失。
- 双视图共用数据、逻辑顺序与锚点，不能 CSS columns 敷衍；首图和正文图片走同一受控策略。
- 产品 Agent Runtime 在范围内；开发子代理永远禁止；inline 配置保留。
- ADR 均拟定，库能力核对不等于实际集成通过；SQLite 探针不是 modernc／100k 性能认证。
- 旧 00-bootstrap-guidelines 任务未擅自归档，代码规范目前是新约定而非从既有源码归纳。

## 下一步

向用户呈现本轮最终规划摘要。后续新消息明确批准后，加载 trellis-before-dev 与 Phase 1.4，运行 task.py start，开始 Phase 0 最小工程和环境诊断；各阶段按路线图逐步独立验收。先看 RISKS，不跳过 doctor、动态 lanes、中文 FTS、凭据和跨平台门禁。禁止未获批准直接发布或修改全局工具链。
