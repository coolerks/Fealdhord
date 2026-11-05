import { StrictMode } from 'react'
import { QueryClient,QueryClientProvider } from '@tanstack/react-query'
import { act,render,screen,waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach,expect,it,vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { preferencesFixture,articleFixtures } from '@/test/fixtures'
import { useWorkspace } from '@/stores/workspace'
import type { Detail } from '@/lib/reader-api'
const api=vi.hoisted(()=>({get:vi.fn(),extract:vi.fn(),open:vi.fn()}))
vi.mock('@/lib/reader-api',()=>({readerAPI:api}))
vi.mock('@/lib/preferences',()=>({preferencesAPI:{get:vi.fn(),update:vi.fn()}}))
vi.mock('@/lib/navigation-api',()=>({navigationQueryKey:['navigation'],navigationAPI:{list:vi.fn(async()=>({categories:[],feeds:[],totals:{total:0,unread:0,favorites:0}}))}}))
import { Reader } from './Reader'
const link='a'.repeat(64)
const feed:Detail={article_id:'a-0',mode:'feed',html:`<p>订阅正文 <a href="#" data-reader-link="${link}">正文链接</a></p><pre><code>const value = 1;</code></pre><table><tbody><tr><td>表格</td></tr></tbody></table>`,text:'订阅正文',has_cached:false,has_original:true}
const extracted:Detail={...feed,mode:'reader',html:'<p>提取后的正文</p>',text:'提取后的正文',has_cached:true}
beforeEach(()=>{vi.resetAllMocks();api.get.mockResolvedValue(feed);api.open.mockResolvedValue(undefined);useWorkspace.setState({preferences:preferencesFixture(),readerModeRequestID:null})})
function setup(patch:Parameters<typeof preferencesFixture>[0]={}){
 const prefs=preferencesFixture(patch),onRead=vi.fn(),article={...articleFixtures(1)[0],contentText:'',isRead:false,url:'https://example.test/article'}
 const renderTree=(revision=1)=><StrictMode><QueryClientProvider client={client}><TooltipProvider><Reader article={{...article,revision}} preferences={prefs} overlay onClose={vi.fn()} onFavorite={vi.fn()} onRead={onRead} onAnchor={vi.fn()}/></TooltipProvider></QueryClientProvider></StrictMode>
 const client=new QueryClient({defaultOptions:{queries:{retry:false}}})
 return {...render(renderTree()),client,onRead,renderTree}
}
it('正文成功呈现后只标已读一次，代码表格保留；原文和正文链接交给Go',async()=>{
 const {onRead}=setup({reader_font_family:'serif'})
 expect(await screen.findByText('订阅正文')).toBeInTheDocument()
 await waitFor(()=>expect(onRead).toHaveBeenCalledTimes(1))
 expect(screen.getByText('const value = 1;').tagName).toBe('CODE')
 expect(screen.getByText('表格').tagName).toBe('TD')
 expect(document.querySelector('.reader-content article')?.getAttribute('style')).toContain('ui-serif')
 await userEvent.click(screen.getByRole('link',{name:'正文链接'}));expect(api.open).toHaveBeenLastCalledWith('a-0',link)
 await userEvent.click(screen.getByRole('button',{name:'原文'}));expect(api.open).toHaveBeenLastCalledWith('a-0','')
 expect(onRead).toHaveBeenCalledTimes(1)
})
it('取消提取保留订阅正文，迟到响应不能切换当前内容',async()=>{
 let release:(value:Detail)=>void=()=>{};let signal:AbortSignal|undefined
 api.extract.mockImplementation((_id:string,next:AbortSignal)=>{signal=next;return new Promise(resolve=>{release=resolve})})
 setup();await screen.findByText('订阅正文')
 await userEvent.click(screen.getByRole('button',{name:'Reader 模式'}))
 expect(screen.getByText('订阅正文')).toBeInTheDocument()
 await userEvent.click(screen.getByRole('button',{name:'取消提取'}));expect(signal?.aborted).toBe(true)
 await act(async()=>release(extracted))
 expect(screen.getByText('订阅正文')).toBeInTheDocument();expect(screen.queryByText('提取后的正文')).not.toBeInTheDocument()
 expect(screen.getByRole('alert')).toHaveTextContent('提取已取消')
})
it('提取成功缓存两种模式，往返切换不重复访问原网页',async()=>{
 api.extract.mockResolvedValue(extracted)
 setup();await screen.findByText('订阅正文')
 const toggle=screen.getByRole('button',{name:'Reader 模式'})
 await userEvent.click(toggle);expect(await screen.findByText('提取后的正文')).toBeInTheDocument()
 await userEvent.click(toggle);expect(await screen.findByText('订阅正文')).toBeInTheDocument()
 await userEvent.click(toggle);expect(await screen.findByText('提取后的正文')).toBeInTheDocument()
 expect(api.extract).toHaveBeenCalledTimes(1)
})
it('关闭自动已读或正文读取失败均不标记；重试成功后可继续阅读',async()=>{
 api.get.mockRejectedValue(new Error('数据库不可用'))
 const {onRead}=setup({reader_auto_read:false})
 expect(await screen.findByRole('alert')).toHaveTextContent('正文读取失败')
 expect(onRead).not.toHaveBeenCalled()
 api.get.mockResolvedValue(feed)
 await userEvent.click(screen.getByRole('button',{name:'重试'}));await screen.findByText('订阅正文')
 expect(onRead).not.toHaveBeenCalled()
})
it('文章新revision不会继续使用旧正文缓存',async()=>{
 const {rerender,renderTree}=setup();await screen.findByText('订阅正文')
 api.get.mockResolvedValue({...feed,html:'<p>新版本正文</p>',text:'新版本正文'})
 rerender(renderTree(2));expect(await screen.findByText('新版本正文')).toBeInTheDocument()
 expect(screen.queryByText('订阅正文')).not.toBeInTheDocument()
})

it('已显示文章更新时取消旧提取，不遗留忙碌状态或再次覆盖新正文',async()=>{
 let release:(value:Detail)=>void=()=>{};let signal:AbortSignal|undefined
 api.extract.mockImplementation((_id:string,next:AbortSignal)=>{signal=next;return new Promise(resolve=>{release=resolve})})
 const {rerender,renderTree,onRead}=setup();await screen.findByText('订阅正文')
 await userEvent.click(screen.getByRole('button',{name:'Reader 模式'}))
 api.get.mockResolvedValue({...feed,html:'<p>更新后的订阅正文</p>',text:'更新后的订阅正文'})
 rerender(renderTree(2));await screen.findByText('更新后的订阅正文')
 expect(signal?.aborted).toBe(true);expect(screen.getByRole('button',{name:'Reader 模式'})).toBeEnabled()
 await act(async()=>release(extracted))
 expect(screen.queryByText('提取后的正文')).not.toBeInTheDocument()
 expect(onRead).toHaveBeenCalledTimes(1)
})
it('菜单请求在Reader挂载前到达也只执行一次',async()=>{
 api.extract.mockResolvedValue(extracted);useWorkspace.setState({readerModeRequestID:'a-0'})
 setup();expect(await screen.findByText('提取后的正文')).toBeInTheDocument()
 expect(api.extract).toHaveBeenCalledTimes(1);expect(useWorkspace.getState().readerModeRequestID).toBeNull()
})
