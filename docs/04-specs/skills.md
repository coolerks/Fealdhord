# 声明式 Skill 规格

Phase 9；目录 `skills/<name>/SKILL.md` 与 assets。frontmatter：name（小写字母数字短横线，唯一）、title（非空）、description（非空）、version（整数 1）、tools（逻辑工具名数组）。正文为中文任务指导，可引用安全 assets。

拟定限制：SKILL.md ≤256KiB、单 asset ≤2MiB、总包 ≤10MiB、深度≤8；拒绝绝对路径、`..`、根目录逃逸与可执行脚本引用。未知版本不启用并解释原因。未知工具可保存但显示未解析，运行前要求解决，不悄悄放宽到全部工具。

设置支持查看／创建／编辑／启用／禁用／删除，展示描述、允许工具、更新时间、内置／用户来源。内置 Skill 不直接覆盖，编辑创建用户副本。删除确认后不影响活动 run 的内容快照。Skill 不能运行 shell／JS／Python／binary；MCP 启动能力也不能由 Skill 自动开启。

## 验收

- SKILL-01：合法 manifest 往返保存，缺字段／重复名／未知版本有明确错误，旧文件不被坏编辑覆盖。
- SKILL-02：路径穿越／symlink 逃逸／大文件被拒；allowlist 与用户权限取交集。
- SKILL-03：活动 run 使用固定 manifest_hash，禁用后新 run 不加载，现有历史可追踪版本。
