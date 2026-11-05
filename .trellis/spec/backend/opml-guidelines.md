# OPML 文件能力与事务

## 1. 范围与触发条件

修改 internal/transfer、database/opml.go、原生文件对话框或导入刷新时。

## 2. 签名

绑定仅注册 transfer.API：ChooseImport／ChooseExport／PreviewOPML／ImportOPML／ExportOPML／Discard。Repository 通过 PlanOPML／ImportOPML／OPMLSnapshot 提供一致事务快照；禁止暴露生命周期或路径方法。

## 3. 契约

token用途限定、一次性、8份／10分钟；私有预览4份／10分钟。前端不传计划或路径；确认只传preview_id与选择key。XML10MiB／10000唯一源／100000outline／200000token；分类5层、XML32层、outline16层；不联网，不跟随引用，拒DTD。导入单事务复核配置树摘要，复用身份／关系并集／保留旧名称与手动规则；可选历史继承。刷新在提交后进行，队列失败不能把已保存导入说成回滚。

## 4. 校验与错误矩阵

文档超限→ValidationError；过期或重复能力→PermissionError；文件目标／配置变化→Conflict；context取消→Cancelled；文件错误不返回宿主路径。取消后不能留下调用方拿不到ID的私有预览，Discard同时取消token／previewID对应在途处理。

## 5. 良好、基础和异常案例

空Folder与多分类同源导出后可重建；空库导出有效body。原子替换失败保留旧文件；新目标独占硬链接，文件系统不支持则安全失败，不降级为可能覆盖新文件的Rename。

## 6. 必需测试

真实SQLite往返、去重、恢复、历史范围、配置冲突与事务回滚；原生picker取消、能力用途／一次性／过期、文件变更、在途取消、导出临时文件清理及目标保留；XML实体、层级／数量／大小边界；相关race。真实对话框可用编译验证，不能据此宣称跨平台实测。

## 7. 错误与正确做法

错误：消费token后不登记在途取消，取消的预览仍占满服务端配额；或前端成功获得preview后又Discard其origin token，立即误删有效预览。正确：处理与能力消费同锁登记，取消贯通；成功预览生命周期用preview_id管理，卸载／显式取消时丢弃。导出仅固定白名单字段，写前构造、同目录临时文件Sync和目标复核，不直接覆盖目标。

实际接口及限制见[OPML规格](../../../docs/04-specs/opml.md)。
