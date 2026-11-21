import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ListChannels, PreviewTask } from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/api'
import type { Task } from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/models'
import { navigationAPI, navigationQueryKey } from '@/lib/navigation-api'
import { DecideAction, PendingActions } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api'

const key = ['ai-platform', 'pending']

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
  const cache = useQueryClient()
  const pending = useQuery({ queryKey: key, queryFn: () => PendingActions(), refetchInterval: 1000, retry: false })
  const decision = useMutation({
    mutationFn: ({ id, allow }: { id: string; allow: boolean }) => Promise.resolve(DecideAction(id, allow)),
    onSettled: () => cache.invalidateQueries({ queryKey: key }),
    onSuccess: (accepted, { allow }) => {
      if (accepted && allow) window.setTimeout(() => { void cache.invalidateQueries() }, 1000)
    },
  })
  const action = pending.data?.[0]
  return <Dialog open={Boolean(action)}>
    {action && <DialogContent showCloseButton={false} onEscapeKeyDown={event => event.preventDefault()} onPointerDownOutside={event => event.preventDefault()}>
      <DialogHeader>
        <DialogTitle>允许 AI 调用工具？</DialogTitle>
        <DialogDescription>{action.principal} · {action.tool}</DialogDescription>
      </DialogHeader>
      {['schedule.create', 'schedule.update'].includes(action.tool) ? <ScheduleApproval args={action.arguments} />
        : <pre className="max-h-52 overflow-auto whitespace-pre-wrap break-all rounded-md bg-muted p-3 text-xs">{action.arguments}</pre>}
      {decision.isError && <p role="alert">提交失败，请重试。</p>}
      <DialogFooter>
        <Button variant="outline" disabled={decision.isPending} onClick={() => decision.mutate({ id: action.id, allow: false })}>拒绝</Button>
        <Button disabled={decision.isPending} onClick={() => decision.mutate({ id: action.id, allow: true })}>允许本次调用</Button>
      </DialogFooter>
    </DialogContent>}
  </Dialog>
}
