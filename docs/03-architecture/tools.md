# Tool Registry 与执行层

统一 Registry 接入内置与 MCP 工具；注册时校验名称唯一、输入／输出 Schema、权限声明与来源。可用工具是系统允许集合、用户策略、Skill allowlist、当前 scope 的交集。模型看到工具不等于拥有执行权。

权限由多个风险维度组成，read／write／destructive 表示本地效果，external 为外部边界附加属性；不能因一个 external 枚举而丢失 destructive 风险。策略以最严格匹配生效：禁止 > 必须询问 > 允许。MCP 自述 readOnlyHint 等不能作为安全授权。

执行先验证参数、目标是否存在和作用域，再评估权限；等待审批期间不调用工具。统一输出结构、审计、限长与错误。业务工具调用应用服务，不直接写表；敏感设置只暴露脱敏白名单。工具名稳定，供应商需要受限字符时通过单射编码映射，不能直接截断造成冲突。

完整清单、参数和验收见 [Tools 规格](../04-specs/tools.md) 与 [Tool 契约](../05-api/tool-contract.md)。
