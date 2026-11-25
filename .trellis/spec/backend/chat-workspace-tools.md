# 会话目录、Skill 与本地工具

## 1. 范围

全局 AI 会话分组、Skill 安装与激活、桌面文件访问及命令执行。

## 2. 入口

迁移 00029 为全局会话记录可空 `group_id` 和固定 `workspace_day`。`Chat.Groups`、`CreateGroup`、`CreateInGroup` 和 `GroupConversations` 使用持久目录与组 ID。运行时的 `agentrun.Workspace` 在 `Chat.Send` 由后端解析并随上下文传入。

## 3. 契约

- 默认输出是 `~/.Fealdhord/会话文件/YYYY-MM-DD/会话ID`，推导路径和发送纯文本不执行磁盘写入，首次写入才能创建缺失祖先。
- `read(path, offset?, limit?, lineByteOffset?)` 受 2,000 行与序列化后 50 KiB 限制，提供续读位置。二进制标为 `binary`。专用结果额度经过普通工具循环和工具目录压缩。
- `write(path, content)` 提交新文件或完整覆盖。`edit(path, oldText, newText, replaceAll?)` 默认只接受唯一匹配。访问基于实际文件根，已安装的 Skill 对文件工具只读。
- `shell(command, workdir?, timeoutMs?)` 只在本机全局 AI 提供。每轮读取本地默认 Shell ID 并解析出绝对路径，模型根据工具说明获知解释器名称。待批准内容绑定原始命令、实际工作目录和解释器路径，批准一次才启动。主机命令的系统权限和工作目录的文件工具范围分别判断。

## 4. 错误与边界

路径越界返回 `PathNotAllowed`，目录无效返回 `DirectoryUnavailable`，写入冲突返回 `FileChanged`。替换未命中返回 `TextNotFound`，存在歧义返回 `TextNotUnique`。审批过期、取消或拒绝时不启动命令。解释器在审批前无法使用时返回 `ShellUnavailable`，等待审批时解释器发生变化则返回 `ShellChanged`。已经启动的命令报告超时或退出结果。审计和聊天工具活动不记录命令、文件正文和原始输出。

## 5. 安装

ZIP 预览暂存与发布目录独立。解析声明并校验归档路径和实际读取大小后才提供预览 ID。保存应复核预览对应的旧版本，失败时保留当前版本。老 JSON 仍可打开，编辑时迁移资源。

## 6. 验证

本地 SQLite 验证组路径与会话 ID 重启恢复。运行时回归包含懒创建、二进制、行与字节分页、精准替换、ZIP 内容保真和命令单次审批。前端回归覆盖三个折叠层级、真实文件选择数据和命令展示。Windows 和 Linux 的交叉编译与实机结果分别记录。

## 7. 权限校验

Skill 文本和模型工具参数均不能创建新的目录授权。文件根由持久化的分组目录或当前会话的固定默认路径产生。MCP 服务端目录只公开既有注册表中的工具，本地工具走独立全局会话入口。
