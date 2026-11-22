import { useId, useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { Message } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'
import { ChatMarkdown } from './ChatMarkdown'
import { formatDuration } from './ReplyActions'

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

export function ReplyProgress({ message }: { message: Message }) {
  const panelId = useId()
  const [toggle, setToggle] = useState<{ running: boolean; open: boolean }>()
  const running = message.status === 'running'
  const waiting = running && !message.text.trim()
  const open = toggle?.running === running ? toggle.open : running
  const work = message.details?.work
  const tools = work?.tools ?? message.tools
  const summary = work?.reasoningSummary?.trim()
  const hasProcess = Boolean(work?.reasoningObserved || tools?.length)
  if (!waiting && !hasProcess) return null

  const duration = work?.durationMs ?? message.details?.durationMs
  const label = waiting ? '正在思考' : '工作过程'
  const title = <span className={waiting ? 'chat-thinking-label' : undefined} role={waiting ? 'status' : undefined}>{label}</span>

  return <div className="mb-3 text-sm text-muted-foreground">
    {hasProcess
      ? <>
        <Button type="button" variant="ghost" size="sm" className="-ml-2"
          aria-controls={panelId} aria-expanded={open} onClick={() => setToggle({ running, open: !open })}>
          {title}
          {!running && typeof duration === 'number' && <span className="tabular-nums">{formatDuration(duration)}</span>}
          {open ? <ChevronDown data-icon="inline-end" /> : <ChevronRight data-icon="inline-end" />}
        </Button>
        <div id={panelId} hidden={!open} className="my-1 max-h-56 overflow-y-auto border-l pl-3">
          {open && <div className="flex flex-col gap-2">
            {summary
              ? <ChatMarkdown text={summary} streaming={running} />
              : work?.reasoningObserved && <p>{waiting ? '模型正在推理' : '模型已执行推理'}</p>}
            {!!tools?.length && <ul className="flex flex-col gap-1" aria-label="工具活动">
              {tools.map((tool, index) => <li key={index} className="flex flex-wrap items-baseline gap-x-2">
                <span>{Object.hasOwn(toolLabels, tool.name) ? toolLabels[tool.name] : tool.name}</span>
                <span>{statusLabels[tool.status] ?? '状态未知'}</span>
              </li>)}
            </ul>}
          </div>}
        </div>
      </>
      : title}
  </div>
}
