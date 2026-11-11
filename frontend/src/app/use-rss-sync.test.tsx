import { StrictMode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, afterEach, expect, it, vi } from 'vitest'
import type { Snapshot } from '@/lib/rss-api'
import { useRefresh } from '@/stores/refresh'
const api=vi.hoisted(()=>({current:vi.fn(),get:vi.fn(),subscribeRefresh:vi.fn(),subscribeLibrary:vi.fn()}))
vi.mock('@/lib/rss-api',()=>({rssAPI:api,feedsQueryKey:['rss','feeds']}))
const notify = vi.hoisted(() => vi.fn())
vi.mock('@/lib/notifications', () => ({ notify }))
import { useRSSSync } from './use-rss-sync'
const running:Snapshot={generation:2,sequence:3,status:'running',operation_id:'new',completed:1,total:2,results:[]}
beforeEach(()=>{
  vi.resetAllMocks();useRefresh.setState({snapshot:null,error:null,dismissedGeneration:-1,pending:false})
  api.subscribeRefresh.mockReturnValue(vi.fn());api.subscribeLibrary.mockReturnValue(vi.fn());api.current.mockResolvedValue(running)
})
afterEach(()=>vi.useRealTimers())
function setup(strict=false) {
 const client=new QueryClient(), invalidate=vi.spyOn(client,'invalidateQueries')
 const wrapper=({children}:{children:React.ReactNode})=><QueryClientProvider client={client}>{strict?<StrictMode>{children}</StrictMode>:children}</QueryClientProvider>
 return {...renderHook(useRSSSync,{wrapper}),invalidate}
}
it('订阅先于快照请求，早到事件不被旧响应覆盖；卸载清理全部订阅',async()=>{
  api.subscribeRefresh.mockImplementation(listener=>{listener(running);return vi.fn()})
  api.current.mockResolvedValue({...running,generation:1,sequence:100})
  const {unmount}=setup(true)
  await waitFor(()=>expect(useRefresh.getState().snapshot).toEqual(running))
  expect(api.subscribeRefresh.mock.invocationCallOrder[0]).toBeLessThan(api.current.mock.invocationCallOrder[0])
  unmount()
  for(const result of [...api.subscribeRefresh.mock.results,...api.subscribeLibrary.mock.results]) expect(result.value).toHaveBeenCalledTimes(1)
})
it('合并频繁领域事件为一次 Query 重取，并清理尚未执行的更新',async()=>{
  const {unmount,invalidate}=setup()
  await waitFor(()=>expect(useRefresh.getState().snapshot).not.toBeNull())
  vi.useFakeTimers()
  const changed=api.subscribeLibrary.mock.calls[0][0]
  act(()=>{changed();changed();changed()})
  await act(()=>vi.advanceTimersByTimeAsync(80))
  expect(invalidate).toHaveBeenCalledTimes(3)
  act(()=>changed());unmount();await vi.advanceTimersByTimeAsync(100)
  expect(invalidate).toHaveBeenCalledTimes(3)
})
it('丢失结束事件时轮询恢复终态，不持续重复请求',async()=>{
  api.get.mockResolvedValue({...running,sequence:4,status:'completed',completed:2})
  const {unmount}=setup()
  await waitFor(()=>expect(useRefresh.getState().snapshot?.operation_id).toBe('new'))
  vi.useFakeTimers()
  // 用真实计时器创建的旧 effect 先卸载，再在假时钟内挂载。
  unmount();const next=setup()
  await act(()=>vi.advanceTimersByTimeAsync(0))
  await act(()=>vi.advanceTimersByTimeAsync(5000))
  expect(api.get).toHaveBeenCalledWith('new')
  expect(useRefresh.getState().snapshot?.status).toBe('completed')
  await act(()=>vi.advanceTimersByTimeAsync(10000))
  expect(api.get).toHaveBeenCalledTimes(1);next.unmount()
})

it('刷新终态通过通知报告累计失败数，同一批次只通知一次', async () => {
  setup()
  await waitFor(() => expect(useRefresh.getState().snapshot).toEqual(running))
  const listener = api.subscribeRefresh.mock.calls[0][0]
  const completed = {...running, sequence:4, status:'partial', completed:2, failed_count:2, results_truncated:true}
  act(() => listener(completed))
  expect(notify).toHaveBeenCalledWith('刷新结束', '2/2 个订阅 · 2 个失败', true)
  act(() => listener({...completed, sequence:5}))
  expect(notify).toHaveBeenCalledTimes(1)
})
