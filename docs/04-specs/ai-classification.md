# AI 自动分类规格

Phase 6；依赖三协议与模型分配，不依赖完整 Agent。用户显式开启分类功能、选择模型与分类 Prompt 后才后台调用；展示将向 Provider 发送的字段与用量边界。

单篇请求尽量一次判断全部启用分类；数量超过上下文预算时按稳定分类集合分组，禁止文章数×每个分类无差别调用。上下文为标题、摘要和有界正文文本，不上传封面 URL 或全库。返回 categories[{id,confidence,reason}]、importance、relevance；概率必须在 0..1，category ID 必须在请求集合内。

结构化输出 supported 时使用；不支持时按文本 JSON 严格解析、校验，失败有限重试一次并记录，不推测非法结果。这里是分类结构化结果，不是用 Prompt JSON 模拟工具调用。

缓存键：content_hash + category_set_hash（ID、Prompt、阈值与启用状态）+ Provider ID／revision + model ID／参数 hash + classification_version + user_preferences_hash。无变化不调用；只改变结果展示方式不重算。修改阈值可复用原始评分后重新决策，不强制再次收费。

任务结果落库前重新比较 content／Prompt hash；已过期结果不覆盖新内容。AI 只写 ai 来源，manual 墓碑保留。无匹配是成功；非法分类、超时、取消不写半份结果。Prompt 测试使用最近样本，只返回匹配／不匹配、置信度与理由，不修改正式分类与已读状态。

后台默认一次处理一篇，上限每小时 100 篇和 200k 估算输入 token，用户可调；实际 Provider usage 记录，缺 usage 标未知，不能标成本 0。达到预算暂停并提示，不影响 RSS。

## 验收

- AI-01：同键只请求一次，内容或 Prompt 改变会失效；过期响应被丢弃。
- AI-02：manual excluded 不被 AI 覆盖，非法 ID／概率不写库，无模型不触发请求。
- AI-03：测试 Prompt 不改变正式状态；限额、取消、429 和 malformed JSON 有可理解状态。
