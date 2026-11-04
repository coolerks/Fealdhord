# 规格阶段历史核验记录

日期：2026-11-04。本文保留规格阶段真实执行结果；随后应用 Phase 0 已实施，最新验收见 [Phase 0](phase0-validation.md)。

## 环境检查

执行 Go、Node、npm、pnpm、Wails 二进制元数据、wails3 doctor／doctor -json、sqlc、sqlite3、Xcode CLT 命令。结果详见 environment.md：doctor 两次退出 1；其他工具存在不代表 GUI 可运行。未安装或升级工具、未读写真实凭据、未调用收费模型。

## 离线语义探针

命令：`python3 docs/09-research/probes/sqlite_search_probe.py`。结果：全部断言通过；Python sqlite3 3.54.1，unicode61 的中文双字子串零命中、trigram 双字零命中而三字命中，显式单字／二字 token 命中，三来源独立保存且 manual excluded 有效。限制：内存小样本，不是 Go 驱动、实际数据规模或产品性能测试。

## 文档静态检查

命令：`python3 docs/09-research/probes/validate_docs.py`。检查源文件哈希、113 节完整顺序映射、唯一 ID、目标路径、验收 ID、Markdown 本地链接、中文标题、占位、七节契约、12 份 ADR、planning 状态、inline 与禁止子代理约束。最终输出见 document-validation-result.json：核验 101 份 Markdown、113 项追踪、218 个本地链接、15 份七节契约与 12 份 ADR，错误为 0。原始快照单独校验哈希，保留原文示例格式。另执行 git diff --check，已跟踪文件无空白错误；新增文档由核验程序检查行尾空白与代码围栏。

机器可核验结果：[文档检查](document-validation-result.json)、[SQLite 探针](sqlite-probe-result.json)。人工复核了阶段顺序、数据所有权、FTS 外部内容列、审批时限、删除语义、媒体禁用范围及官方事实与拟定决定的区分。

## 未执行

没有应用工程，因此 go test／vet、前端 typecheck／lint／组件／E2E、Wails build／launch、五平台、100k 性能、SecretStore、实际 Adapter／MCP 互操作均未执行；不标记通过。官方文档研究只能支持设计依据，不能替代运行验证。
