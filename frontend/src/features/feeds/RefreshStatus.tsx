import { Button } from '@/components/ui/button'
import { useRefresh, isRefreshing } from '@/stores/refresh'
import { cancelRefresh } from '@/lib/refresh'
export function RefreshStatus() {
  const snapshot=useRefresh(state=>state.snapshot), error=useRefresh(state=>state.error), dismiss=useRefresh(state=>state.dismiss)
  const active=isRefreshing(snapshot)
  const failures=snapshot?.results?.filter(result=>result.status==='failed') ?? []
  const failedCount=snapshot?.failed_count ?? failures.length
  const successCount=snapshot?.success_count ?? snapshot?.results?.filter(result=>result.status==='completed').length ?? 0
  const cancelledCount=snapshot?.cancelled_count ?? snapshot?.results?.filter(result=>result.status==='cancelled').length ?? 0
  const skippedCount=snapshot?.skipped_count ?? snapshot?.results?.filter(result=>result.status==='skipped').length ?? 0
  if(!snapshot?.operation_id && !error) return null
  return <div className="refresh-status" role="status" aria-live="polite">
    {snapshot?.operation_id && <><span>{active ? snapshot.status==='cancelling' ? '正在取消' : '正在刷新' : snapshot.status==='cancelled' ? `已取消刷新，成功 ${successCount}，未处理 ${cancelledCount}` : failedCount ? `刷新结束，${failedCount} 个订阅失败` : skippedCount ? `刷新结束，${skippedCount} 个订阅跳过` : '刷新完成'} · {snapshot.completed}/{snapshot.total}</span>{active && <progress aria-label="订阅刷新进度" value={snapshot.completed} max={Math.max(1,snapshot.total)}/>}</>}
    {failures.slice(0,3).map(result=><p key={result.feed_id}>{result.message}</p>)}
    {error && <p role="alert">{error}</p>}
    {active ? <Button size="sm" variant="ghost" disabled={snapshot?.status==='cancelling'} onClick={()=>{void cancelRefresh()}}>取消刷新</Button> : <Button size="sm" variant="ghost" onClick={dismiss}>关闭</Button>}
  </div>
}
