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
vi.mock('@/lib/navigation-api',()=>({navigationQueryKey:['navigation'],navigationAPI:{list:vi.fn(async()=>({categories:[],feeds:[],totals:{total:0,unread:0,favorites:0}})),memberships:async()=>[]}}))
import { Reader } from './Reader'
const link='a'.repeat(64)
const feed:Detail={article_id:'a-0',mode:'feed',html:`<p>订阅正文 <a href="#" data-reader-link="${link}">正文链接</a></p><pre><code>const value = 1;</code></pre><table><tbody><tr><td>表格</td></tr></tbody></table>`,text:'订阅正文',has_cached:false,has_original:true}
const extracted:Detail={...feed,mode:'reader',html:'<p>提取后的正文</p>',text:'提取后的正文',has_cached:true}
beforeEach(()=>{vi.resetAllMocks();api.get.mockResolvedValue(feed);api.open.mockResolvedValue(undefined);useWorkspace.setState({preferences:preferencesFixture(),readerModeRequestID:null})})
function setup(patch:Parameters<typeof preferencesFixture>[0]={}){
 const prefs=preferencesFixture(patch),onRead=vi.fn(),article={...articleFixtures(1)[0],contentText:'',isRead:false,url:'https://example.test/article'}
 const renderTree=(revision=1,isRead=false)=><StrictMode><QueryClientProvider client={client}><TooltipProvider><Reader article={{...article,revision,isRead}} preferences={prefs} overlay onClose={vi.fn()} onFavorite={vi.fn()} onRead={onRead} onAnchor={vi.fn()}/></TooltipProvider></QueryClientProvider></StrictMode>
 const client=new QueryClient({defaultOptions:{queries:{retry:false}}})
 return {...render(renderTree()),client,onRead,renderTree}
}
async function selectMoreItem(name:string){
 await userEvent.click(screen.getByRole('button',{name:'更多'}))
 await userEvent.click(await screen.findByRole('menuitem',{name}))
}

async function selectReaderMode(name:'阅读模式' | '订阅预览'){
 await selectMoreItem('阅读方式')
 await userEvent.click(await screen.findByRole('menuitemradio',{name}))
}

it('正文成功呈现后只标已读一次，代码表格保留；原文和正文链接交给Go',async()=>{
 const {onRead}=setup({reader_font_family:'serif'})
 expect(await screen.findByText('订阅正文')).toBeInTheDocument()
 await waitFor(()=>expect(onRead).toHaveBeenCalledTimes(1))
 expect(screen.getByText('const value = 1;').tagName).toBe('CODE')
 expect(screen.getByText('表格').tagName).toBe('TD')
 expect(document.querySelector('.reader-content article')?.getAttribute('style')).toContain('ui-serif')
 await userEvent.click(screen.getByRole('link',{name:'正文链接'}));expect(api.open).toHaveBeenLastCalledWith('a-0',link)
 await selectMoreItem('打开原文');expect(api.open).toHaveBeenLastCalledWith('a-0','')
 expect(onRead).toHaveBeenCalledTimes(1)
})
it('取消提取保留订阅正文，迟到响应不能切换当前内容',async()=>{
 let release:(value:Detail)=>void=()=>{};let signal:AbortSignal|undefined
 api.extract.mockImplementation((_id:string,next:AbortSignal)=>{signal=next;return new Promise(resolve=>{release=resolve})})
 setup();await screen.findByText('订阅正文')
 await selectReaderMode('阅读模式')
 expect(screen.getByText('订阅正文')).toBeInTheDocument()
 await userEvent.click(screen.getByRole('button',{name:'取消提取'}));expect(signal?.aborted).toBe(true)
 await act(async()=>release(extracted))
 expect(screen.getByText('订阅正文')).toBeInTheDocument();expect(screen.queryByText('提取后的正文')).not.toBeInTheDocument()
 expect(screen.getByRole('alert')).toHaveTextContent('提取已取消')
})
it('提取成功缓存两种模式，往返切换不重复访问原网页',async()=>{
 api.extract.mockResolvedValue(extracted)
 setup();await screen.findByText('订阅正文')
 await selectReaderMode('阅读模式');expect(await screen.findByText('提取后的正文')).toBeInTheDocument()
 await selectReaderMode('订阅预览');expect(await screen.findByText('订阅正文')).toBeInTheDocument()
 await selectReaderMode('阅读模式');expect(await screen.findByText('提取后的正文')).toBeInTheDocument()
 expect(api.extract).toHaveBeenCalledTimes(1)
})

