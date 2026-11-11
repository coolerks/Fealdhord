import { Button } from '@/components/ui/button'
import { useRefresh, isRefreshing } from '@/stores/refresh'
import { cancelRefresh } from '@/lib/refresh'

export function RefreshStatus() {
  const snapshot = useRefresh(state => state.snapshot)
  if (!snapshot || !isRefreshing(snapshot)) return null
  const cancelling = snapshot.status === 'cancelling'
  return (
    <div className="refresh-status" role="status">
      <span>{cancelling ? '正在取消' : '正在刷新'} · {snapshot.completed}/{snapshot.total}</span>
      <progress aria-label="订阅刷新进度" value={snapshot.completed} max={Math.max(1, snapshot.total)} />
      <Button size="sm" variant="ghost" disabled={cancelling} onClick={() => { void cancelRefresh() }}>取消刷新</Button>
    </div>
  )
}
