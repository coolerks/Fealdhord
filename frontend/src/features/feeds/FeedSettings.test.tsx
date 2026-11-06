import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'
import { preferencesFixture } from '@/test/fixtures'
import { useWorkspace } from '@/stores/workspace'
import type { Feed } from '@/lib/rss-api'
const api=vi.hoisted(()=>({list:vi.fn(),discover:vi.fn(),previewIcon:vi.fn(),add:vi.fn(),update:vi.fn(),archive:vi.fn(),restore:vi.fn(),clearCache:vi.fn(),start:vi.fn()}))
vi.mock('@/lib/rss-api',()=>({rssAPI:api,feedsQueryKey:['rss','feeds']}))
vi.mock('@/lib/preferences',()=>({preferencesAPI:{update:vi.fn()}}))
vi.mock('@/lib/navigation-api',()=>({navigationQueryKey:['navigation'],navigationAPI:{list:vi.fn(async()=>({feeds:[],categories:[],totals:{total:0,unread:0,favorites:0}}))}}))
import { FeedSettings } from './FeedSettings'
const feed:Feed={id:'one',title:'测试源',url:'https://example.test/feed',revision:4,archived:false,site_url:'',description:'',last_error:'',category_ids:[]}
function setup() { return render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}})}><FeedSettings/></QueryClientProvider>) }
beforeEach(()=>{
  vi.resetAllMocks();useWorkspace.setState({preferences:preferencesFixture()})
  api.list.mockResolvedValue({items:[feed],categories:[]})
  api.previewIcon.mockResolvedValue('')
})
it('网页发现多个源后必须选择，再添加；失败保留输入供重试',async()=>{
  api.discover.mockResolvedValue([{title:'全文',url:'https://example.test/full',type:'rss'},{title:'摘要',url:'https://example.test/short',type:'atom'}])
  api.add.mockRejectedValueOnce(new Error('无法连接订阅')).mockResolvedValueOnce(feed)
  setup();await userEvent.type(screen.getByRole('textbox',{name:'订阅或网页地址'}),'https://example.test')
  await userEvent.click(screen.getByRole('button',{name:'查找订阅'}))
  const add=await screen.findByRole('button',{name:'添加订阅'})
  expect(add).toBeDisabled();expect(api.add).not.toHaveBeenCalled()
  await userEvent.click(screen.getByRole('radio',{name:/全文/}));await userEvent.click(add)
  expect(await screen.findByRole('alert')).toHaveTextContent('无法连接订阅')
  expect(screen.getByRole('textbox',{name:'订阅或网页地址'})).toHaveValue('https://example.test')
  await userEvent.click(add)
  await waitFor(()=>expect(api.add).toHaveBeenLastCalledWith({url:'https://example.test/full',title:'全文',category_ids:[]}))
  await waitFor(()=>expect(screen.queryByRole('button',{name:'添加订阅'})).not.toBeInTheDocument())
})
it('编辑保存携带打开时的 revision；冲突不会自动覆盖或关闭',async()=>{
  api.update.mockRejectedValue(new Error('数据已变更，请重新载入'))
  setup();await userEvent.click(await screen.findByRole('button',{name:'编辑 测试源'}))
  const dialog=screen.getByRole('dialog',{name:'编辑订阅'})
  await userEvent.clear(within(dialog).getByRole('textbox',{name:'名称'}));await userEvent.type(within(dialog).getByRole('textbox',{name:'名称'}),'新名称')
  await userEvent.click(within(dialog).getByRole('button',{name:'保存订阅'}))
  expect(await within(dialog).findByRole('alert')).toHaveTextContent('数据已变更')
  expect(api.update).toHaveBeenCalledWith(expect.objectContaining({revision:4,title:'新名称'}),expect.any(AbortSignal))
})
it('归档先确认并保留文章，归档源可以恢复',async()=>{
  api.archive.mockResolvedValue({...feed,archived:true,revision:5})
  setup();await userEvent.click(await screen.findByRole('button',{name:'归档 测试源'}))
  expect(api.archive).not.toHaveBeenCalled()
  const dialog=screen.getByRole('dialog',{name:'归档“测试源”？'})
  expect(within(dialog).getByText(/保留文章、已读和收藏/)).toBeInTheDocument()
  api.list.mockResolvedValue({items:[{...feed,archived:true,revision:5}],categories:[]})
  await userEvent.click(within(dialog).getByRole('button',{name:'确认归档'}))
  await waitFor(()=>expect(api.archive).toHaveBeenCalledWith({id:'one',revision:4}))
  await userEvent.click(screen.getByRole('switch',{name:'显示已归档订阅'}))
  await userEvent.click(await screen.findByRole('button',{name:'恢复订阅'}))
  await waitFor(()=>expect(api.restore).toHaveBeenCalledWith({id:'one',revision:5}))
})

it('有网站图标时默认选中，用户仍可改用默认图标',async()=>{
  api.discover.mockResolvedValue([{title:'测试源',url:feed.url,type:'rss'}])
  api.previewIcon.mockResolvedValue('data:image/x-icon;base64,AA==')
  api.add.mockResolvedValue(feed)
  setup()
  await userEvent.type(screen.getByRole('textbox',{name:'订阅或网页地址'}),feed.url)
  await userEvent.click(screen.getByRole('button',{name:'查找订阅'}))
  const website=await screen.findByRole('radio',{name:'网站图标'})
  await waitFor(()=>expect(website).toHaveAttribute('aria-checked','true'))
  await userEvent.click(screen.getByRole('radio',{name:'默认图标'}))
  await userEvent.click(screen.getByRole('button',{name:'添加订阅'}))
  await waitFor(()=>expect(api.add).toHaveBeenCalledWith(expect.objectContaining({use_default_icon:true})))
})
