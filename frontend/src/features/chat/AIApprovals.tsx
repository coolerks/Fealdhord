import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DecideAction, PendingActions } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api'

const key = ['ai-platform', 'pending']

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
      <pre className="max-h-52 overflow-auto whitespace-pre-wrap break-all rounded-md bg-muted p-3 text-xs">{action.arguments}</pre>
      {decision.isError && <p role="alert">提交失败，请重试。</p>}
      <DialogFooter>
        <Button variant="outline" disabled={decision.isPending} onClick={() => decision.mutate({ id: action.id, allow: false })}>拒绝</Button>
        <Button disabled={decision.isPending} onClick={() => decision.mutate({ id: action.id, allow: true })}>允许本次调用</Button>
      </DialogFooter>
    </DialogContent>}
  </Dialog>
}
