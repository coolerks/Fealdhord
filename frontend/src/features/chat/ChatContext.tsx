import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { ContextStatus, GenerateContextSummary, SaveContext } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/api'

export function ChatContext({ id, modelId, windowSize, disabled }: { id: string; modelId: string; windowSize: number; disabled: boolean }) {
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
          </dl>
          {id && <><label htmlFor="chat-context-summary" className="text-sm">保留给模型的摘要（可选）</label>
            <Textarea id="chat-context-summary" rows={4} value={summary ?? state.data.summary} onChange={event => setSummary(event.target.value)} />
            <div className="flex flex-wrap justify-end gap-2">
              <Button variant="outline" disabled={disabled || save.isPending || summarize.isPending || !state.data.history}
                title="将会话历史发给当前模型，可能产生费用" onClick={() => summarize.mutate()}>
                {summarize.isPending ? '正在整理…' : '用当前模型生成摘要'}
              </Button>
              {state.data.contextAfter > 0 && <Button variant="outline" disabled={disabled || save.isPending} onClick={() => save.mutate(true)}>恢复历史上下文</Button>}
              <Button disabled={disabled || save.isPending} onClick={() => save.mutate(false)}>从这里重置上下文</Button>
            </div></>}
          {summarize.isError && <p role="alert">无法生成摘要，可以手动编辑。</p>}
          {save.isError && <p role="alert">保存失败。请停止当前回复，并检查摘要长度。</p>}
        </>}
      </DialogContent>
    </Dialog>
  </>
}
