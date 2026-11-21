import { Switch } from '@/components/ui/switch'
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { ContextStatus, GenerateContextSummary, PinContextArticle, SaveContext, SetAutomaticCompaction } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/api'

export function ChatContext({ id, modelId, windowSize, disabled, attachments }: {
  id: string; modelId: string; windowSize: number; disabled: boolean; attachments: { id: string; title: string }[]
}) {
  const cache = useQueryClient()
  const [open, setOpen] = useState(false)
  const [summary, setSummary] = useState<string>()
  const key = ['chat-context', id, modelId]
  const state = useQuery({ queryKey: key, queryFn: () => ContextStatus(id, modelId), enabled: open, retry: false, staleTime: 0 })
  const save = useMutation({
    mutationFn: (restore: boolean) => Promise.resolve(SaveContext(id, summary ?? state.data?.summary ?? '', restore)),
    onSuccess: async () => { await cache.invalidateQueries({ queryKey: key }); setSummary(undefined) },
  })
  const summarize = useMutation({
    mutationFn: () => Promise.resolve(GenerateContextSummary(id, modelId)),
    onSuccess: setSummary,
  })
  const pin = useMutation({
    mutationFn: ({ article, pinned }: { article: string; pinned: boolean }) => Promise.resolve(PinContextArticle(id, article, pinned)),
    onSuccess: () => cache.invalidateQueries({ queryKey: key }),
  })
  const automatic = useMutation({
    mutationFn: (enabled: boolean) => Promise.resolve(SetAutomaticCompaction(id, enabled)),
    onSuccess: () => cache.invalidateQueries({ queryKey: key }),
  })
  const pins = state.data?.pins ?? []
  return <>
    <Button type="button" size="sm" variant="ghost" className="text-muted-foreground" onClick={() => { setSummary(undefined); setOpen(true) }}>
      {windowSize.toLocaleString()} 上下文
    </Button>
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent>
        <DialogHeader><DialogTitle>会话上下文</DialogTitle>
          <DialogDescription>可以保留可见聊天记录，并从当前消息之后重新积累模型上下文。</DialogDescription></DialogHeader>
        {state.isPending && <p role="status">读取上下文…</p>}
        {state.isError && <p role="alert">无法读取上下文。</p>}
        {state.data && <>
          <dl className="grid grid-cols-[1fr_auto] gap-2 text-sm">
            <dt>模型窗口{state.data.budget.estimated ? '（估算）' : ''}</dt><dd>{state.data.budget.context.toLocaleString()}</dd>
            <dt>输入预算</dt><dd>{state.data.budget.inputLimit.toLocaleString()}</dd>
            <dt>输出预留</dt><dd>{state.data.budget.outputReserve.toLocaleString()}</dd>
            <dt>历史及摘要（估算）</dt><dd>{state.data.used.toLocaleString()}</dd>
            <dt>可用历史轮次</dt><dd>{state.data.history}</dd>
            {state.data.lastInput > 0 && <><dt title={state.data.lastModel}>上次服务报告的输入令牌</dt><dd>{state.data.lastInput.toLocaleString()}</dd></>}
          </dl>
          {id && <>
            <div className="flex items-center gap-2 text-sm">
              <Switch id="chat-context-auto" checked={state.data.autoCompact} disabled={disabled || automatic.isPending} onCheckedChange={value => automatic.mutate(value)} />
              <label htmlFor="chat-context-auto">超出上下文时用当前模型整理旧消息（可能计费）</label>
            </div>
            {pins.length > 0 && <div className="space-y-1">
              <h3 className="text-sm font-medium">固定的文章摘要</h3>
              {pins.map(article => <div key={article.id} className="flex items-center gap-2 text-sm">
                <span className="mr-auto truncate" title={article.title}>{article.title}</span>
                <Button size="sm" variant="ghost" aria-label={`移除固定文章 ${article.title}`} disabled={disabled || pin.isPending}
                  onClick={() => pin.mutate({ article: article.id, pinned: false })}>移除</Button>
              </div>)}
            </div>}
            {attachments.filter(article => !pins.some(item => item.id === article.id)).map(article => <div key={article.id} className="flex items-center gap-2 text-sm">
              <span className="mr-auto truncate" title={article.title}>{article.title}</span>
              <Button size="sm" variant="outline" aria-label={`固定文章 ${article.title}`} disabled={disabled || pin.isPending || pins.length >= 3}
                onClick={() => pin.mutate({ article: article.id, pinned: true })}>固定摘要</Button>
            </div>)}
            <label htmlFor="chat-context-summary" className="text-sm">保留给模型的摘要（可选）</label>
            <Textarea id="chat-context-summary" rows={4} value={summary ?? state.data.summary} onChange={event => setSummary(event.target.value)} />
            <div className="flex flex-wrap justify-end gap-2">
              <Button variant="outline" disabled={disabled || save.isPending || summarize.isPending || !state.data.history}
                title="将会话历史发给当前模型，可能产生费用" onClick={() => summarize.mutate()}>
                {summarize.isPending ? '正在整理…' : '用当前模型生成摘要'}
              </Button>
              {state.data.contextAfter > 0 && <Button variant="outline" disabled={disabled || save.isPending} onClick={() => save.mutate(true)}>恢复历史上下文</Button>}
              <Button disabled={disabled || save.isPending} onClick={() => save.mutate(false)}>从这里重置上下文</Button>
            </div></>}
          {automatic.isError && <p role="alert">无法保存自动整理设置。</p>}
          {pin.isError && <p role="alert">无法更新固定文章。请确认文章仍可用。</p>}
          {summarize.isError && <p role="alert">无法生成摘要，可以手动编辑。</p>}
          {save.isError && <p role="alert">保存失败。请停止当前回复，并检查摘要长度。</p>}
        </>}
      </DialogContent>
    </Dialog>
  </>
}
