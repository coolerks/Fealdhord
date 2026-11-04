# 设置与默认值

设置采用分区导航和右侧表单；范围未交付时标注阶段，不用可点击但无结果的假开关。普通配置离开字段后保存或显式保存；凭据与服务连接设置必须显式保存，失败保留输入并解释，不误报成功。

| 分区 | 首版内容与建议默认 |
| --- | --- |
| General | 启动与刷新策略；不开系统登录启动、不请求通知权限 |
| Appearance | Theme=System；Animation=Full；Follow System Reduce Motion=true |
| Reader | 字体、17px、1.7 行高、720px；打开正文成功后已读=true |
| Article View | List；Compact；Thumbnail=None；Masonry ratio=Original |
| Media & Image Loading | Subscriptions Only；网页补图=false；缓存上限与清除 |
| Animation / Reduced Motion | Full／Subtle／Off 与系统优先级说明 |
| Feeds | 添加、编辑、归档、刷新、分类与错误源 |
| AI Providers | 多服务、三协议、Key 与高级兼容配置 |
| Models | 模型能力与五类任务分配 |
| AI Classification | 默认关闭；分类 Prompt、阈值、样本测试与预算 |
| Skills | 查看／创建／编辑／启用／禁用／删除 |
| MCP | 默认无连接，Server 关闭；工具／资源／Prompt 与权限 |
| Tools and Permissions | 默认 read auto、write ask、destructive ask、external 写 ask |
| Data | OPML、备份、清缓存、清执行历史；首版不提供未设计的全库永久删除 |
| About | 版本、许可证、数据目录诊断与文档 |

上述默认值是本轮可逆方案，不是用户逐项确认。布局与阅读偏好存 SQLite，不因重启丢失；运行中切主题即时生效。危险数据动作显示影响与确认；不得输出 secret 到诊断导出。

验收 SETTINGS-01：非法旧值回退默认，保存失败显示错误；无 Key 可使用 RSS；键盘切分区、表单 label、错误关联和焦点恢复正确。
