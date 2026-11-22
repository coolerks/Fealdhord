import { useId, useLayoutEffect, useRef, useState } from 'react'
import { ChevronDown, ChevronRight, Loader2 } from 'lucide-react'
import { clearSelectionWithin } from '@/app/TextSelection'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import type { Message, ReplyToolStep, ReplyWork } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'
import { ChatMarkdown } from './ChatMarkdown'
import type { RevealBatch } from './stream-reveal'
import { getToolMeta } from './tool-meta'

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
  const Icon = tool.status === 'running' ? Loader2 : meta.icon
  const status = statusLabels[tool.status] ?? '状态未知'
  const tone = statusTone(tool.status)
  const hasData = tool.arguments != null || tool.result != null
  const iconClass = cn('size-3.5 shrink-0', tool.status === 'running' ? 'chat-spin text-foreground' : 'text-muted-foreground')

  if (!hasData) {
    return <li className="flex min-h-7 items-center gap-1.5 text-xs">
      <Icon className={iconClass} aria-hidden="true" />
      <span className="truncate text-foreground/90">{meta.label}</span>
      <span className={cn('shrink-0', tone)}>{status}</span>
    </li>
  }

  return <li>
    <Button type="button" variant="ghost" size="sm" className="-ml-2 h-7 max-w-full gap-1.5 px-2 text-xs font-normal"
      aria-label={`${meta.label} ${status}`} aria-expanded={open} aria-controls={id}
      onClick={() => {
        if (open) clearCollapsedSelection(panelRef.current)
        setOpen(!open)
      }}>
      <Icon className={iconClass} aria-hidden="true" />
      <span className="truncate text-foreground/90">{meta.label}</span>
      <span className={cn('shrink-0', tone)}>{status}</span>
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

function ReplyProgress({ message, sections }: {
  message: Message; sections?: ReplyWork[]
}) {
  const panelId = useId()
  const panelRef = useRef<HTMLDivElement>(null)
  const [toggle, setToggle] = useState<{ running: boolean; open: boolean }>()
  const running = message.status === 'running'
  const waiting = running && !message.text.trim()
  const work = message.details?.work
  const entries = (sections ?? [{ ...work, tools: work?.tools ?? message.tools }]).map(entry => ({
    ...entry, summary: entry.reasoningSummary?.trim(), thinking: entry.reasoningText?.trim(),
  })).filter(entry => entry.summary || entry.thinking || entry.reasoningObserved || entry.reasoningTruncated || entry.tools?.length)
  const hasProcess = entries.length > 0

  const hasRunningTool = running && entries.some(entry => entry.tools?.some(tool => tool.status === 'running'))
  const inProgress = waiting || hasRunningTool
  const open = toggle?.running === running ? toggle.open : inProgress

  useLayoutEffect(() => {
    if (!open) return
    const panel = panelRef.current
    return () => { clearCollapsedSelection(panel) }
  }, [open])

  if (!waiting && !hasProcess) return null

  const duration = sections?.at(-1)?.durationMs ?? work?.durationMs ?? message.details?.durationMs
  const label = waiting
    ? '正在思考'
    : hasRunningTool
      ? '正在调用工具'
      : typeof duration === 'number'
        ? `用时 ${elapsedLabel(duration)}`
        : '查看记录'

  const title = <span className={inProgress ? 'chat-thinking-label' : undefined} role={inProgress ? 'status' : undefined}>{label}</span>

  return <div className="text-sm text-muted-foreground">
    {hasProcess
      ? <>
        <Button type="button" variant="ghost" size="sm" className="-ml-2 text-muted-foreground hover:text-foreground"
          aria-controls={panelId} aria-expanded={open} onClick={() => {
            if (open) clearCollapsedSelection(panelRef.current)
            setToggle({ running, open: !open })
          }}>
          {title}
          {open ? <ChevronDown data-icon="inline-end" /> : <ChevronRight data-icon="inline-end" />}
        </Button>
        <div ref={panelRef} id={panelId} hidden={!open} className="mt-1 max-h-56 overflow-y-auto border-l border-border/80 pl-3">
          {open && <div className="flex flex-col gap-1.5">
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
                {entry.tools.map((tool, index) => <ToolStep key={index} tool={tool} />)}
              </ul>}
            </div>)}
          </div>}
        </div>
      </>
      : <div className="py-1">{title}</div>}
  </div>
}

export function ReplyBody({ message, markdown, reveal }: {
  message: Message; markdown: boolean; reveal?: readonly RevealBatch[]
}) {
  const work = message.details?.work
  const positions = work?.positions
  const running = message.status === 'running'

  function textBetween(start: number, end: number) {
    const text = message.text.slice(start, end)
    if (!text) return null
    if (!markdown) return <p className="whitespace-pre-wrap break-words">{text}</p>
    const batches = reveal?.filter(batch => batch.end > start && batch.start < end)
      .map(batch => ({ ...batch, start: Math.max(batch.start, start) - start, end: Math.min(batch.end, end) - start }))
    return <ChatMarkdown text={text} streaming={running && end === message.text.length} reveal={batches?.length ? batches : undefined} />
  }

  if (!work || !positions?.length) {
    const body = textBetween(0, message.text.length)
    return <div className="flex flex-col gap-2">
      <ReplyProgress message={message} />
      {body}
    </div>
  }

  const groups: { offset: number; sections: ReplyWork[] }[] = []
  positions.forEach((position, i) => {
    const previous = positions[i - 1]
    const last = groups.at(-1)
    const group = last?.offset === position.offset ? last : { offset: position.offset, sections: [] }
    if (group !== last) groups.push(group)
    group.sections.push({
      durationMs: position.durationMs,
      reasoningObserved: false,
      reasoningText: work.reasoningText?.slice(previous?.reasoningEnd ?? 0, position.reasoningEnd),
      reasoningSummary: work.reasoningSummary?.slice(previous?.summaryEnd ?? 0, position.summaryEnd),
      reasoningTruncated: i === positions.length - 1 && work.reasoningTruncated,
      tools: work.tools?.slice(previous?.toolEnd ?? 0, position.toolEnd) ?? [],
    })
  })

  return <div className="flex flex-col gap-2">
    {groups.map((group, i) => {
      const before = textBetween(groups[i - 1]?.offset ?? 0, group.offset)
      return <div key={group.offset} className="contents">
        {before}
        <ReplyProgress message={message} sections={group.sections} />
      </div>
    })}
    {textBetween(positions[positions.length - 1].offset, message.text.length)}
  </div>
}
