import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { rssAPI } from '@/lib/rss-api'
import { invalidateRSS } from '@/lib/refresh'
import { useRefresh, isRefreshing } from '@/stores/refresh'
export function useRSSSync() {
  const client = useQueryClient()
  const snapshot = useRefresh(state=>state.snapshot)
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
