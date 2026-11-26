import { SkillMarks } from '@/components/skill-mark'
import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { clearSelectionWithin } from '@/app/TextSelection'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import type { Message, ReplyToolStep, ReplyWork } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'
import { ChatMarkdown } from './ChatMarkdown'
import type { RevealBatch } from './stream-reveal'
import { getToolMeta, summarizeToolActivity } from './tool-meta'

const statusLabels: Record<string, string> = {
  running: '进行中',
  completed: '已完成',
  denied: '未获授权',
  failed: '失败',
  interrupted: '已中断',
  cancelled: '已取消',
  unknown: '结果未知',
}

function statusTone(status: string) {
  switch (status) {
    case 'running':
      return 'text-foreground font-medium'
    case 'failed':
    case 'denied':
      return 'text-destructive'
    case 'cancelled':
    case 'interrupted':
      return 'text-amber-600 dark:text-amber-400'
    default:
      return 'text-muted-foreground'
  }
}

function elapsedLabel(ms: number) {
  const seconds = Math.max(0, Math.round(ms / 1000))
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor(seconds / 60) % 60
  const rest = seconds % 60
  return [hours ? `${hours} 小时` : '', minutes ? `${minutes} 分钟` : '', rest || !seconds ? `${rest} 秒` : ''].filter(Boolean).join(' ')
}

function clearCollapsedSelection(panel: HTMLElement | null) {
  if (panel) clearSelectionWithin(panel)
  else document.dispatchEvent(new Event('selectionchange'))
}

function normalizeEntries(sections: Partial<ReplyWork>[]) {
  return sections.map(entry => ({
    ...entry,
    summary: entry.reasoningSummary?.trim(),
    thinking: entry.reasoningText?.trim(),
  })).filter(entry => entry.summary || entry.thinking || entry.reasoningObserved || entry.reasoningTruncated || entry.tools?.length)
}

const NEXT_ROUND_THINKING_DELAY_MS = 250

// 仅在持续处于等待状态超过阈值时才显示，避免同一轮并行工具交接的毫秒级闪烁；退出等待时立即隐藏。
function useDelayedTrue(active: boolean, delayMs = NEXT_ROUND_THINKING_DELAY_MS) {
  const [visible, setVisible] = useState(false)
  if (!active && visible) setVisible(false)
  useEffect(() => {
    if (!active) return
    const timer = setTimeout(() => setVisible(true), delayMs)
    return () => clearTimeout(timer)
  }, [active, delayMs])
  return active && visible
}

function ThinkingIndicator() {
  return <div className="py-0.5 text-sm text-muted-foreground">
    <span className="chat-thinking-label" role="status">正在思考</span>
  </div>
}

function RetryIndicator({ attempt, max }: { attempt: number; max: number }) {
  return <div className="py-0.5 text-sm text-muted-foreground">
    <span className="chat-thinking-label" role="status">网络或服务异常，正在重试（第 {attempt}/{max} 次）…</span>
  </div>
}

function ToolPayload({ label, text, truncated }: { label: string; text: string; truncated?: boolean }) {
  return <div>
    <div className="mb-1 text-xs text-muted-foreground">{label}{truncated && '（已截断）'}</div>
    <pre className="max-h-40 overflow-auto rounded-md bg-muted p-2 text-xs whitespace-pre-wrap break-all text-foreground">{text || '（空）'}</pre>
  </div>
}

function ToolStep({ tool }: { tool: ReplyToolStep }) {
  const id = useId()
  const panelRef = useRef<HTMLDivElement>(null)
  const [open, setOpen] = useState(false)
  const meta = getToolMeta(tool.name)
  const isRunning = tool.status === 'running'
  const status = statusLabels[tool.status] ?? '状态未知'
  const tone = statusTone(tool.status)
  const hasData = tool.arguments != null || tool.result != null
  const icon = <AnimatedGlyph meaning={meta.icon} className="chat-activity-glyph shrink-0 text-muted-foreground" />
  const content = isRunning
    ? <span className="chat-thinking-label truncate">{meta.runningSummary}</span>
    : <>
        <span className="truncate text-foreground/90">{meta.label}</span>
        <span className={cn('shrink-0', tone)}>{status}</span>
      </>

  if (!hasData) {
    return <li className="flex min-h-7 items-center gap-1.5 text-xs">
      {icon}
      {content}
    </li>
  }

  return <li>
    <Button type="button" variant="ghost" size="sm" className="-ml-2 h-7 max-w-full gap-1.5 border-0 px-2 text-xs font-normal aria-expanded:bg-transparent hover:aria-expanded:bg-muted dark:hover:aria-expanded:bg-muted/50"
      aria-label={isRunning ? meta.runningSummary : `${meta.label} ${status}`} aria-expanded={open} aria-controls={id}
      onClick={() => {
        if (open) clearCollapsedSelection(panelRef.current)
        setOpen(!open)
      }}>
      {icon}
      {content}
      {open ? <ChevronDown data-icon="inline-end" /> : <ChevronRight data-icon="inline-end" />}
    </Button>
    <div ref={panelRef} id={id} hidden={!open}>
      {open && <div className="flex flex-col gap-2 py-1 pl-5">
        {tool.arguments != null && <ToolPayload label="调用参数" text={tool.arguments} truncated={tool.argumentsTruncated} />}
        {tool.result != null && <ToolPayload label="返回值" text={tool.result} truncated={tool.resultTruncated} />}
      </div>}
    </div>
  </li>
}

