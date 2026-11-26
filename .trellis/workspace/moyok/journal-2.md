# Journal - moyok (Part 2)

> Continuation from `journal-1.md` (archived at ~2000 lines)
> Started: 2026-11-26

---



## Session 78: 文章正文流式翻译
<!-- trellis-session: v=2 fp=14c5d701dac6157b -->

**Date**: 2026-11-26
**Task**: 文章正文流式翻译
**Branch**: `codex/interface-clarity`

### Summary

正文按输入输出预算串行流式组批，按段提交与续译，支持定位漏段和配置批大小。

### Git Commits

| Hash | Message |
|------|---------|
| `e448eac` | feat(reader): 按段流式保存和续译文章正文 |

### Testing

- [OK] Go 全量、vet、相关竞态及前端 231 项回归、类型、lint、构建通过。

### Status

[OK] **Completed**

### Next Steps

- 使用用户实际模型和网关测量《Google Antigravity 完整指南》的首段与全文耗时。


## Session 79: 修复正文翻译首轮预算和逐段回填
<!-- trellis-session: v=2 fp=939f99257f4da168 -->

**Date**: 2026-11-26
**Task**: 修复正文翻译首轮预算和逐段回填
**Branch**: `codex/interface-clarity`

### Summary

首轮输出为译文及推理留出余量，优先使用已配置的低推理档位。完整译段在流中独立落库和发布。同网关六段及 89 段样例均一次完成。Go 全量、竞态、静态检查及阅读器回归通过。

### Git Commits

| Hash | Message |
|------|---------|
| `07b09e3` | fix(reader): 修复首轮翻译预算并即时逐段回填 |

### Status

[OK] **Completed**
