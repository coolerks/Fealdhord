# 视觉设计系统

首版设计基线，Phase 1 以真实 Wails 窗口验证，不将建议尺寸冒充视觉验收结果。风格为克制、轻、高信息密度、安静的桌面阅读器；参考 Linear／Raycast／Reeder 的精细程度，独立设计。Telegram 只参考反馈密度，不复制配色、聊天气泡或信息架构。

## 语义 Token

以下颜色为候选 HEX 值；实现用 CSS Variables，先测对比度再固定。组件禁止散落硬编码颜色。

| token | Light | Dark |
| --- | --- | --- |
| background | #F5F5F3 | #171918 |
| foreground | #202521 | #E8EBE8 |
| card | #FFFFFF | #212522 |
| popover | #FFFFFF | #252A26 |
| muted | #E9ECE7 | #2B312C |
| muted-foreground | #59635A | #B0BAB1 |
| border | #CDD4CB | #49524A |
| input | #D9DFD6 | #4D574E |
| accent | #E1EADD | #344737 |
| accent-foreground | #243D28 | #E4EFE2 |
| primary | #365E3C | #B6D7AE |
| primary-foreground | #FFFFFF | #182619 |
| destructive | #AB2929 | #FFB4AB |
| sidebar | #EAF0E7 | #202922 |
| sidebar-foreground | #28352B | #DDE8DC |
| sidebar-accent | #D4E1CF | #3C5140 |
| reader-background | #FCFCFA | #1B1E1B |

系统字体优先，界面默认 13px、行高 1.45；正文见 Reader。间距尺度 4／8／12／16／24／32px；控件圆角 6px、面板 10px，避免巨大圆角。图标常用 16px，按钮命中区至少 32×32px，触控模式建议 40px。

文本对比目标普通文字≥4.5:1，大文字／必要图形≥3:1；浅深主题、透明背景上均测。Unread／Error／Selected 同时使用形状／文字／权重，不单靠颜色。Focus-visible 2px 清晰轮廓，不能被 overflow 裁掉。

主题 Light／Dark／System 持久保存；System 运行时跟随 OS，首屏应用正确 token，避免亮色闪屏。减少透明度或无 backdrop-filter 时使用实色 sidebar，保持信息层级。

验收 DS-01：全部语义 token 有两套值、焦点可见、无单靠颜色、首屏和系统主题切换正确；候选色值经真实对比度和三平台 WebView 验证后才标通过。
