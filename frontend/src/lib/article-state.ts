import type { QueryClient } from '@tanstack/react-query'
import { articleQueryKey } from './library'
import { findCachedArticle,updateCachedArticle } from './library-query'
import { libraryAPI } from './library-api'
import { useWorkspace } from '@/stores/workspace'

type Entry={saved:boolean;version:number;chain:Promise<void>}
const writers=new WeakMap<QueryClient,Map<string,Entry>>()
// 同篇同字段串行落盘；失败仅回滚最新意图到最后一次真正保存的值。
export function toggleArticleState(client:QueryClient,id:string,kind:'favorite'|'read') {
 const field=kind==='favorite'?'isFavorite':'isRead'
 const article=findCachedArticle(client,id)
 if(!article)return
 const value=!article[field]
 let entries=writers.get(client);if(!entries){entries=new Map();writers.set(client,entries)}
 const key=`${kind}|${id}`
 let entry=entries.get(key)
 if(!entry){entry={saved:article[field],version:0,chain:Promise.resolve()};entries.set(key,entry)}
 const current=entry,version=++current.version
 const update=(next:boolean)=>updateCachedArticle(client,id,{[field]:next})
 update(value)
 current.chain=current.chain.then(async()=>{
  await (kind==='favorite'?libraryAPI.setFavorite(id,value):libraryAPI.setRead(id,value))
  current.saved=value
 }).catch(()=>{
  if(current.version===version){update(current.saved);useWorkspace.setState({commandError:'文章状态保存失败，已恢复最后保存的状态。请重试。'})}
 }).finally(()=>{if(current.version===version){entries.delete(key);void client.invalidateQueries({queryKey:['navigation']});void client.invalidateQueries({predicate:query=>query.queryKey.length>2 && query.queryKey[0]===articleQueryKey[0] && query.queryKey[1]===articleQueryKey[1]})}})
}
