import { useEffect, useMemo, useRef, useState } from 'react'
import { Popover } from 'radix-ui'
import { AnimatedGlyph } from '@/components/animated-glyph'
import type { IconMeaning } from '@/components/animated-glyph'
import { CustomIcon, pascalToKebab, resolveAnimatedIcon, useLucideRegistry } from '@/components/custom-icon'
import { animatedIcons } from '@/components/icon-catalog'
import type { AnimatedIconName } from '@/components/icon-catalog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { SegmentedControl } from '@/components/ui/segmented-control'
import { ToggleGroupItem } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'

const animatedMeta: Partial<Record<AnimatedIconName, string>> = {
  'activity': '动态 脉冲 趋势',
  'arrow-down-0-1': '排序 升序 数字',
  'arrow-down-1-0': '排序 降序 数字',
  'badge-alert': '警告 提醒 异常',
  'bell': '通知 铃铛 提醒',
  'blocks': '模块 积木 扩展 技能',
  'book-text': '图书 文章 阅读 书籍',
  'bookmark': '收藏 书签 星标',
  'bot': '机器人 AI 智能',
  'box': '盒子 归档 包裹',
  'check-check': '已读 完成 双勾',
  'check': '勾选 确认 完成',
  'chevron-left': '左箭头 返回',
  'chevron-right': '右箭头 展开',
  'circle-check': '成功 完成 勾选',
  'circle-dashed': '未读 虚线圆 待办',
  'circle-help': '帮助 问号 关于',
  'clipboard-check': '剪贴板 清单',
  'clock': '时钟 时间 定时 日程',
  'copy': '复制 文档',
  'database-backup': '数据库 备份 存储',
  'delete': '删除 退格 清理',
  'download': '下载 导出',
  'ellipsis': '更多 菜单',
  'external-link': '外链 网页 链接',
  'flask': '实验 科学 测试 化学',
  'folder-open': '文件夹 目录 分类',
  'folder-tree': '分类树 目录 层级',
  'history': '历史 记录 时间',
  'key': '密钥 钥匙 安全 密码',
  'languages': '翻译 语言 国际',
  'layers': '图层 列表 堆叠',
  'layout-grid': '网格 瀑布流 视图',
  'maximize': '最大化 全屏',
  'message-square': '消息 对话 聊天 评论',
  'minimize': '最小化 收起',
  'monitor-cog': '系统 电脑 显示器 科技',
  'moon': '深色 月亮 夜间',
  'palette': '外观 调色板 设计 美术',
  'panel-left-close': '收起侧栏',
  'panel-left-open': '展开侧栏',
  'panel-right-open': '右侧面板 抽屉',
  'plus': '添加 新建 加号',
  'radio-tower': '订阅 信号 RSS 广播',
  'redo': '重做 前进',
  'refresh-cw': '刷新 同步 更新',
  'scan-text': '阅读 文本 扫描',
  'search': '搜索 查找 放大镜',
  'send': '发送 推送 分享',
  'server': '服务器 后端 托管',
  'settings': '设置 齿轮 配置',
  'sliders-horizontal': '排版 滑块 调节 参数',
  'sparkles': '精选 星光 AI 灵感',
  'sun': '浅色 太阳 日间',
  'undo': '撤销 返回',
  'upload': '上传 导入',
  'webhook': '接口 连接 MCP 钩子',
  'wrench': '工具 扳手 维修 开发',
  'x': '关闭 取消',
}

const animatedList = (Object.keys(animatedIcons) as AnimatedIconName[]).map(name => ({
  name,
  search: `${name} ${animatedMeta[name] ?? ''}`.toLowerCase(),
  title: animatedMeta[name]?.split(' ')[0] ?? name,
}))

