# 仓库与生成物规范

说明文档中文，代码标识符稳定；所有工作主会话独立，禁止辅助代理。

Phase 0 创建 .gitignore，至少覆盖 .env 与私密变体、运行数据库及 WAL／SHM、缓存、node_modules、frontend/dist、build/bin、打包产物、日志和本机 IDE 临时文件。允许无秘密的 .env.example；数据库迁移、sqlc 查询、合法 fixture、锁文件与可复现构建配置进入版本控制。不能为省事忽略整个 docs 或 .trellis，也不删除已有未跟踪配置。

固定 Go／Node／包管理器／Wails／sqlc／goose 版本，避免 @latest 在 CI 漂移；具体固定值在对应阶段验证后更新，不使用本机 Codex fallback 路径。禁止把绝对开发机路径写进产品启动脚本。生成文件注明来源并通过再生成一致性检查，不手改；变更 schema／API 时同时更新生成物与文档。

验收 REPO-01：git 状态无意外密钥／运行 DB／缓存／产物；新的环境按 README 可复现，生成器无非预期差异。当前尚未生成应用工程，该门禁未执行。
