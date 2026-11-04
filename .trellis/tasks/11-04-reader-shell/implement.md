# Phase 1 执行清单

- [x] 复用已批准 Phase 1 方案，建立任务并读取相关规范。
- [x] 最小 SQLite 设置、迁移／sqlc、SettingsService 与持久化单元测试完成；绑定生成后提交。
- [ ] Tailwind／shadcn／Lucide／Query／Zustand／Virtual／motion 及正式 Shell、主题与偏好恢复。
- [ ] 真实虚拟双视图、Reader 占位／抽屉、锚点、语义动效、设置／Palette 与键盘。
- [ ] 必要组件／单元测试、类型／lint／原生构建；不运行非必要 E2E。
- [ ] 更新中文契约、CURRENT／Handoff、完成提交与归档。

变更边界：main.go 装配设置服务，internal/database／settings、migrations／sqlc 配置，frontend/src 拆分 Shell 并保留 Phase 0 诊断、组件与必要依赖，相关文档。不存在未知业务代码的重构。

验证：go test ./internal/database/... ./internal/settings/...、go vet ./...，前端 typecheck／lint／test，wails3 build。高风险点为 SQLite 并发 revision、初次恢复覆盖、虚拟 lanes／锚点、Reduced Motion 和分隔条键盘。
