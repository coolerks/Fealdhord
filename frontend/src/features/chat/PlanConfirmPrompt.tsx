import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, ClipboardList, MessageSquare, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PendingPlans, ResolvePlan } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api'
import type { PendingPlan } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/models'

export const pendingPlansKey = ['chat', 'pending-plans'] as const

export function useInlinePlan(conversationId: string, enabled: boolean, active: boolean) {
  const query = useQuery({
    queryKey: pendingPlansKey,
    queryFn: () => PendingPlans(),
    enabled: Boolean(conversationId && enabled && active),
    refetchInterval: enabled && active ? 450 : false,
    retry: false,
  })
  if (!conversationId || !enabled || !active) return undefined
  return query.data?.find(item => item.conversationId === conversationId)
}

export function PlanConfirmPrompt({ pending, onDecision, onStop, stopping }: {
  pending: PendingPlan
  onDecision?: (decision: 'approve' | 'revise' | 'reject') => void
  onStop: () => void
  stopping: boolean
}) {
  const cache = useQueryClient()
  const [feedback, setFeedback] = useState('')
  const [revising, setRevising] = useState(false)
  const resolve = useMutation({
    mutationFn: ({ decision, note }: { decision: 'approve' | 'revise' | 'reject'; note: string }) =>
      ResolvePlan(pending.id, decision, note),
    onSuccess: (_ok, { decision }) => {
      void cache.invalidateQueries({ queryKey: pendingPlansKey })
      onDecision?.(decision)
    },
  })

  return <div className="space-y-3">
    <div className="flex items-start justify-between gap-2">
      <div className="flex min-w-0 items-center gap-2">
        <ClipboardList className="size-4 shrink-0 text-primary" aria-hidden="true" />
        <div className="min-w-0">
          <p className="truncate text-sm font-medium text-foreground">{pending.title}</p>
          {pending.summary && <p className="text-xs text-muted-foreground">{pending.summary}</p>}
        </div>
      </div>
      <Button type="button" size="xs" variant="ghost" disabled={stopping || resolve.isPending} onClick={onStop}>
        停止回复
      </Button>
    </div>

    {pending.steps && pending.steps.length > 0 && (
      <ol className="max-h-48 space-y-1.5 overflow-y-auto rounded-lg border border-border/80 bg-muted/35 px-3 py-2 text-xs text-foreground">
        {pending.steps.map((step, index) => (
          <li key={index} className="flex gap-2">
            <span className="shrink-0 font-medium tabular-nums text-muted-foreground">{index + 1}.</span>
            <span className="min-w-0 break-words">{step}</span>
          </li>
        ))}
      </ol>
    )}

    {revising && (
      <form className="flex items-center gap-2" onSubmit={event => {
        event.preventDefault()
        if (!feedback.trim() || resolve.isPending) return
        resolve.mutate({ decision: 'revise', note: feedback.trim() })
      }}>
        <Input
          aria-label="修改意见"
          placeholder="输入对计划的修改或补充意见…"
          value={feedback}
          disabled={resolve.isPending}
          onChange={event => setFeedback(event.target.value)}
          autoFocus
        />
        <Button type="submit" size="sm" disabled={!feedback.trim() || resolve.isPending}>发送意见</Button>
        <Button type="button" size="sm" variant="ghost" disabled={resolve.isPending} onClick={() => setRevising(false)}>取消</Button>
      </form>
    )}

    {!revising && (
      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button type="button" size="sm" variant="ghost" disabled={resolve.isPending}
          onClick={() => resolve.mutate({ decision: 'reject', note: '' })}>
          <X className="size-3.5" />不执行计划
        </Button>
        <Button type="button" size="sm" variant="outline" disabled={resolve.isPending}
          onClick={() => setRevising(true)}>
          <MessageSquare className="size-3.5" />修改计划
        </Button>
        <Button type="button" size="sm" disabled={resolve.isPending}
          onClick={() => resolve.mutate({ decision: 'approve', note: '' })}>
          <Check className="size-3.5" />确认并执行
        </Button>
      </div>
    )}

    {resolve.isError && <p role="alert" className="text-xs text-destructive">无法提交计划确认，请重试。</p>}
  </div>
}