it('更多仅展示阅读方式入口，键盘可展开子菜单并切换唯一勾选项', async () => {
 const user = userEvent.setup()
 api.extract.mockResolvedValue(extracted)
 setup()
 await screen.findByText('订阅正文')
 const more = screen.getByRole('button', { name: '更多' })
 more.focus()
 await user.keyboard('{Enter}{ArrowDown}{ArrowDown}')
 const trigger = screen.getByRole('menuitem', { name: '阅读方式' })
 expect(trigger).toHaveFocus()
 expect(trigger).toHaveAttribute('aria-haspopup', 'menu')
 expect(screen.queryByRole('menuitemradio')).not.toBeInTheDocument()

 await user.keyboard('{ArrowRight}')
 expect(await screen.findByRole('menuitemradio', { name: '订阅预览', checked: true })).toHaveFocus()
 expect(screen.getByRole('menuitemradio', { name: '阅读模式', checked: false })).toBeInTheDocument()
 await user.keyboard('{ArrowDown}{Enter}')
 expect(await screen.findByText('提取后的正文')).toBeInTheDocument()
 await waitFor(() => expect(more).toHaveFocus())

 await selectMoreItem('阅读方式')
 expect(await screen.findByRole('menuitemradio', { name: '阅读模式', checked: true })).toBeInTheDocument()
 expect(screen.getByRole('menuitemradio', { name: '订阅预览', checked: false })).toBeInTheDocument()
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
 await selectReaderMode('阅读模式')
 api.get.mockResolvedValue({...feed,html:'<p>更新后的订阅正文</p>',text:'更新后的订阅正文'})
 rerender(renderTree(2));await screen.findByText('更新后的订阅正文')
 expect(signal?.aborted).toBe(true);expect(screen.getByRole('button',{name:'重新提取正文'})).toBeEnabled()
 await act(async()=>release(extracted))
 expect(screen.queryByText('提取后的正文')).not.toBeInTheDocument()
 expect(onRead).toHaveBeenCalledTimes(1)
})
it('菜单请求在Reader挂载前到达也只执行一次',async()=>{
 api.extract.mockResolvedValue(extracted);useWorkspace.setState({readerModeRequestID:'a-0'})
 setup();expect(await screen.findByText('提取后的正文')).toBeInTheDocument()
 expect(api.extract).toHaveBeenCalledTimes(1);expect(useWorkspace.getState().readerModeRequestID).toBeNull()
})

it('只有两个阅读标签页，键盘切换后仍可在浏览器打开原文',async()=>{
 api.extract.mockResolvedValue(extracted)
 setup({reader_toolbar_pinned:['view','original']})
 await screen.findByText('订阅正文')
 expect(screen.getAllByRole('tab')).toHaveLength(2)
 const preview=screen.getByRole('tab',{name:'订阅预览'})
 await userEvent.click(preview)
 expect(preview).toHaveAttribute('aria-selected','true')
 await userEvent.keyboard('{ArrowRight}{Enter}')
 expect(await screen.findByText('提取后的正文')).toBeInTheDocument()
 expect(screen.getByRole('tab',{name:'阅读模式'})).toHaveAttribute('aria-selected','true')
 expect(screen.getByRole('tabpanel')).toHaveAccessibleName('阅读模式')
 const external=screen.getByRole('button',{name:'在浏览器打开原文'})
 expect(external).toHaveTextContent(/^$/)
 await userEvent.click(external)
 expect(api.open).toHaveBeenCalledWith('a-0','')
})

it('全文默认阅读模式时自动提取，严格模式只发一次请求',async()=>{
 api.extract.mockResolvedValue(extracted)
 setup({reader_open_mode:'full'})
 expect(await screen.findByText('提取后的正文')).toBeInTheDocument()
 expect(api.extract).toHaveBeenCalledTimes(1)
})

it('默认常驻四项，更多中的分类对话框关闭后回到触发器', async () => {
 setup()
 await screen.findByText('订阅正文')
 const toolbar = document.querySelector('.reader-toolbar-actions')!
 expect(Array.from(toolbar.querySelectorAll('button'), button => button.getAttribute('aria-label'))).toEqual([
  '重新提取正文', '收藏当前文章', '翻译（暂未开放）', '文章 AI 助手', '更多',
 ])
 expect(screen.getByRole('button',{name:'翻译（暂未开放）'})).toBeDisabled()
 await selectMoreItem('归类')
 expect(await screen.findByRole('dialog',{name:'文章分类'})).toBeInTheDocument()
 await userEvent.keyboard('{Escape}')
 await waitFor(() => expect(screen.getByRole('button',{name:'更多'})).toHaveFocus())
})

it('已读操作显示执行后的状态，点击沿用同一篇文章的切换操作', async () => {
 const { onRead, rerender, renderTree } = setup({ reader_auto_read: false })
 await screen.findByText('订阅正文')
 await selectMoreItem('标为已读')
 expect(onRead).toHaveBeenLastCalledWith('a-0')

 rerender(renderTree(1, true))
 await selectMoreItem('标为未读')
 expect(onRead).toHaveBeenCalledTimes(2)
 expect(onRead).toHaveBeenLastCalledWith('a-0')
})