function ProcessEntries({ entries, running, waiting }: {
  entries: ReturnType<typeof normalizeEntries>
  running: boolean
  waiting: boolean
}) {
  return <div className="flex flex-col gap-1.5">
    {entries.map((entry, index) => <div key={index} className="flex flex-col gap-1.5">
      {entry.summary && <div>
        {entry.thinking && <p className="mb-0.5 text-xs font-medium text-foreground/80">思考摘要</p>}
        <ChatMarkdown text={entry.summary} streaming={running} />
      </div>}
      {entry.thinking && <div>
        {entry.summary && <p className="mb-0.5 text-xs font-medium text-foreground/80">推理过程</p>}
        <p className="whitespace-pre-wrap text-xs leading-relaxed">{entry.thinking}</p>
      </div>}
      {entry.reasoningTruncated && <p className="text-xs">思考内容已截断</p>}
      {!entry.summary && !entry.thinking && entry.reasoningObserved && <p className="text-xs">{waiting ? '模型正在推理' : '模型已执行推理'}</p>}
      {!!entry.tools?.length && <ul className="flex flex-col gap-0.5" aria-label="工具活动">
        {entry.tools.map((tool, toolIndex) => <ToolStep key={toolIndex} tool={tool} />)}
      </ul>}
    </div>)}
  </div>
}

function ActivityRound({ sections, running, waiting, activeRound }: {
  sections: Partial<ReplyWork>[]
  running: boolean
  waiting: boolean
  activeRound: boolean
}) {
  const panelId = useId()
  const panelRef = useRef<HTMLDivElement>(null)
  const entries = normalizeEntries(sections)
  const allTools = entries.flatMap(entry => entry.tools ?? [])
  const toolNames = allTools.map(tool => tool.name)
  const runningTool = running ? [...allTools].reverse().find(tool => tool.status === 'running') : undefined
  const hasRunningTool = Boolean(runningTool)
  const activeReasoningOnly = running && activeRound && allTools.length === 0
  const inProgress = hasRunningTool || activeReasoningOnly
  const [toggle, setToggle] = useState<{ running: boolean; open: boolean }>()
  const open = toggle?.running === running ? toggle.open : inProgress

  useLayoutEffect(() => {
    if (!open) return
    const panel = panelRef.current
    return () => { clearCollapsedSelection(panel) }
  }, [open])

  if (entries.length === 0) return null

  const runningMeta = runningTool ? getToolMeta(runningTool.name) : undefined
  const activity = runningMeta
    ? { text: runningMeta.runningSummary, icon: runningMeta.icon }
    : activeReasoningOnly
      ? { text: '正在思考', icon: 'sparkles' as const }
      : summarizeToolActivity(toolNames)

  return <div className="text-sm text-muted-foreground">
    <Button type="button" variant="ghost" size="sm"
      className="-ml-2 h-7 max-w-full gap-1.5 border-0 px-2 text-xs font-normal text-muted-foreground hover:text-foreground aria-expanded:bg-transparent aria-expanded:text-muted-foreground hover:aria-expanded:bg-muted hover:aria-expanded:text-foreground dark:hover:aria-expanded:bg-muted/50"
      aria-controls={panelId} aria-expanded={open} onClick={() => {
        if (open) clearCollapsedSelection(panelRef.current)
        setToggle({ running, open: !open })
      }}>
      <AnimatedGlyph meaning={activity.icon} className="chat-activity-glyph shrink-0" />
      <span className={cn('truncate', inProgress && 'chat-thinking-label')} role={inProgress ? 'status' : undefined}>{activity.text}</span>
      {open ? <ChevronDown data-icon="inline-end" /> : <ChevronRight data-icon="inline-end" />}
    </Button>
    <div ref={panelRef} id={panelId} hidden={!open} className="mt-1 max-h-56 overflow-y-auto border-l border-border/80 pl-3">
      {open && <ProcessEntries entries={entries} running={running} waiting={waiting} />}
    </div>
  </div>
}

