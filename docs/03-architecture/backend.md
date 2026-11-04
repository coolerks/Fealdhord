# Go 后端组织

拟定目录：`internal/app` 为组合入口和服务 DTO；`feed/article/category/search/reader` 为领域服务；`database` 为迁移、事务和查询生成；`ai` 为模型契约和 Adapter；`agent/tools/skills/mcp` 为执行层；`credentials/settings/scheduler` 为平台与后台能力；`netpolicy` 为受控 HTTP。当前仅 internal/app 的基础信息与诊断服务已实现；其余目录按功能阶段创建。

Go 包名小写，按职责划分文件；接口由使用者定义，避免全包共享大接口。跨模块传稳定 ID 或 DTO，不传 sqlc 行结构。所有有 I/O 的方法首参数 `context.Context`，错误使用包装与 `errors.Is/As` 分类，不能忽略失败。

应用服务持有事务边界；仓储只处理 SQL。并行抓取与图片探测不持有数据库事务；写入串行短事务。注入 Clock、HTTPDoer、CredentialStore、Model 便于离线测试。后台 panic 必须记录并结束对应操作，不能把不一致状态伪装成功。

生成 SQL 放 `internal/database/sqlc`，生成绑定使用 Wails 固定版本输出路径；禁止手改生成文件。根 main 仅装配，不堆业务逻辑。通过引用接口解耦，避免循环 import。