const staticKeywords: Record<string, string> = {
  Globe: '全球 网页 世界 国际 互联网',
  Newspaper: '新闻 报纸 资讯 媒体',
  Cpu: '芯片 硬件 科技 计算 处理器',
  Code: '代码 编程 开发 技术',
  CodeXml: '代码 编程 开发 前端',
  Terminal: '终端 命令行 开发 技术',
  Flame: '热门 火焰 精选 流行',
  Star: '星标 收藏 评分 精选',
  Heart: '喜欢 收藏 爱心 生活',
  Music: '音乐 音频 娱乐',
  Film: '电影 视频 影视 娱乐',
  Camera: '摄影 相机 图片 视觉',
  Gamepad2: '游戏 电竞 娱乐',
  Briefcase: '商业 职场 工作 财经',
  TrendingUp: '财经 股票 趋势 增长 投资',
  GraduationCap: '教育 学习 学术 课程',
  Lightbulb: '灵感 想法 创意 思考',
  Rocket: '创业 火箭 发布 科技',
  Shield: '安全 隐私 防护',
  Coffee: '生活 咖啡 日常 随笔',
  Compass: '发现 探索 指南 旅行',
  Rss: '订阅 RSS 资讯',
  Tag: '标签 分类',
  Hash: '话题 标签 频道',
  Brain: 'AI 智能 思考 认知',
  Atom: '科学 物理 技术',
  Podcast: '播客 音频 电台',
  PenTool: '设计 写作 创作',
  BookOpen: '阅读 书籍 文档',
  Folder: '文件夹 分类 目录',
}

const PAGE_SIZE = 120