function SingleStageProgress({ message }: { message: Message }) {
  const panelId = useId()
  const panelRef = useRef<HTMLDivElement>(null)
  const [toggle, setToggle] = useState<{ running: boolean; open: boolean }>()
  const running = message.status === 'running'
  const hasText = Boolean(message.text.trim())
  const work = message.details?.work
  const section: Partial<ReplyWork> = { ...work, tools: work?.tools ?? message.tools }
  const entries = normalizeEntries([section])
  const hasProcess = entries.length > 0
  const retrying = Boolean(running && work?.retryAttempt && work.retryMax)
  const hasRunningTool = running && entries.some(entry => entry.tools?.some(tool => tool.status === 'running'))
  const hasAnyTools = Boolean(section.tools?.length)
  const showNextRoundThinking = useDelayedTrue(running && !retrying && !hasRunningTool && !hasText && hasAnyTools)
  const open = toggle?.running === running ? toggle.open : false

  useLayoutEffect(() => {
    if (!open) return
    const panel = panelRef.current
    return () => { clearCollapsedSelection(panel) }
  }, [open])

  if (running) {
    if (retrying) {
      return <>
        {hasProcess && <ActivityRound sections={[section]} running={running} waiting={!hasText} activeRound={false} />}
        <RetryIndicator attempt={work!.retryAttempt!} max={work!.retryMax!} />
      </>
    }
    if (!hasText && !hasProcess) return <ThinkingIndicator />
    if (!hasProcess) return null
    return <>
      <ActivityRound sections={[section]} running={running} waiting={!hasText} activeRound={!hasText} />
      {showNextRoundThinking && <ThinkingIndicator />}
    </>
  }

  if (!hasProcess) return null

  const duration = work?.durationMs ?? message.details?.durationMs
  const label = typeof duration === 'number' ? `用时 ${elapsedLabel(duration)}` : '查看记录'

  return <div className="text-sm text-muted-foreground">
    <Button type="button" variant="ghost" size="sm" className="-ml-2 border-0 px-2 text-muted-foreground hover:text-foreground aria-expanded:bg-transparent aria-expanded:text-muted-foreground hover:aria-expanded:bg-muted hover:aria-expanded:text-foreground dark:hover:aria-expanded:bg-muted/50"
      aria-controls={panelId} aria-expanded={open} onClick={() => {
        if (open) clearCollapsedSelection(panelRef.current)
        setToggle({ running, open: !open })
      }}>
      <span>{label}</span>
      {open ? <ChevronDown data-icon="inline-end" /> : <ChevronRight data-icon="inline-end" />}
    </Button>
    <div ref={panelRef} id={panelId} hidden={!open} className="mt-1 max-h-56 overflow-y-auto border-l border-border/80 pl-3">
      {open && <ProcessEntries entries={entries} running={false} waiting={false} />}
    </div>
  </div>
}

