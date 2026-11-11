import { useEffect, useRef } from 'react'
import { notify } from '@/lib/notifications'
import { useQueryClient } from '@tanstack/react-query'
import { rssAPI } from '@/lib/rss-api'
import { invalidateRSS } from '@/lib/refresh'
import { useRefresh, isRefreshing } from '@/stores/refresh'
export function useRSSSync() {
  const client = useQueryClient()
  const snapshot = useRefresh(state=>state.snapshot)
  const error = useRefresh(state => state.error)
  useEffect(() => {
    if (error) void notify(error, undefined, true)
  }, [error])
  const notified = useRef(-1)
  useEffect(() => {
    if (!snapshot?.operation_id || isRefreshing(snapshot) || snapshot.generation <= notified.current) return
    notified.current = snapshot.generation
    const failed = snapshot.failed_count ?? snapshot.results?.filter(result => result.status === 'failed').length ?? 0
    const skipped = snapshot.skipped_count ?? 0
    const cancelled = snapshot.cancelled_count ?? 0
    const title = snapshot.status === 'cancelled' ? '刷新已取消' : failed || skipped ? '刷新结束' : '刷新完成'
    const detail = [`${snapshot.completed}/${snapshot.total} 个订阅`, failed && `${failed} 个失败`, skipped && `${skipped} 个跳过`, cancelled && `${cancelled} 个取消`].filter(Boolean).join(' · ')
    void notify(title, detail, failed > 0, 'subscription-refresh')
  }, [snapshot])
  const operationID = isRefreshing(snapshot) ? snapshot?.operation_id : undefined
  useEffect(() => {
    if(!operationID) return
    let alive=true, pending=false
    const timer=setInterval(() => {
      if(pending) return
      pending=true
      void rssAPI.get(operationID).then(next=>{if(alive){useRefresh.getState().accept(next);invalidateRSS(client)}}).catch(()=>{
        if(alive) useRefresh.setState({error:'刷新状态同步失败，正在重试。'})
      }).finally(()=>{pending=false})
    },5000)
    return ()=>{alive=false;clearInterval(timer)}
  },[client,operationID])
  useEffect(() => {
    let alive = true
    let timer:ReturnType<typeof setTimeout>|undefined
    const changed = () => { if(timer===undefined) timer=setTimeout(() => { timer=undefined; if(alive) invalidateRSS(client) },80) }
    const stopRefresh = rssAPI.subscribeRefresh(snapshot => { if(alive) { useRefresh.getState().accept(snapshot); changed() } })
    const stopLibrary = rssAPI.subscribeLibrary(changed)
    void rssAPI.current().then(snapshot => { if(alive) useRefresh.getState().accept(snapshot) }).catch(() => { if(alive) useRefresh.setState({error:'刷新状态读取失败，可重新刷新。'}) })
    return () => { alive=false; stopRefresh(); stopLibrary(); clearTimeout(timer) }
  },[client])
}
