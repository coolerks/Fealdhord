import { useEffect, useMemo, useState } from 'react'
import { useInfiniteQuery, useQueryClient } from '@tanstack/react-query'
import { libraryAPI } from '@/lib/library-api'
import { libraryArticles, libraryKey, visibleArticles } from '@/lib/library-query'
import type { ArticleQuery } from '@/lib/library-query'
import { useWorkspace } from '@/stores/workspace'

export function useLibraryQuery() {
  const scope=useWorkspace(state=>state.preferences?.selected_scope ?? 'all')
  const scopeID=useWorkspace(state=>state.preferences?.selected_scope_id ?? '')
  const readFilter=useWorkspace(state=>state.readFilter),favoriteFilter=useWorkspace(state=>state.favoriteFilter)
  const search=useWorkspace(state=>state.search),sort=useWorkspace(state=>state.sort)
  const [term,setTerm]=useState(search)
  const [repairing,setRepairing]=useState(false),[repairError,setRepairError]=useState('')
  useEffect(()=>{const timer=setTimeout(()=>setTerm(search),150);return ()=>clearTimeout(timer)},[search])
  const request=useMemo<ArticleQuery>(()=>({scope:{kind:scope,...(scope==='feed'||scope==='category'?{id:scopeID}:{})},search:term,read:readFilter,favorite:favoriteFilter==='all'?undefined:favoriteFilter==='favorites',sort:sort==='oldest'?'published_asc':sort==='relevance'&&term?'relevance':'published_desc',limit:50}),[scope,scopeID,term,sort,readFilter,favoriteFilter])
  const key=useMemo(()=>libraryKey(request),[request])
  const result=useInfiniteQuery({queryKey:key,initialPageParam:undefined as string|undefined,
    queryFn:({pageParam,signal})=>libraryAPI.list({...request,cursor:pageParam},signal),
    getNextPageParam:page=>page.nextCursor || undefined,staleTime:30000,retry:false})
  const client=useQueryClient()
  useEffect(()=>{useWorkspace.setState({activeArticleQueryKey:key})},[key])
  const data=useMemo(()=>libraryArticles(result.data),[result.data])
  const articles=useMemo(()=>visibleArticles(data,request),[data,request])
  const error=result.error
  const expired=error instanceof Error && 'code' in error && error.code==='CursorExpired'
  async function repairSearch() {
    if(repairing) return
    setRepairing(true);setRepairError('')
    try {await libraryAPI.rebuildSearch();await client.invalidateQueries({queryKey:['library','articles']})}
    catch(error){setRepairError(error instanceof Error?error.message:'重建索引失败。')}
    finally {setRepairing(false)}
  }
  return {...result,repairSearch,repairing,repairError,searchLimited:result.data?.pages.some(page=>page.searchLimited) ?? false,articles,data,request,key,expired,searchPending:term!==search,reset:()=>client.resetQueries({queryKey:key,exact:true})}
}
