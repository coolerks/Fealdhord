import { useLayoutEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronLeft, Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Textarea } from '@/components/ui/textarea'
import { SemanticButton } from '@/components/semantic-button'
import { defaultZone, FlagField, Selection, TextField, time, toggle, zones } from '@/features/notifications/notification-fields'
import { navigationAPI, navigationQueryKey } from '@/lib/navigation-api'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/api'
import type { Task } from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/models'

const tasksKey = ['notifications', 'tasks'] as const
const actions = [{ value: 'reminder', label: '提醒' }, { value: 'articles', label: '文章列表' }, { value: 'summary', label: 'AI 文章摘要' }]
const frequencies = [{ value: 'once', label: '一次' }, { value: 'daily', label: '每天' }, { value: 'weekly', label: '指定星期' }]
const weekdays = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'].map((label, index) => ({ value: String(index), label }))
const runStatus: Record<string, string> = {
  complete: '已完成', running: '运行中', interrupted: '已中断', failed: '失败', cancelled: '已取消',
}
const blankTask: Task = {
  id: '', name: '', action: 'reminder', lookback_days: 1, text: '', categories: [], channels: [], enabled: true, revision: 0, next_at: 0,
  calendar: { kind: 'daily', timezone: defaultZone, local_time: '20:00' },
}

