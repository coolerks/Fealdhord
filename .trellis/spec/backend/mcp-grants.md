# MCP 客户端授权

## 1. 范围与触发条件

修改「MCP 与权限」的客户端授权表单、Wails 绑定或 MCP 服务端的授权持久化时。

## 2. 签名

- `SaveGrant(grant MCPGrant, revoke bool) (string, error)` 创建或撤销授权。
- `UpdateGrant(grant MCPGrant) error` 更新已存在的工具及数据范围。
- `RotateGrantToken(name string) (string, error)` 更换指定客户端令牌。

## 3. 契约

`MCPGrant` 传递 `name`、`tools` 和 `scope`。新建和主动更换时仅响应一次明文令牌，服务端存储摘要。编辑保留现有摘要，轮换保留工具和范围。持久化成功后切换在线权限，并刷新前端授权列表。

## 4. 校验与错误

同名新建返回 `ClientGrantExists`。编辑或轮换不存在的授权返回 `ClientGrantNotFound`。范围无效返回 `InvalidClientScope`，工具须属于服务端开放集合且适用于范围，否则返回 `UnknownTool`。持久化失败沿用原授权。

## 5. 案例

基础案例是新建客户端并复制对应行的令牌。收窄现有授权后，原令牌只能看到新范围内的工具。尝试以同名新建时，现有权限和令牌继续生效。

## 6. 回归测试

用回环 MCP 连接验证编辑和重启后原令牌有效、工具列表按更新后的授权返回。显式更换后断言旧令牌得到 HTTP 401、新令牌可连接且权限保持。

## 7. 错误与正确做法

错误做法是通过 `SaveGrant(existing, false)` 编辑现有授权。编辑界面调用 `UpdateGrant`，确认更换界面调用 `RotateGrantToken` 并在目标客户端处展示新令牌。
