import { useId, useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { Message, ReplyToolStep, ReplyWork } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'
import { ChatMarkdown } from './ChatMarkdown'
import type { RevealBatch } from './stream-reveal'

const toolLabels: Record<string, string> = {
  article_list: '列出文章',
  article_search: '搜索文章',
  article_get: '读取文章正文',
  article_read: '读取文章正文',
  article_get_metadata: '读取文章摘要',
  article_extract: '提取文章正文',
  read_article: '读取授权文章',
  library_search: '搜索本地文章',
  find_app_tools: '查找可用工具',
  run_app_tool: '调用工具',
}

const statusLabels: Record<string, string> = {
  running: '进行中',
  completed: '已完成',
  denied: '未获授权',
  failed: '失败',
  interrupted: '已中断',
  cancelled: '已取消',
  unknown: '结果未知',
}

function elapsedLabel(ms: number) {
  const seconds = Math.max(0, Math.round(ms / 1000))
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor(seconds / 60) % 60
  const rest = seconds % 60
  return [hours ? `${hours} 小时` : '', minutes ? `${minutes} 分钟` : '', rest || !seconds ? `${rest} 秒` : ''].filter(Boolean).join(' ')
}

function ToolPayload({ label, text, truncated }: { label: string; text: string; truncated?: boolean }) {
  return <div>
    <div className="mb-1">{label}{truncated && '（已截断）'}</div>
    <pre className="max-h-40 overflow-auto rounded-md bg-muted p-2 text-xs whitespace-pre-wrap break-all text-foreground">{text || '（空）'}</pre>
  </div>
}

function ToolStep({ tool }: { tool: ReplyToolStep }) {
  const id = useId()
  const [open, setOpen] = useState(false)
  const label = Object.hasOwn(toolLabels, tool.name) ? toolLabels[tool.name] : tool.name
  const status = statusLabels[tool.status] ?? '状态未知'
  const hasData = tool.arguments != null || tool.result != null
  if (!hasData) return <li className="flex flex-wrap items-baseline gap-x-2"><span>{label}</span><span>{status}</span></li>

  return <li>
    <Button type="button" variant="ghost" size="sm" className="max-w-full" aria-label={`${label} ${status}`} aria-expanded={open} aria-controls={id}
      onClick={() => setOpen(!open)}>
      <span className="truncate">{label}</span><span>{status}</span>
      {open ? <ChevronDown data-icon="inline-end" /> : <ChevronRight data-icon="inline-end" />}
    </Button>
    <div id={id} hidden={!open}>
      {open && <div className="flex flex-col gap-2 py-1">
        {tool.arguments != null && <ToolPayload label="调用参数" text={tool.arguments} truncated={tool.argumentsTruncated} />}
        {tool.result != null && <ToolPayload label="返回值" text={tool.result} truncated={tool.resultTruncated} />}
      </div>}
    </div>
  </li>
}

function ReplyProgress({ message, sections }: { message: Message; sections?: ReplyWork[] }) {
  const panelId = useId()
  const [toggle, setToggle] = useState<{ running: boolean; open: boolean }>()
  const running = message.status === 'running'
  const waiting = running && !message.text.trim()
  const open = toggle?.running === running ? toggle.open : running
  const work = message.details?.work
  const entries = (sections ?? [{ ...work, tools: work?.tools ?? message.tools }]).map(entry => ({
    ...entry, summary: entry.reasoningSummary?.trim(), thinking: entry.reasoningText?.trim(),
  })).filter(entry => entry.summary || entry.thinking || entry.reasoningObserved || entry.reasoningTruncated || entry.tools?.length)
  const hasProcess = entries.length > 0
  if (!waiting && !hasProcess) return null

  const duration = sections?.at(-1)?.durationMs ?? work?.durationMs ?? message.details?.durationMs
  const label = waiting ? '正在思考' : typeof duration === 'number' ? `用时 ${elapsedLabel(duration)}` : '查看记录'
  const title = <span className={waiting ? 'chat-thinking-label' : undefined} role={waiting ? 'status' : undefined}>{label}</span>

  return <div className="mb-3 text-sm text-muted-foreground">
    {hasProcess
      ? <>
        <Button type="button" variant="ghost" size="sm" className="-ml-2"
          aria-controls={panelId} aria-expanded={open} onClick={() => setToggle({ running, open: !open })}>
          {title}
          {open ? <ChevronDown data-icon="inline-end" /> : <ChevronRight data-icon="inline-end" />}
        </Button>
        <div id={panelId} hidden={!open} className="my-1 max-h-56 overflow-y-auto border-l pl-3">
          {open && <div className="flex flex-col gap-2">
            {entries.map((entry, index) => <div key={index} className="flex flex-col gap-2">
              {entry.summary && <div>
                {entry.thinking && <p className="font-medium">思考摘要</p>}
                <ChatMarkdown text={entry.summary} streaming={running} />
              </div>}
              {entry.thinking && <div>
                {entry.summary && <p className="font-medium">推理过程</p>}
                <p className="whitespace-pre-wrap">{entry.thinking}</p>
              </div>}
              {entry.reasoningTruncated && <p>思考内容已截断</p>}
              {!entry.summary && !entry.thinking && entry.reasoningObserved && <p>{waiting ? '模型正在推理' : '模型已执行推理'}</p>}
              {!!entry.tools?.length && <ul className="flex flex-col gap-1" aria-label="工具活动">
                {entry.tools.map((tool, index) => <ToolStep key={index} tool={tool} />)}
              </ul>}
            </div>)}
          </div>}
        </div>
      </>
      : title}
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

  if (!work || !positions?.length) return <>
    <ReplyProgress message={message} />
    {textBetween(0, message.text.length)}
  </>

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
    {groups.map((group, i) =>
      <div key={group.offset}>
        {textBetween(groups[i - 1]?.offset ?? 0, group.offset)}
        <ReplyProgress message={message} sections={group.sections} />
      </div>
    )}
    {textBetween(positions[positions.length - 1].offset, message.text.length)}
  </div>
}
