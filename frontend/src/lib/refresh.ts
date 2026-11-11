import { notify } from './notifications'
import type { QueryClient } from '@tanstack/react-query'
import { rssAPI, feedsQueryKey } from './rss-api'
import { articleQueryKey } from './library'
import { useRefresh } from '@/stores/refresh'
export function invalidateRSS(client:QueryClient) {
  void client.invalidateQueries({queryKey:['navigation']})
  void client.invalidateQueries({queryKey:articleQueryKey})
  void client.invalidateQueries({queryKey:feedsQueryKey})
}
export async function startRefresh(client:QueryClient, ids:string[] = []) {
  if(useRefresh.getState().pending) return
  useRefresh.setState({pending:true,error:null})
  try { useRefresh.getState().accept(await rssAPI.start(ids)); invalidateRSS(client) }
  catch(error) { void notify('刷新失败', error instanceof Error ? error.message : undefined, true) }
  finally { useRefresh.setState({pending:false}) }
}
export async function cancelRefresh() {
  const id = useRefresh.getState().snapshot?.operation_id
  if(!id) return
  try { useRefresh.getState().accept(await rssAPI.cancel(id)) }
  catch(error) { void notify('取消刷新失败', error instanceof Error ? error.message : undefined, true) }
}
