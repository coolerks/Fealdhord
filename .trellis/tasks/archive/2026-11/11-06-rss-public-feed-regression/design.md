# 修复设计

复用同一 DNS 校验函数筛出可拨号的公网 IP。保留 DNS 顺序、固定地址拨号、Host、TLS SNI、跳转复检及代理禁用。过滤后为空返回原 PermissionError。同步共享网关的安全契约。
