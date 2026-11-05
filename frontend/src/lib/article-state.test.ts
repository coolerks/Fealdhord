import { beforeEach,describe,expect,it,vi } from 'vitest'
import { QueryClient } from '@tanstack/react-query'
import { articleFixtures } from '@/test/fixtures'
import { articleQueryKey } from './library'
import { initialLibraryData,libraryArticles } from './library-query'
import type { LibraryData } from './library-query'
import { useWorkspace } from '@/stores/workspace'
const api=vi.hoisted(()=>({setFavorite:vi.fn<(id:string,value:boolean)=>Promise<void>>(),setRead:vi.fn<(id:string,value:boolean)=>Promise<void>>()}))
vi.mock('./library-api',()=>({libraryAPI:api}))
vi.mock('./preferences',()=>({preferencesAPI:{get:vi.fn(),update:vi.fn()}}))
import { toggleArticleState } from './article-state'
const flush=async()=>{for(let index=0;index<12;index++)await Promise.resolve()}
function setup(){const client=new QueryClient();const article={...articleFixtures(1)[0],isFavorite:false,isRead:false};client.setQueryData(articleQueryKey,initialLibraryData({items:[article],hasMore:false}));return client}
function favorite(client:QueryClient){return libraryArticles(client.getQueryData<LibraryData>(articleQueryKey))[0].isFavorite}
beforeEach(()=>{vi.resetAllMocks();api.setFavorite.mockResolvedValue();api.setRead.mockResolvedValue();useWorkspace.setState({commandError:null})})
describe('文章状态串行落盘',()=>{
 it('连续反转即时反馈，按顺序保存最终意图',async()=>{
  const client=setup();let release:()=>void=()=>{}
  api.setFavorite.mockImplementationOnce(()=>new Promise<void>(resolve=>{release=resolve}))
  toggleArticleState(client,'a-0','favorite');await flush()
  toggleArticleState(client,'a-0','favorite')
  expect(favorite(client)).toBe(false)
  expect(api.setFavorite).toHaveBeenCalledTimes(1)
  release();await flush()
  expect(api.setFavorite.mock.calls).toEqual([['a-0',true],['a-0',false]])
  expect(favorite(client)).toBe(false)
 })
 it('最新保存失败回滚到最后成功值，不回滚到失败过的乐观值',async()=>{
  const client=setup()
  api.setFavorite.mockRejectedValueOnce(new Error('断开')).mockRejectedValueOnce(new Error('断开'))
  toggleArticleState(client,'a-0','favorite');toggleArticleState(client,'a-0','favorite')
  await flush()
  expect(favorite(client)).toBe(false)
  expect(useWorkspace.getState().commandError).toMatch('保存失败')
  api.setFavorite.mockResolvedValueOnce()
  toggleArticleState(client,'a-0','favorite');await flush()
  expect(favorite(client)).toBe(true)
  api.setFavorite.mockRejectedValueOnce(new Error('失败'))
  toggleArticleState(client,'a-0','favorite');await flush()
  expect(favorite(client)).toBe(true)
 })
 it('失败的过时意图不覆盖更新的成功意图，已读与收藏分别持久化',async()=>{
  const client=setup()
  api.setFavorite.mockRejectedValueOnce(new Error('第一次失败')).mockResolvedValueOnce()
  toggleArticleState(client,'a-0','favorite');toggleArticleState(client,'a-0','favorite');toggleArticleState(client,'a-0','read')
  await flush()
  expect(favorite(client)).toBe(false)
  expect(api.setRead).toHaveBeenCalledWith('a-0',true)
  expect(useWorkspace.getState().commandError).toBeNull()
 })
})

it('同篇状态写入覆盖所有分页查询，失败将各视图回滚到已保存值',async()=>{
 const client=setup()
 const other=[...articleQueryKey,{scope:{kind:'favorites'}}]
 client.setQueryData(other,{pages:[{items:[],hasMore:true},{items:[{...articleFixtures(1)[0],isFavorite:false}],hasMore:false}],pageParams:[undefined,'second']})
 api.setFavorite.mockRejectedValueOnce(new Error('数据库失败'))
 toggleArticleState(client,'a-0','favorite')
 expect(libraryArticles(client.getQueryData<LibraryData>(other))[0].isFavorite).toBe(true)
 await flush()
 expect(favorite(client)).toBe(false)
 expect(libraryArticles(client.getQueryData<LibraryData>(other))[0].isFavorite).toBe(false)
})