export function ScheduledTasks({ collapsed, onBack }: { collapsed: boolean; onBack(): void }) {
  const client = useQueryClient()
  const title = useRef<HTMLHeadingElement>(null)
  const dialogTrigger = useRef<HTMLButtonElement>(null)
  const [editor, setEditor] = useState<Task>()
  const [deleting, setDeleting] = useState<Task>()
  const [openedTask, setOpenedTask] = useState('')
  const [actionTarget, setActionTarget] = useState('')
  const tasks = useQuery({ queryKey: tasksKey, queryFn: () => api.ListTasks(), retry: false })
  const mutation = useMutation({
    mutationFn: (work: () => Promise<unknown>) => work(),
    onSettled: () => client.invalidateQueries({ queryKey: tasksKey }),
  })
  useLayoutEffect(() => { title.current?.focus({ preventScroll: true }) }, [])
  const busy = mutation.isPending
  const error = mutation.error?.message
  function edit(task: Task, trigger: HTMLButtonElement) {
    dialogTrigger.current = trigger
    mutation.reset()
    setEditor(task)
  }
  function restoreDialogFocus(event: Event) {
    event.preventDefault()
    const trigger = dialogTrigger.current
    const target = trigger?.isConnected && !trigger.disabled ? trigger : title.current
    target?.focus({ preventScroll: true })
  }

  return <section className="row-span-full flex min-h-0 min-w-0 flex-col" aria-labelledby="scheduled-tasks-title">
    <header className="workspace-toolbar chat-header" data-sidebar-collapsed={collapsed}>
      <SemanticButton icon={ChevronLeft} meaning="chevron-left" label="返回对话" onClick={onBack} />
      <h1 id="scheduled-tasks-title" ref={title} tabIndex={-1} className="outline-none">定时任务</h1>
      <Button size="sm" variant="outline" disabled={busy} onClick={event => edit(blankTask, event.currentTarget)}><Plus />添加任务</Button>
    </header>
    <div className="min-h-0 overflow-auto px-[var(--chat-gutter)] py-6">
      <div className="mx-auto w-full max-w-[var(--chat-column)]">
        {tasks.isPending && <p role="status" className="text-sm text-muted-foreground">正在读取定时任务…</p>}
        {tasks.isError && <p role="alert" className="text-sm text-destructive">暂时无法读取定时任务</p>}
        {tasks.data?.length === 0 && <p className="text-sm text-muted-foreground">还没有定时任务</p>}
        <ul className="divide-y divide-border">
          {tasks.data?.map(task => {
            const finished = task.enabled && task.next_at === 0
            return <li key={task.id} className="space-y-3 py-4 first:pt-0">
              <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
                <div className="min-w-0">
                  <h2 className="break-words text-sm font-medium">{task.name}</h2>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {actions.find(action => action.value === task.action)?.label} · {finished ? '已运行' : task.enabled ? `下次执行 ${time(task.next_at)}` : '已暂停'}
                  </p>
                </div>
                <div className="flex flex-wrap gap-1">
                  {!finished && <Button variant="ghost" size="sm" disabled={busy} onClick={() => {
                    setActionTarget(task.id)
                    mutation.mutate(() => api.SaveTask({ ...task, enabled: !task.enabled }))
                  }}>{task.enabled ? '暂停' : '恢复'}</Button>}
                  <Button variant="ghost" size="sm" disabled={busy} onClick={event => edit(task, event.currentTarget)}>编辑</Button>
                  <Button variant="ghost" size="sm" aria-expanded={openedTask === task.id}
                    onClick={() => setOpenedTask(openedTask === task.id ? '' : task.id)}>运行记录</Button>
                  <Button variant="ghost" size="sm" disabled={busy} onClick={event => {
                    dialogTrigger.current = event.currentTarget
                    mutation.reset()
                    setDeleting(task)
                  }}>删除</Button>
                </div>
              </div>
              {openedTask === task.id && <TaskRuns id={task.id} />}
              {error && !editor && !deleting && actionTarget === task.id && <p role="alert" className="text-sm text-destructive">{error}</p>}
            </li>
          })}
        </ul>
      </div>
    </div>
    <Dialog open={Boolean(editor)} onOpenChange={open => { if (!open && !busy) { setEditor(undefined); mutation.reset() } }}>
      {editor && <DialogContent aria-describedby={undefined} onCloseAutoFocus={restoreDialogFocus} className="max-h-[85dvh] overflow-y-auto sm:max-w-lg">
        <DialogHeader><DialogTitle>{editor.id ? '编辑任务' : '添加任务'}</DialogTitle></DialogHeader>
        <TaskEditor task={editor} busy={busy} error={error} onChange={setEditor}
          onCancel={() => { setEditor(undefined); mutation.reset() }}
          onSave={() => mutation.mutate(async () => { await api.SaveTask(editor); setEditor(undefined) })} />
      </DialogContent>}
    </Dialog>
    <Dialog open={Boolean(deleting)} onOpenChange={open => { if (!open && !busy) { setDeleting(undefined); mutation.reset() } }}>
      {deleting && <DialogContent onCloseAutoFocus={restoreDialogFocus}>
        <DialogHeader>
          <DialogTitle>删除 {deleting.name}？</DialogTitle>
          <DialogDescription>之后将停止运行此任务，已发送的通知会保留。</DialogDescription>
        </DialogHeader>
        {error && <p role="alert" className="text-destructive">{error}</p>}
        <DialogFooter>
          <Button variant="outline" disabled={busy} onClick={() => { setDeleting(undefined); mutation.reset() }}>取消</Button>
          <Button variant="destructive" disabled={busy} onClick={() => mutation.mutate(async () => {
            await api.DeleteTask(deleting.id, deleting.revision)
            if (openedTask === deleting.id) setOpenedTask('')
            setDeleting(undefined)
          })}>删除任务</Button>
        </DialogFooter>
      </DialogContent>}
    </Dialog>
  </section>
}

function TaskRuns({ id }: { id: string }) {
  const runs = useQuery({ queryKey: [...tasksKey, 'runs', id], queryFn: () => api.ListRuns(id), retry: false })
  return <div className="space-y-3 rounded-lg border p-3 text-sm">
    {runs.isPending && <p role="status" className="text-muted-foreground">正在读取运行记录…</p>}
    {runs.isError && <p role="alert" className="text-destructive">暂时无法读取运行记录</p>}
    {runs.data?.length === 0 && <p className="text-muted-foreground">还没有运行记录</p>}
    {runs.data?.map(run => <div key={run.occurrence}>
      <p>{time(run.occurrence)} · {runStatus[run.state] ?? run.state}</p>
      {run.result && <p className="whitespace-pre-wrap break-words text-muted-foreground">{run.result}</p>}
    </div>)}
  </div>
}