export function IconPicker({
  id,
  value,
  onChange,
  fallback = 'folder',
  disabled = false,
  label = '图标',
  className,
}: {
  id?: string
  value: string
  onChange: (next: string) => void
  fallback?: IconMeaning
  disabled?: boolean
  label?: string
  className?: string
}) {
  const [open, setOpen] = useState(false)
  const [tab, setTab] = useState<'animated' | 'static'>('animated')
  const [query, setQuery] = useState('')
  const [limit, setLimit] = useState(PAGE_SIZE)
  const searchRef = useRef<HTMLInputElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const sentinelRef = useRef<HTMLDivElement>(null)

  const lucideRegistry = useLucideRegistry(open && tab === 'static')

  const staticList = useMemo(() => {
    if (!lucideRegistry) return []
    return Object.keys(lucideRegistry).map(name => {
      const kebab = pascalToKebab(name)
      return {
        name,
        value: `lucide:${name}`,
        kebab,
        search: `${name} ${kebab} ${staticKeywords[name] ?? ''}`.toLowerCase(),
      }
    })
  }, [lucideRegistry])

  const term = query.trim().toLowerCase()

  const filteredAnimated = useMemo(() => {
    if (!term) return animatedList
    return animatedList.filter(item => item.search.includes(term))
  }, [term])

  const filteredStatic = useMemo(() => {
    if (!term) return staticList
    return staticList.filter(item => item.search.includes(term))
  }, [staticList, term])

  const totalInTab = tab === 'animated' ? filteredAnimated.length : filteredStatic.length
  const hasMore = totalInTab > limit

  useEffect(() => {
    if (!open || !hasMore) return
    const sentinel = sentinelRef.current
    const root = scrollRef.current
    if (!sentinel || !root || typeof IntersectionObserver === 'undefined') return
    const observer = new IntersectionObserver(
      entries => {
        if (entries.some(entry => entry.isIntersecting)) {
          setLimit(current => current + PAGE_SIZE)
        }
      },
      { root, rootMargin: '120px' },
    )
    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [open, hasMore, tab, term, limit])

  const activeAnimated = resolveAnimatedIcon(value)
  const displayName = value.startsWith('lucide:')
    ? pascalToKebab(value.slice(7))
    : (activeAnimated ? (animatedMeta[activeAnimated]?.split(' ')[0] ?? activeAnimated) : (animatedMeta[resolveAnimatedIcon(fallback) ?? 'folder-open']?.split(' ')[0] ?? fallback))

  return (
    <Popover.Root
      open={open && !disabled}
      onOpenChange={next => {
        setOpen(next)
        if (next) {
          setTab('animated')
          setQuery('')
          setLimit(PAGE_SIZE)
        }
      }}
      modal
    >
      <Popover.Trigger asChild>
        <Button
          id={id}
          type="button"
          variant="outline"
          disabled={disabled}
          aria-label={`选择${label}`}
          className={cn('justify-between gap-2 font-normal', className)}
        >
          <span className="inline-flex min-w-0 items-center gap-2">
            <CustomIcon value={value} fallback={fallback} className="size-4 [&_svg]:size-4" />
            <span className="truncate text-xs text-muted-foreground">{displayName}</span>
          </span>
          <AnimatedGlyph meaning="chevron-right" className="rotate-90 text-muted-foreground" />
        </Button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          align="start"
          sideOffset={6}
          className="z-50 w-80 space-y-2.5 rounded-xl border bg-popover p-3 text-popover-foreground shadow-md"
          onOpenAutoFocus={event => {
            event.preventDefault()
            searchRef.current?.focus({ preventScroll: true })
          }}
        >
          <div className="flex items-center justify-between gap-2">
            <SegmentedControl
              aria-label="图标类型"
              value={tab}
              onValueChange={next => {
                if (next === 'animated' || next === 'static') {
                  setTab(next)
                  setLimit(PAGE_SIZE)
                }
              }}
            >
              <ToggleGroupItem value="animated">动态图标</ToggleGroupItem>
              <ToggleGroupItem value="static">静态图标</ToggleGroupItem>
            </SegmentedControl>
            <span className="text-[11px] text-muted-foreground tabular-nums">
              {tab === 'animated' ? `${filteredAnimated.length} 个` : lucideRegistry ? `${filteredStatic.length} 个` : '加载中…'}
            </span>
          </div>

          <Input
            ref={searchRef}
            type="search"
            aria-label="搜索图标"
            placeholder="搜索图标名称或关键词…"
            value={query}
            onChange={event => {
              setQuery(event.target.value)
              setLimit(PAGE_SIZE)
            }}
            className="h-8 text-xs"
          />

          <div
            ref={scrollRef}
            className="max-h-60 overflow-y-auto pr-0.5"
            onScroll={event => {
              if (!hasMore) return
              const el = event.currentTarget
              if (el.scrollTop + el.clientHeight >= el.scrollHeight - 120) {
                setLimit(current => current + PAGE_SIZE)
              }
            }}
          >
            {tab === 'animated' ? (
              filteredAnimated.length === 0 ? (
                <p className="py-6 text-center text-xs text-muted-foreground">没有匹配的动态图标</p>
              ) : (
                <div className="grid grid-cols-6 gap-1">
                  {filteredAnimated.slice(0, limit).map(item => {
                    const selected = activeAnimated === item.name
                    return (
                      <button
                        key={item.name}
                        type="button"
                        title={`${item.title} (${item.name})`}
                        aria-label={`${item.title} (${item.name})`}
                        aria-pressed={selected}
                        onClick={() => {
                          onChange(item.name)
                          setOpen(false)
                        }}
                        className={cn(
                          'flex h-10 items-center justify-center rounded-lg border border-transparent text-foreground transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
                          selected && 'border-ring bg-accent text-accent-foreground',
                        )}
                      >
                        <AnimatedGlyph meaning={item.name} className="size-4 [&_svg]:size-4" />
                      </button>
                    )
                  })}
                </div>
              )
            ) : !lucideRegistry ? (
              <p role="status" className="py-6 text-center text-xs text-muted-foreground">正在加载静态图标…</p>
            ) : filteredStatic.length === 0 ? (
              <p className="py-6 text-center text-xs text-muted-foreground">没有匹配的静态图标</p>
            ) : (
              <div className="grid grid-cols-6 gap-1">
                {filteredStatic.slice(0, limit).map(item => {
                  const IconComponent = lucideRegistry[item.name]
                  const selected = value === item.value
                  return (
                    <button
                      key={item.name}
                      type="button"
                      title={item.kebab}
                      aria-label={item.kebab}
                      aria-pressed={selected}
                      onClick={() => {
                        onChange(item.value)
                        setOpen(false)
                      }}
                      className={cn(
                        'flex h-10 items-center justify-center rounded-lg border border-transparent text-foreground transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
                        selected && 'border-ring bg-accent text-accent-foreground',
                      )}
                    >
                      <IconComponent className="size-4" strokeWidth={1.7} aria-hidden="true" />
                    </button>
                  )
                })}
              </div>
            )}
            {hasMore && <div ref={sentinelRef} className="h-4" aria-hidden="true" />}
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}
