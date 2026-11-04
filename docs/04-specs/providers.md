# Provider 与模型设置规格

Phase 5。Provider 字段 id、name、protocol、base_url、credential_ref、enabled、timestamps；Model 独立关联 Provider，记录 model、display_name、capabilities 与 enabled。聊天／分类／摘要／翻译／精选分别选择模型。

添加服务默认仅显示名称、API 类型、Base URL 和 Key 输入；高级选项折叠：Custom Headers、Endpoint Path、Query Parameters、Extra Body、Timeout、禁用参数。UI 帮助明确三种 endpoint；永不出现旧 `/v1/completions`。

测试连接是用户显式发起的最小请求，预先说明可能产生费用；模型列表 endpoint 不统一，允许手动填模型，不能因 `/models` 不存在判定整个服务不可用。能力探测可选，标注 unknown／检测值／手动覆盖，不能偷偷探测收费工具。测试请求可取消，错误脱敏。

Key 提交后进入 CredentialStore；配置回显只返回 has_credential，不回传实际密钥。保存新 key 后先写新引用，DB 提交再回收旧引用，失败补偿。删除 Provider 需提示受影响模型分配和历史引用；历史保留显示名快照，新的运行要求重新配置。

## 验收

- PROVIDER-01：多个服务与模型可独立保存、禁用、分配；缺模型返回 ConfigurationError。
- PROVIDER-02：Base URL 带／不带 v1、自定义路径、header、参数禁用均有 Mock 请求断言；敏感信息不进入 DB／日志／缓存。
- PROVIDER-03：三 Adapter 各通过完整 wire contract 测试；unsupported 能力返回明确错误，不模拟工具协议。
