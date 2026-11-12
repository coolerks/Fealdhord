import { StrictMode } from 'react'
import { readdirSync } from 'node:fs'
import { QueryClient,QueryClientProvider } from '@tanstack/react-query'
import { act,fireEvent,render,screen,waitFor,within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach,expect,it,vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { preferencesFixture,articleFixtures } from '@/test/fixtures'
import { installGeometry } from '@/test/geometry'
import { useWorkspace } from '@/stores/workspace'
import { native } from '@/lib/native'
import type { Detail } from '@/lib/reader-api'
const api=vi.hoisted(()=>({get:vi.fn(),extract:vi.fn(),open:vi.fn()}))
const copyImage = vi.hoisted(() => vi.fn())
vi.mock('../../../bindings/github.com/coolerks/Fealdhord/internal/media/api', () => ({ CopyInlineImage: copyImage }))
vi.mock('@/lib/reader-api',()=>({readerAPI:api}))
vi.mock('@/lib/preferences',()=>({preferencesAPI:{get:vi.fn(),update:vi.fn()}}))
vi.mock('@/lib/navigation-api',()=>({navigationQueryKey:['navigation'],navigationAPI:{list:vi.fn(async()=>({categories:[],feeds:[],totals:{total:0,unread:0,favorites:0}})),memberships:async()=>[]}}))
import { Reader } from './Reader'
import { codeThemeOptions } from './code-themes'
const link='a'.repeat(64)
const feed:Detail={article_id:'a-0',mode:'feed',html:`<p>订阅正文 <a href="#" data-reader-link="${link}">正文链接</a></p><pre><code>const value = 1;</code></pre><table><tbody><tr><td>表格</td></tr></tbody></table>`,text:'订阅正文',has_cached:false,has_original:true}
const extracted:Detail={...feed,mode:'reader',html:'<p>提取后的正文</p>',text:'提取后的正文',has_cached:true}
const imageIDs = ['d'.repeat(64), 'e'.repeat(64)]
const illustrated: Detail = { ...feed, html: `<h2>章节</h2><img src="/media/inline/${imageIDs[0]}" alt="第一张"><h6>小节</h6><img src="/media/inline/${imageIDs[1]}" alt="第二张">` }
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

it('主题目录包含已安装版本的每个独立 CSS 主题', () => {
 const files = readdirSync('node_modules/highlight.js/styles', { recursive: true, encoding: 'utf8' })
 const themes = files.filter(file => file.endsWith('.css') && !file.endsWith('.min.css'))
  .map(file => file.replaceAll('\\', '/').slice(0, -4)).sort()
 expect(codeThemeOptions.map(option => option.value)).toEqual(['app', ...themes])
})

it('正文显示代码语言，复制保留缩进和换行，所选主题只作用于阅读正文', async () => {
 const user = userEvent.setup()
 api.get.mockResolvedValue({
  ...feed,
  html: '<pre><code class="language-powershell">  Get-ChildItem<br>$env:PATH</code></pre><pre>hello</pre>',
 })
 const copy = vi.spyOn(native, 'copy').mockResolvedValue(undefined)
 try {
  const { container, unmount } = setup({ reader_code_theme: 'github-dark' })
  const language = await screen.findByText('PowerShell')
  const block = language.closest('.reader-code-block')!
  expect(block.querySelector('[data-highlighted=yes]')).toBeInTheDocument()
  expect(screen.getByText('纯文本')).toBeInTheDocument()
  await user.click(within(block as HTMLElement).getByRole('button', { name: '复制代码' }))
  expect(copy).toHaveBeenCalledExactlyOnceWith('  Get-ChildItem\n$env:PATH')
  expect(within(block as HTMLElement).getByRole('status')).toHaveTextContent('代码已复制')

  await waitFor(() => expect(container.querySelector('.reader-html')).toHaveAttribute('data-code-theme', 'github-dark'))
  const style = document.querySelector<HTMLStyleElement>('style[data-reader-code-theme=github-dark]')!
  const rules = Array.from(style.sheet!.cssRules)
  expect(rules.length).toBeGreaterThan(0)
  expect(rules.every(rule => rule.cssText.startsWith('.reader-html[data-code-theme="github-dark"]'))).toBe(true)
  unmount()
  expect(style).not.toBeInTheDocument()
 } finally {
  copy.mockRestore()
 }
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
 const external=screen.getByRole('button',{name:'打开原文'})
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
  '重新提取正文', '收藏', '翻译（暂未开放）', 'AI 助手', '更多',
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

it('打开看图后可以连续缩放、切图、复制和按 Esc 回到正文位置', async () => {
 const user = userEvent.setup()
 api.get.mockResolvedValue(illustrated)
 copyImage.mockResolvedValue({ ok: true })
 const { container } = setup()
 const first = await screen.findByRole('button', { name: '查看图片：第一张' })
 const proseScroll = container.querySelector('.reader-content')!
 proseScroll.scrollTop = 120
 await user.click(first)
 const viewer = screen.getByRole('region', { name: '看图模式' })
 expect(viewer).toHaveFocus()
 expect(proseScroll).toHaveAttribute('inert')
 const scale = within(viewer).getByRole('slider', { name: '图片缩放' })
 fireEvent.wheel(viewer, { deltaY: -36 })
 expect(scale).toHaveAttribute('aria-valuenow', '100')
 await waitFor(() => expect(Number(scale.getAttribute('aria-valuenow'))).toBeGreaterThan(100))
 expect(Number(scale.getAttribute('aria-valuenow'))).toBeLessThan(110)

 fireEvent.contextMenu(viewer)
 const menu = await screen.findByRole('menu', { name: '看图操作' })
 expect(within(menu).getByRole('menuitem', { name: '上一张' })).toHaveAttribute('aria-disabled', 'true')
 await user.click(within(menu).getByRole('menuitem', { name: '下一张' }))
 expect(within(viewer).getByRole('img', { name: '第二张' })).toBeInTheDocument()
 expect(scale).toHaveAttribute('aria-valuenow', '100')
 fireEvent.contextMenu(viewer)
 await user.click(await screen.findByRole('menuitem', { name: '复制图片' }))
 expect(copyImage).toHaveBeenCalledWith(imageIDs[1])
 await waitFor(() => expect(viewer).toHaveFocus())
 await user.keyboard('{Escape}')
 expect(screen.queryByRole('region', { name: '看图模式' })).not.toBeInTheDocument()
 expect(proseScroll.scrollTop).toBe(120)
 await waitFor(() => expect(first).toHaveFocus())

 await user.keyboard('{Enter}')
 fireEvent.contextMenu(screen.getByRole('region', { name: '看图模式' }))
 expect(await screen.findByRole('menu', { name: '看图操作' })).toBeInTheDocument()
 await user.keyboard('{Escape}')
 expect(screen.queryByRole('region', { name: '看图模式' })).not.toBeInTheDocument()
})

it('看图放大到 114% 时，滚动条显隐不改变图片尺寸', async () => {
 api.get.mockResolvedValue(illustrated)
 const previousObserver = ResizeObserver
 const geometry = installGeometry()
 try {
  const { unmount } = setup()
  fireEvent.click(await screen.findByRole('button', { name: '查看图片：第一张' }))
  const viewer = screen.getByRole('region', { name: '看图模式' })
  const picture = within(viewer).getByRole('img', { name: '第一张' })
  const scroll = viewer.querySelector('.reader-image-scroll')!
  let availableWidth = 360
  let availableHeight = 600
  Object.defineProperties(scroll, {
   clientWidth: { get: () => availableWidth },
   clientHeight: { get: () => availableHeight },
  })
  Object.defineProperties(picture, { naturalWidth: { value: 360 }, naturalHeight: { value: 540 } })
  fireEvent.load(picture)
  await waitFor(() => expect(picture).toHaveStyle({ width: '360px', height: '540px' }))
  fireEvent.wheel(viewer, { deltaY: -Math.log(1.14) / .0018 })
  await waitFor(() => expect(parseFloat(picture.style.width)).toBeCloseTo(410.4))
  const fittedSize = { width: picture.style.width, height: picture.style.height }
  for (const scrollbar of [true, false, true]) {
   availableWidth = scrollbar ? 352 : 360
   availableHeight = scrollbar ? 592 : 600
   act(() => geometry.resize())
   expect(picture).toHaveStyle(fittedSize)
  }
  unmount()
 } finally {
  geometry.cleanup()
  vi.restoreAllMocks()
  vi.stubGlobal('ResizeObserver', previousObserver)
 }
})

it('滚轮切图偏好保持缩放比例为默认值，禁止远程图时无法进入看图', async () => {
 api.get.mockResolvedValue(illustrated)
 const first = setup({ reader_image_wheel: 'navigate' })
 await userEvent.click(await screen.findByRole('button', { name: '查看图片：第一张' }))
 const viewer = screen.getByRole('region', { name: '看图模式' })
 fireEvent.wheel(viewer, { deltaY: 60 })
 expect(within(viewer).getByRole('img', { name: '第二张' })).toBeInTheDocument()
 expect(within(viewer).getByRole('slider')).toHaveAttribute('aria-valuenow', '100')
 first.unmount()

 const blocked = setup({ remote_images: 'never' })
 await screen.findByRole('heading', { name: '章节' })
 fireEvent.click(blocked.container.querySelector('.reader-prose img')!)
 expect(screen.queryByRole('region', { name: '看图模式' })).not.toBeInTheDocument()
})

it('切图等下一张就绪后按方向平推，动画结束才移除旧图', async () => {
 api.get.mockResolvedValue(illustrated)
 const root = document.documentElement
 const previousMotion = root.dataset.motion
 root.dataset.motion = 'full'
 try {
  setup()
  fireEvent.click(await screen.findByRole('button', { name: '查看图片：第一张' }))
  const viewer = screen.getByRole('region', { name: '看图模式' })
  Object.defineProperty(viewer, 'animate', { value: vi.fn() })
  const controls = viewer.querySelector('.reader-image-controls')

  for (const [current, next, key, direction] of [[0, 1, 'ArrowRight', 1], [1, 0, 'ArrowLeft', -1]] as const) {
   fireEvent.keyDown(viewer, { key })
   const outgoing = viewer.querySelector(`[data-image-index="${current}"]`)!
   const incoming = viewer.querySelector(`[data-image-index="${next}"]`)!
   const exit = vi.fn(() => ({ cancel: vi.fn() }))
   const enter = { onfinish: null as (() => void) | null, cancel: vi.fn() }
   const animate = vi.fn(() => enter)
   Object.defineProperty(outgoing, 'animate', { value: exit, configurable: true })
   Object.defineProperty(incoming, 'animate', { value: animate, configurable: true })
   expect(outgoing).toBeInTheDocument()
   expect(incoming).toHaveAttribute('inert')
   expect(exit).not.toHaveBeenCalled()

   fireEvent.load(incoming.querySelector('img')!)
   expect(exit).toHaveBeenCalledWith([
    { transform: 'translateX(0)' }, { transform: `translateX(${-100 * direction}%)` },
   ], expect.objectContaining({ duration: 220 }))
   expect(animate).toHaveBeenCalledWith([
    { transform: `translateX(${100 * direction}%)` }, { transform: 'translateX(0)' },
   ], expect.objectContaining({ duration: 220 }))
   expect(outgoing).toBeInTheDocument()
   act(() => enter.onfinish?.())
   expect(outgoing).not.toBeInTheDocument()
   expect(incoming).not.toHaveAttribute('inert')
   expect(viewer.querySelector('.reader-image-controls')).toBe(controls)
  }
 } finally {
  if (previousMotion === undefined) delete root.dataset.motion
  else root.dataset.motion = previousMotion
 }
})

it('看图从正文图片位置和尺寸开始，退出动画结束后才还原正文', async () => {
 api.get.mockResolvedValue(illustrated)
 const previousObserver = ResizeObserver
 const previousMotion = document.documentElement.dataset.motion
 const geometry = installGeometry()
 document.documentElement.dataset.motion = 'full'
 try {
  setup()
  const source = await screen.findByRole('button', { name: '查看图片：第一张' })
  source.getBoundingClientRect = () => new DOMRect(50, 160, 180, 120)
  fireEvent.click(source)
  const viewer = screen.getByRole('region', { name: '看图模式' })
  const picture = within(viewer).getByRole('img', { name: '第一张' })
  const cancel = vi.fn()
  const cancelReturn = vi.fn()
  const animate = vi.fn().mockReturnValueOnce({ cancel }).mockReturnValueOnce({ cancel: cancelReturn })
  Object.defineProperties(picture, {
   animate: { value: animate },
   naturalWidth: { value: 720 },
   naturalHeight: { value: 480 },
  })
  picture.getBoundingClientRect = () => new DOMRect(20, 100, 360, 240)
  expect(viewer).toHaveAttribute('data-entered', 'false')
  fireEvent.load(picture)
  await waitFor(() => expect(animate).toHaveBeenCalledExactlyOnceWith([
   { transform: 'translate(30px, 60px) scale(0.5, 0.5)', borderRadius: '6px' },
   { transform: 'translate(0, 0) scale(1)', borderRadius: '0px' },
  ], expect.objectContaining({ duration: 280 })))
  expect(viewer).toHaveAttribute('data-entered', 'true')
  expect(source).toHaveClass('reader-image-source-hidden')
  const exit = { onfinish: null as (() => void) | null, cancel: vi.fn() }
  Object.defineProperty(viewer, 'animate', { value: vi.fn(() => exit) })
  viewer.getBoundingClientRect = () => new DOMRect(0, 0, 360, 600)
  fireEvent.keyDown(viewer, { key: 'Escape' })
  expect(viewer).toHaveAttribute('data-leaving', 'true')
  expect(viewer).toBeInTheDocument()
  expect(animate.mock.calls[1][0][1]).toEqual({ transform: 'translate(30px, 60px) scale(0.5, 0.5)', borderRadius: '6px' })
  act(() => exit.onfinish?.())
  expect(viewer).not.toBeInTheDocument()
  expect(cancel).toHaveBeenCalledOnce()
  expect(cancelReturn).toHaveBeenCalledOnce()
  expect(source).not.toHaveClass('reader-image-source-hidden')
  expect(source).not.toHaveClass('reader-image-source-returning')
 } finally {
  geometry.cleanup()
  vi.restoreAllMocks()
  vi.stubGlobal('ResizeObserver', previousObserver)
  if (previousMotion === undefined) delete document.documentElement.dataset.motion
  else document.documentElement.dataset.motion = previousMotion
 }
})

it.each([true, false])('仅在启用空白点击退出时关闭看图（%s）', async backdropClose => {
 api.get.mockResolvedValue(illustrated)
 const user = userEvent.setup()
 setup({ reader_image_backdrop_close: backdropClose })
 await user.click(await screen.findByRole('button', { name: '查看图片：第一张' }))
 const viewer = screen.getByRole('region', { name: '看图模式' })
 const picture = within(viewer).getByRole('img', { name: '第一张' })
 await user.click(picture)
 expect(viewer).toBeInTheDocument()
 await user.click(picture.parentElement!)
 expect(Boolean(screen.queryByRole('region', { name: '看图模式' }))).toBe(!backdropClose)
})

it('标题目录只收集正文 H1–H6，悬停展开并跳转到对应标题', async () => {
 api.get.mockResolvedValue(illustrated)
 const user = userEvent.setup()
 const { container } = setup()
 const trigger = await screen.findByRole('button', { name: '文章目录' })
 const scroll = vi.fn()
 Object.defineProperty(container.querySelector('.reader-content'), 'scrollTo', { value: scroll })
 expect(trigger).toHaveAttribute('aria-expanded', 'false')
 await user.hover(trigger)
 expect(trigger).toHaveAttribute('aria-expanded', 'true')
 const outline = screen.getByRole('navigation', { name: '文章目录' })
 const options = within(outline).getAllByRole('button')
 expect(options.map(option => option.textContent)).toEqual(['章节H2', '小节H6'])
 expect(options[1].querySelector('kbd')).toHaveTextContent('H6')
 await user.click(options[1])
 expect(scroll).toHaveBeenCalledWith({ top: 0, behavior: 'instant' })
 expect(screen.getByRole('heading', { name: '小节' })).toHaveFocus()
})

it.each([
 { motion: 'full', target: 320, first: undefined },
 { motion: 'full', target: 3000, first: 2550 },
 { motion: 'off', target: 3000, first: 3000 },
])('目录跳转按距离和动效设置滚动到 $target（$motion）', async ({ motion, target, first }) => {
 api.get.mockResolvedValue(illustrated)
 const root = document.documentElement
 const previous = root.dataset.motion
 root.dataset.motion = motion
 try {
  const { container } = setup()
  const heading = await screen.findByRole('heading', { name: '小节' })
  const viewport = container.querySelector('.reader-content')!
  const scrollTo = vi.fn<(options: ScrollToOptions) => void>()
  Object.defineProperties(viewport, {
   scrollHeight: { value: 5000 },
   clientHeight: { value: 500 },
   scrollTo: { value: scrollTo },
  })
  heading.getBoundingClientRect = () => new DOMRect(0, target + 20, 120, 24)
  const trigger = await screen.findByRole('button', { name: '文章目录' })
  fireEvent.click(trigger)
  fireEvent.click(within(screen.getByRole('navigation', { name: '文章目录' })).getByRole('button', { name: '小节H6' }))

  if (first !== undefined) expect(scrollTo.mock.calls[0]?.[0].top).toBe(first)
  await waitFor(() => expect(scrollTo).toHaveBeenLastCalledWith({ top: target, behavior: 'instant' }))
  if (motion === 'off') expect(scrollTo).toHaveBeenCalledTimes(1)
  else {
   const initial = scrollTo.mock.calls[0]?.[0].top ?? -1
   expect(initial).toBeGreaterThanOrEqual(0)
   expect(initial).toBeLessThan(target)
   expect(scrollTo.mock.calls.some(([options]) => options.top! > initial && options.top! < target)).toBe(true)
  }
 } finally {
  if (previous === undefined) delete root.dataset.motion
  else root.dataset.motion = previous
 }
})
