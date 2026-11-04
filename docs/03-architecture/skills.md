# Skill 架构

Skill 是声明式指导与工具 allowlist；工具是可执行原子操作。两者分离。内置目录 `skills/<name>/SKILL.md` 与用户数据目录下 skills 各自索引，用户 Skill 不覆盖内置同名内容而不提示。

读取时限制 manifest 与 assets 总大小，拒绝路径穿越、逃逸根目录的 symlink、重复 name 与不支持版本。解析 frontmatter 后，只有允许字段进入运行时；正文作为用户启用的指导，不具有系统权限。asset 只按需作为文本／安全媒体加载，不执行脚本。

编辑采用临时文件写入再原子替换，校验失败保留旧版本；运行开始记录 manifest_hash，以固定快照执行，编辑不改变活动 run。创建／查看／编辑／启用／禁用／删除 UI 与错误见 [Skill 规格](../04-specs/skills.md)。