export function ReplyBody({ message, markdown, reveal }: {
  message: Message; markdown: boolean; reveal?: readonly RevealBatch[]
}) {
  const work = message.details?.work
  const positions = work?.positions
  const running = message.status === 'running'
  const waiting = running && !message.text.trim()
  const panelId = useId()
  const panelRef = useRef<HTMLDivElement>(null)
  const [toggle, setToggle] = useState<{ running: boolean; open: boolean }>()

  function textBetween(start: number, end: number) {
    const text = message.text.slice(start, end)
    if (!text) return null
    if (!markdown) return <p className="whitespace-pre-wrap break-words">{text}</p>
    const batches = reveal?.filter(batch => batch.end > start && batch.start < end)
      .map(batch => ({ ...batch, start: Math.max(batch.start, start) - start, end: Math.min(batch.end, end) - start }))
    return <ChatMarkdown text={text} streaming={running && end === message.text.length} reveal={batches?.length ? batches : undefined} />
  }

  const hasMultiStage = Boolean(work && positions?.length)
  const finalOffset = hasMultiStage ? positions![positions!.length - 1].offset : 0
  const hasFinalText = Boolean(message.text.slice(finalOffset).trim())
  const retrying = Boolean(running && work?.retryAttempt && work.retryMax)
  const hasRunningTool = Boolean(running && hasMultiStage && work!.tools?.some(tool => tool.status === 'running'))
  const hasAnyTools = Boolean(hasMultiStage && work!.tools?.length)
  const showNextRoundThinking = useDelayedTrue(running && !retrying && hasMultiStage && !hasRunningTool && !hasFinalText && hasAnyTools)
  const open = toggle?.running === running ? toggle.open : !hasFinalText

  useLayoutEffect(() => {
    if (!hasMultiStage || running || !open) return
    const panel = panelRef.current
    return () => { clearCollapsedSelection(panel) }
  }, [hasMultiStage, running, open])

  const steers = work?.steers ?? []
  const steerBanner = steers.length > 0 && (
    <div className="space-y-1">
      {steers.map((steer, index) => (
        <div key={`${steer.at}-${index}`} className="flex items-start gap-1.5 rounded-lg border border-border/80 bg-muted/45 px-2.5 py-1.5 text-xs text-muted-foreground">
          <span className="shrink-0 font-medium text-foreground">中途引导</span>
          <div className="min-w-0 break-words"><SkillMarks skills={steer.skills} />{steer.text}</div>
        </div>
      ))}
    </div>
  )

  if (!hasMultiStage) {
    const body = textBetween(0, message.text.length)
    return <div className="flex flex-col gap-2">
      <SingleStageProgress message={message} />
      {steerBanner}
      {body}
    </div>
  }

  const groups: { offset: number; sections: ReplyWork[] }[] = []
  positions!.forEach((position, i) => {
    const previous = positions![i - 1]
    const last = groups.at(-1)
    const group = last?.offset === position.offset ? last : { offset: position.offset, sections: [] }
    if (group !== last) groups.push(group)
    group.sections.push({
      durationMs: position.durationMs,
      reasoningObserved: false,
      reasoningText: work!.reasoningText?.slice(previous?.reasoningEnd ?? 0, position.reasoningEnd),
      reasoningSummary: work!.reasoningSummary?.slice(previous?.summaryEnd ?? 0, position.summaryEnd),
      reasoningTruncated: i === positions!.length - 1 && work!.reasoningTruncated,
      tools: work!.tools?.slice(previous?.toolEnd ?? 0, position.toolEnd) ?? [],
    })
  })

  const finalBody = textBetween(finalOffset, message.text.length)

  if (running) {
    return <div className="flex flex-col gap-2">
      {groups.map((group, i) => {
        const before = textBetween(groups[i - 1]?.offset ?? 0, group.offset)
        const isLast = i === groups.length - 1
        return <div key={group.offset} className="contents">
          {before}
          <ActivityRound sections={group.sections} running={running} waiting={waiting} activeRound={isLast && !hasFinalText} />
        </div>
      })}
      {retrying
        ? <RetryIndicator attempt={work!.retryAttempt!} max={work!.retryMax!} />
        : showNextRoundThinking && <ThinkingIndicator />}
      {steerBanner}
      {finalBody}
    </div>
  }

  const totalDuration = message.details?.durationMs ?? work!.durationMs
  const label = typeof totalDuration === 'number'
    ? `用时 ${elapsedLabel(totalDuration)}`
    : '查看工作过程'

  return <div className="flex flex-col gap-2">
    <div>
      <Button type="button" variant="ghost" size="sm" className="-ml-2 border-0 px-2 text-muted-foreground hover:text-foreground aria-expanded:bg-transparent aria-expanded:text-muted-foreground hover:aria-expanded:bg-muted hover:aria-expanded:text-foreground dark:hover:aria-expanded:bg-muted/50"
        aria-controls={panelId} aria-expanded={open} onClick={() => {
          if (open) clearCollapsedSelection(panelRef.current)
          setToggle({ running, open: !open })
        }}>
        <span>{label}</span>
        {open ? <ChevronDown data-icon="inline-end" /> : <ChevronRight data-icon="inline-end" />}
      </Button>
      <div ref={panelRef} id={panelId} hidden={!open} className="mt-2 flex flex-col gap-2">
        {open && groups.map((group, i) => {
          const before = textBetween(groups[i - 1]?.offset ?? 0, group.offset)
          return <div key={group.offset} className="contents">
            {before}
            <ActivityRound sections={group.sections} running={false} waiting={false} activeRound={false} />
          </div>
        })}
      </div>
    </div>
    {steerBanner}
    {finalBody}
  </div>
}
