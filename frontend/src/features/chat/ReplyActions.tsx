import { useEffect, useState } from 'react'
import { Check, Copy, GitBranch, Info } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { native } from '@/lib/native'
import { notify } from '@/lib/notifications'
import { cn } from '@/lib/utils'
import type { Message } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'

const numbers = new Intl.NumberFormat('zh-CN')
const decimals = new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 })
const count = (value?: number | null) => typeof value === 'number' ? numbers.format(value) : '未记录'
export const formatDuration = (value: number) => `${decimals.format(value / 1000)} 秒`

export function ReplyActions({ message, branchDisabled, onBranch }: {
  message: Message
  branchDisabled: boolean
  onBranch: () => void
}) {
  const [copied, setCopied] = useState(false)
  useEffect(() => {
    if (!copied) return
    const timer = window.setTimeout(() => setCopied(false), 1600)
    return () => window.clearTimeout(timer)
  }, [copied])

  const details = message.details
  const legacySuffix = ' · 工具模式（关闭思考）'
  const legacyToolMode = message.modelName.endsWith(legacySuffix)
  const modelName = legacyToolMode ? message.modelName.slice(0, -legacySuffix.length) : message.modelName
  const firstToken = details?.firstTokenMs
  const speed = details?.outputTokensPerSecond
  const rows = [
    ['本次耗时', details ? formatDuration(details.durationMs) : '未记录'],
    ['首字耗时', typeof firstToken === 'number' ? formatDuration(firstToken) : '未记录'],
    ['平均输出速度', typeof speed === 'number' ? `${decimals.format(speed)} token/s` : '未记录'],
    ['轮次', count(details?.rounds)],
    ['模型', modelName || '未记录'],
    ['思考等级', details?.reasoningEffort || (legacyToolMode ? '关闭' : '未记录')],
    ['输入 Token', count(details?.inputTokens)],
    ['缓存输入 Token', count(details?.cachedInputTokens)],
    ['输出 Token', count(details?.outputTokens)],
  ]

  async function copy() {
    try {
      await native.copy(message.text)
      setCopied(true)
      void notify('已复制回复')
    } catch {
      void notify('无法复制回复', undefined, true)
    }
  }

  return <div className="mt-2 flex items-center gap-1 text-muted-foreground" role="group" aria-label="回复操作">
    <Tooltip>
      <TooltipTrigger asChild>
        <Button type="button" variant="ghost" size="icon-xs" aria-label="复制回复"
          disabled={!message.text} onClick={() => void copy()}>
          {copied ? <Check /> : <Copy />}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{copied ? '已复制' : '复制'}</TooltipContent>
    </Tooltip>
    <Tooltip>
      <TooltipTrigger asChild>
        <Button type="button" variant="ghost" size="icon-xs" aria-label="分支到新聊天"
          disabled={branchDisabled} onClick={onBranch}>
          <GitBranch />
        </Button>
      </TooltipTrigger>
      <TooltipContent>从此处分支</TooltipContent>
    </Tooltip>
    <Dialog>
      <Tooltip>
        <TooltipTrigger asChild>
          <DialogTrigger asChild>
            <Button type="button" variant="ghost" size="icon-xs" aria-label="查看回复详情">
              <Info />
            </Button>
          </DialogTrigger>
        </TooltipTrigger>
        <TooltipContent>详情</TooltipContent>
      </Tooltip>
      <DialogContent aria-describedby={undefined} className="max-w-sm">
        <DialogHeader><DialogTitle>回复详情</DialogTitle></DialogHeader>
        <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2.5 text-sm">
          {rows.map(([label, value]) => <div key={label} className="contents">
            <dt className="text-muted-foreground">{label}</dt>
            <dd className={cn('wrap-anywhere text-right tabular-nums', value === '未记录' && 'text-muted-foreground')}>{value}</dd>
          </div>)}
        </dl>
      </DialogContent>
    </Dialog>
  </div>
}
