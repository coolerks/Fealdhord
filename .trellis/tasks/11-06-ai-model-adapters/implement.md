# 实施与验收

先建立契约和 HTTP Mock，再逐个实现 Adapter。三协议覆盖文本、非流式、工具往返、交错参数、未知事件、坏流、认证、限流、取消和 EOF。

通过相关检查后独立提交，并同步父任务验收记录。

## 已完成的文本路径

Phase 5 已接入 Chat Completions 的单轮文本适配器，支持非流式响应和流式事件、用量、拒绝及取消。已通过本地 HTTP 协议测试。工具调用、Responses、Messages、Provider 设置和桌面 Chat 待实现。

下一步完善 Chat Completions 工具调用与结果续轮，再分别实现 Responses 和 Messages。
