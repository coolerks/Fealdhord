import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ListChannels, PreviewTask } from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/api'
import type { Task } from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/models'
import { navigationAPI, navigationQueryKey } from '@/lib/navigation-api'
import { DecideAction, PendingActions } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api'
import type { PendingAction } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/models'
import { useLayoutEffect } from 'react'
import { create } from 'zustand'
import { getToolMeta } from './tool-meta'

const key = ['ai-platform', 'pending']
const useApprovalSurface = create<{ id: string }>(() => ({ id: '' }))

export function useInlineApproval(id: string, enabled: boolean) {
  const pending = useQuery({ queryKey: key, queryFn: () => PendingActions(), enabled: false })
  useLayoutEffect(() => {
    if (!enabled || !id) return
    useApprovalSurface.setState({ id })
    return () => {
      if (useApprovalSurface.getState().id === id) useApprovalSurface.setState({ id: '' })
    }
  }, [id, enabled])
  return enabled && id ? pending.data?.find(action => action.conversationId === id) : undefined
}

function ScheduleApproval({ args }: { args: string }) {
  let task: Task | undefined
  try {
    const candidate: Task = JSON.parse(args)
    if (typeof candidate?.name === 'string' && typeof candidate.calendar?.timezone === 'string' &&
      typeof candidate.action === 'string' && (candidate.text == null || typeof candidate.text === 'string') &&
      [candidate.channels, candidate.categories].every(items => items == null ||
        (Array.isArray(items) && items.every(item => typeof item === 'string'))) &&
      [candidate.calendar.local_time, candidate.calendar.once_at].every(value => value == null || typeof value === 'string') &&
      (candidate.calendar.weekdays == null || (Array.isArray(candidate.calendar.weekdays) &&
        candidate.calendar.weekdays.every(day => Number.isInteger(day) && day >= 0 && day <= 6)))) {
      task = candidate
    }
  } catch { /* 原始参数由服务端校验。 */ }
  const { data: channels } = useQuery({ queryKey: ['notifications', 'channels'], queryFn: () => ListChannels(), enabled: Boolean(task), retry: false })
  const { data: navigation } = useQuery({ queryKey: navigationQueryKey, queryFn: () => navigationAPI.list(), enabled: Boolean(task?.categories?.length), retry: false })
  const { data: next } = useQuery({ queryKey: ['notifications', 'preview-task', task], queryFn: () => PreviewTask(task!), enabled: Boolean(task), retry: false })
  if (!task) return <pre className="whitespace-pre-wrap break-all text-xs">{args}</pre>
  const targets = task.channels?.map(id => channels?.find(channel => channel.id === id)?.name ?? id)
  const categories = task.categories?.map(id => navigation?.categories?.find(category => category.id === id)?.name ?? id)
  return <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 break-words text-sm">
    <dt className="text-muted-foreground">任务</dt><dd>{task.name}</dd>
    <dt className="text-muted-foreground">时间</dt><dd>{task.calendar.once_at || task.calendar.local_time} · {task.calendar.timezone}</dd>
    {task.calendar.kind === 'weekly' && <><dt className="text-muted-foreground">星期</dt><dd>{task.calendar.weekdays?.map(day => ['日', '一', '二', '三', '四', '五', '六'][day]).join('、')}</dd></>}
    {next && <><dt className="text-muted-foreground">下次执行</dt><dd>{new Date(next).toLocaleString()}</dd></>}
    <dt className="text-muted-foreground">接收渠道</dt><dd>{targets?.join('、') || '应用内'}</dd>
    {task.action !== 'reminder' && <><dt className="text-muted-foreground">分类</dt><dd>{categories?.join('、') || '全部分类'}</dd></>}
    {task.text && <><dt className="text-muted-foreground">内容</dt><dd className="max-h-36 overflow-auto whitespace-pre-wrap">{task.text}</dd></>}
  </dl>
}

export function AIApprovals() {
  const inline = useApprovalSurface(state => state.id)
  const pending = useQuery({ queryKey: key, queryFn: () => PendingActions(), refetchInterval: 1000, retry: false })
  const action = pending.data?.find(item => !inline || item.conversationId !== inline)
  return <Dialog open={Boolean(action)}>
    {action && <DialogContent showCloseButton={false} onEscapeKeyDown={event => event.preventDefault()} onPointerDownOutside={event => event.preventDefault()}>
      <ToolApproval key={action.id} action={action} />
    </DialogContent>}
  </Dialog>
}

export function ToolApproval({ action, inline = false, onStop, stopping }: {
  action: PendingAction; inline?: boolean; onStop?: () => void; stopping?: boolean
}) {
  const cache = useQueryClient()
  const decision = useMutation({
    mutationFn: ({ id, allow }: { id: string; allow: boolean }) => Promise.resolve(DecideAction(id, allow)),
    onSettled: () => cache.invalidateQueries({ queryKey: key }),
    onSuccess: (accepted, { allow, id }) => {
      cache.setQueryData<PendingAction[]>(key, current => current?.filter(item => item.id !== id))
      if (accepted && allow) window.setTimeout(() => { void cache.invalidateQueries() }, 1000)
    },
  })
  const Footer = inline ? 'div' : DialogFooter
  const meta = getToolMeta(action.tool)
  const ToolIcon = meta.icon
  const toolSummary = meta.label === action.tool ? action.tool : `${meta.label}（${action.tool}）`
  return <>
      {inline ? <div className="flex items-start gap-2">
        <ToolIcon className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <div className="min-w-0">
          <h3 className="text-sm font-medium">允许 AI 调用工具？</h3>
          <p className="truncate text-xs text-muted-foreground" title={toolSummary}>{toolSummary}</p>
        </div>
      </div> : <DialogHeader>
        <DialogTitle className="flex items-center gap-2">
          <ToolIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          允许 AI 调用工具？
        </DialogTitle>
        <DialogDescription>{action.principal} · {toolSummary}</DialogDescription>
      </DialogHeader>}
      {['schedule.create', 'schedule.update'].includes(action.tool) ? <ScheduleApproval args={action.arguments} />
        : <pre className="max-h-52 overflow-auto whitespace-pre-wrap break-all rounded-md bg-muted p-3 text-xs">{action.arguments}</pre>}
      {decision.isError && <p role="alert" className="text-sm text-destructive">提交失败，请重试。</p>}
      <Footer className={inline ? 'flex flex-wrap justify-end gap-2' : undefined}>
        {onStop && <Button type="button" variant="ghost" className="sm:mr-auto" disabled={stopping || decision.isPending} onClick={onStop}>停止回复</Button>}
        <Button type="button" variant="outline" disabled={stopping || decision.isPending} onClick={() => decision.mutate({ id: action.id, allow: false })}>拒绝</Button>
        <Button type="button" disabled={stopping || decision.isPending} onClick={() => decision.mutate({ id: action.id, allow: true })}>允许本次调用</Button>
      </Footer>
  </>
}
