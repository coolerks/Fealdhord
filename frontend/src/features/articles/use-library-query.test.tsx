import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'vitest'
import { useWorkspace } from '@/stores/workspace'
import { preferencesFixture, articleFixtures } from '@/test/fixtures'
import type { ArticleQuery } from '@/lib/library-query'
import type { LibraryPage } from '@/lib/library'
const api=vi.hoisted(()=>({list:vi.fn<(request:ArticleQuery,signal:AbortSignal)=>Promise<LibraryPage>>()}))
vi.mock('@/lib/library-api',()=>({libraryAPI:api}))
vi.mock('@/lib/preferences',()=>({preferencesAPI:{get:vi.fn(),update:vi.fn()}}))
import { useLibraryQuery } from './use-library-query'
function setup(){const client=new QueryClient();return renderHook(useLibraryQuery,{wrapper:({children}:{children:React.ReactNode})=><QueryClientProvider client={client}>{children}</QueryClientProvider>})}
beforeEach(()=>{vi.resetAllMocks();useWorkspace.setState({preferences:preferencesFixture(),sort:'relevance',readFilter:'all',favoriteFilter:'all',search:'',activeArticleQueryKey:null})})
it('数据库分页合入同一 Query，两视图不重置分页',async()=>{
 const items=articleFixtures(120)
 api.list.mockImplementation(async request=>request.cursor ? {items:items.slice(50,100),hasMore:true,nextCursor:'third'} : {items:items.slice(0,50),hasMore:true,nextCursor:'second'})
 const {result}=setup();await waitFor(()=>expect(result.current.articles).toHaveLength(50))
 await act(()=>result.current.fetchNextPage())
 await waitFor(()=>expect(result.current.articles).toHaveLength(100))
 act(()=>useWorkspace.setState(state=>({preferences:{...state.preferences!,article_view_mode:'masonry'}})))
 expect(result.current.articles).toHaveLength(100);expect(api.list).toHaveBeenCalledTimes(2)
 expect(api.list.mock.calls[1][0]).toEqual(expect.objectContaining({cursor:'second',limit:50}))
})
it('新搜索取消旧请求，迟到旧结果不会盖住当前查询',async()=>{
 let release:(page:LibraryPage)=>void=()=>{};let oldSignal:AbortSignal|undefined
 api.list.mockImplementation((request,signal)=>{
  if(request.search==='旧') {oldSignal=signal;return new Promise(resolve=>{release=resolve})}
  return Promise.resolve({items:request.search==='新'?[{...articleFixtures(1)[0],title:'新结果'}]:[],hasMore:false})
 })
 const {result}=setup();await waitFor(()=>expect(result.current.isSuccess).toBe(true))
 act(()=>useWorkspace.setState({search:'旧'}));await waitFor(()=>expect(oldSignal).toBeDefined())
 act(()=>useWorkspace.setState({search:'新'}));await waitFor(()=>expect(result.current.articles[0]?.title).toBe('新结果'))
 expect(oldSignal?.aborted).toBe(true)
 await act(async()=>release({items:[{...articleFixtures(1)[0],title:'旧结果'}],hasMore:false}))
 expect(result.current.articles[0].title).toBe('新结果')
 expect(api.list.mock.calls.at(-1)?.[0].sort).toBe('relevance')
})
it('过期游标保留已加载内容，重新读取可恢复并继续',async()=>{
 let expired=true
 api.list.mockImplementation(async request=>{
  if(request.cursor && expired) throw Object.assign(new Error('列表已变化'),{code:'CursorExpired'})
  return {items:articleFixtures(2),hasMore:!request.cursor,nextCursor:request.cursor?undefined:'next'}
 })
 const {result}=setup();await waitFor(()=>expect(result.current.articles).toHaveLength(2))
 await act(()=>result.current.fetchNextPage())
 await waitFor(()=>expect(result.current.expired).toBe(true));expect(result.current.articles).toHaveLength(2)
 expired=false;await act(()=>result.current.refetch())
 await waitFor(()=>expect(result.current.expired).toBe(false));expect(result.current.articles).toHaveLength(2)
})