function TaskEditor({ task, busy, error, onChange, onCancel, onSave }: {
  task: Task; busy: boolean; error?: string; onChange(task: Task): void; onCancel(): void; onSave(): void
}) {
  const channels = useQuery({ queryKey: ['notifications', 'channels'], queryFn: () => api.ListChannels(), retry: false })
  const navigation = useQuery({ queryKey: navigationQueryKey, queryFn: () => navigationAPI.list(), enabled: task.action !== 'reminder' })
  const channelOptions = (channels.data ?? []).filter(value => (value.enabled && value.verified) || task.channels?.includes(value.id))
    .map(value => ({ label: value.name, value: value.id }))
  const categoryOptions = (navigation.data?.categories ?? []).map(value => ({ label: value.name, value: value.id }))
  const setCalendar = (fields: Partial<Task['calendar']>) => onChange({ ...task, calendar: { ...task.calendar, ...fields } })

  return <form onSubmit={event => { event.preventDefault(); if (!busy) onSave() }}>
    <FieldGroup>
      <TextField id="notification-task-name" label="任务名称" value={task.name} onChange={value => onChange({ ...task, name: value })} />
      <Selection id="notification-task-action" label="任务类型" value={task.action} options={actions} onChange={value => onChange({ ...task, action: value })} />
      <div className="grid gap-4 sm:grid-cols-2">
        <Selection id="notification-task-frequency" label="重复方式" value={task.calendar.kind} options={frequencies} onChange={value => setCalendar({ kind: value })} />
        <Selection id="notification-task-timezone" label="时区" value={task.calendar.timezone} options={zones} onChange={value => setCalendar({ timezone: value })} />
      </div>
      {task.calendar.kind === 'once'
        ? <TextField id="notification-task-once" label="执行时间" type="datetime-local" value={task.calendar.once_at?.slice(0, 16) ?? ''} onChange={value => setCalendar({ once_at: value })} />
        : <TextField id="notification-task-at" label="执行时间" type="time" value={task.calendar.local_time ?? ''} onChange={value => setCalendar({ local_time: value })} />}
      {task.calendar.kind === 'weekly' && <Selection id="notification-task-days" label="星期" value={(task.calendar.weekdays ?? []).map(String)} options={weekdays} onChange={value => setCalendar({ weekdays: toggle(task.calendar.weekdays, Number(value)) })} />}
      {task.action !== 'reminder' && <>
        <Selection id="notification-task-categories" label="分类" value={task.categories ?? []} options={categoryOptions} placeholder="全部分类" onChange={value => onChange({ ...task, categories: toggle(task.categories, value) })} />
        <TextField id="notification-task-lookback" label="回看天数（1 至 30 天）" type="number" value={String(task.lookback_days || 1)} onChange={value => onChange({ ...task, lookback_days: Number(value) })} />
      </>}
      <Field>
        <FieldLabel htmlFor="notification-task-text">{task.action === 'summary' ? '摘要要求' : '任务正文'}</FieldLabel>
        <Textarea id="notification-task-text" value={task.text} onChange={event => onChange({ ...task, text: event.target.value })} />
      </Field>
      <Selection id="notification-task-channels" label="接收渠道" value={task.channels ?? []} options={channelOptions} placeholder="仅应用内" onChange={value => onChange({ ...task, channels: toggle(task.channels, value) })} />
      <FlagField id="notification-task-enabled" label="启用任务" value={task.enabled} onChange={value => onChange({ ...task, enabled: value })} />
      {error && <p role="alert" className="text-destructive">{error}</p>}
      <DialogFooter>
        <Button type="button" variant="outline" disabled={busy} onClick={onCancel}>取消</Button>
        <Button type="submit" disabled={busy}>保存任务</Button>
      </DialogFooter>
    </FieldGroup>
  </form>
}
