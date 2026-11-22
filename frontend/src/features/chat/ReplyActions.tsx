import { Copy, GitBranch, Info } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { native } from '@/lib/native'
import { notify } from '@/lib/notifications'
import type { Message } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'

const numbers = new Intl.NumberFormat('zh-CN')
const seconds = new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 })
const count = (value?: number | null) => typeof value === 'number' ? numbers.format(value) : '未记录'

export function ReplyActions({ message, branchDisabled, onBranch }: {
  message: Message
  branchDisabled: boolean
  onBranch: () => void
}) {
  const details = message.details
  const legacySuffix = ' · 工具模式（关闭思考）'
  const legacyToolMode = message.modelName.endsWith(legacySuffix)
  const modelName = legacyToolMode ? message.modelName.slice(0, -legacySuffix.length) : message.modelName
  const rows = [
    ['本次耗时', details ? `${seconds.format(details.durationMs / 1000)} 秒` : '未记录'],
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
      void notify('已复制回复')
    } catch {
      void notify('无法复制回复', undefined, true)
    }
  }

  return <div className="mt-2 flex items-center gap-1 text-muted-foreground" role="group" aria-label="回复操作">
    <Button type="button" variant="ghost" size="icon-xs" aria-label="复制回复" title="复制"
      disabled={!message.text} onClick={() => void copy()}><Copy /></Button>
    <Button type="button" variant="ghost" size="icon-xs" aria-label="分支到新聊天" title="分支"
      disabled={branchDisabled} onClick={onBranch}><GitBranch /></Button>
    <Dialog>
      <DialogTrigger asChild>
        <Button type="button" variant="ghost" size="icon-xs" aria-label="查看回复详情" title="详情"><Info /></Button>
      </DialogTrigger>
      <DialogContent aria-describedby={undefined} className="max-w-sm">
        <DialogHeader><DialogTitle>回复详情</DialogTitle></DialogHeader>
        <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-3 text-sm">
          {rows.map(([label, value]) => <div key={label} className="contents">
            <dt className="text-muted-foreground">{label}</dt>
            <dd className="wrap-anywhere text-right tabular-nums">{value}</dd>
          </div>)}
        </dl>
      </DialogContent>
    </Dialog>
  </div>
}
