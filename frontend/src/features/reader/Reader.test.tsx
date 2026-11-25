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
import type { TranslationRecord } from '@/lib/translation-api'
const api=vi.hoisted(()=>({get:vi.fn(),extract:vi.fn(),open:vi.fn()}))
const translationMock = vi.hoisted(() => ({ get: vi.fn(), translate: vi.fn(), cancel: vi.fn(), clear: vi.fn(), languages: vi.fn() }))
const translationPreferences = vi.hoisted(() => ({ translateOnOpen: false }))
const summaryMock = vi.hoisted(() => ({ get: vi.fn(), generate: vi.fn(), settings: { onOpen: false, hanLimit: 600, englishLimit: 350 } }))
vi.mock('../../../bindings/github.com/coolerks/Fealdhord/internal/articlesummary/service', () => ({
  Get: summaryMock.get, Generate: summaryMock.generate,
}))
const copyImage = vi.hoisted(() => vi.fn())
const translationEvents = vi.hoisted(() => new Set<(event: { data: { record: TranslationRecord } }) => void>())
vi.mock('@wailsio/runtime', async importOriginal => {
 const actual = await importOriginal<typeof import('@wailsio/runtime')>()
 return { ...actual, Events: {
  ...actual.Events,
  On(name: string, callback: (event: { data: { record: TranslationRecord } }) => void) {
   if (name === 'translation:progress') translationEvents.add(callback)
   return () => { translationEvents.delete(callback) }
  },
 } }
})
vi.mock('../../../bindings/github.com/coolerks/Fealdhord/internal/media/api', () => ({ CopyInlineImage: copyImage }))
vi.mock('../../../bindings/github.com/coolerks/Fealdhord/internal/articleai/api', () => ({
  Settings: vi.fn(async () => ({ classification: false, featured: false, prompt: '', revision: 1, requestsPerHour: 100, tokensPerHour: 200000, minimumImportance: 0.7, minimumRelevance: 0.6, autoTranslate: false, autoTranslateScope: 'summary', translateOnOpen: translationPreferences.translateOnOpen, translationTargetLang: 'zh-CN', translationPrompt: '', summary: { ...summaryMock.settings } })),
}))
vi.mock('@/lib/translation-api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/translation-api')>('@/lib/translation-api')
  return { ...actual, translationAPI: translationMock }
})
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
beforeEach(()=>{vi.resetAllMocks();summaryMock.settings.onOpen=false;api.get.mockResolvedValue(feed);api.open.mockResolvedValue(undefined);translationMock.get.mockResolvedValue({articleId:'a-0',targetLang:'zh-CN',sourceMode:'feed',contentHash:'',detectedLang:'',status:'none',translatedTitle:'',translatedSummary:'',blocks:[],translatedBlocks:0,totalBlocks:0,modelId:'',errorCode:'',updatedAt:0});translationMock.translate.mockResolvedValue({articleId:'a-0',targetLang:'zh-CN',sourceMode:'feed',contentHash:'',detectedLang:'en',status:'completed',translatedTitle:'译文标题',translatedSummary:'',blocks:[],translatedBlocks:0,totalBlocks:0,modelId:'m1',errorCode:'',updatedAt:1});useWorkspace.setState({preferences:preferencesFixture(),readerModeRequestID:null})})
function setup(patch:Parameters<typeof preferencesFixture>[0]={}){
 const prefs=preferencesFixture(patch),onRead=vi.fn(),article={...articleFixtures(1)[0],contentText:'',isRead:false,url:'https://example.test/article'}
 const renderTree=(revision=1,isRead=false,feedAutoTranslateMode?:string)=><StrictMode><QueryClientProvider client={client}><TooltipProvider><Reader article={{...article,revision,isRead}} preferences={prefs} feedAutoTranslateMode={feedAutoTranslateMode} overlay onClose={vi.fn()} onFavorite={vi.fn()} onRead={onRead} onAnchor={vi.fn()}/></TooltipProvider></QueryClientProvider></StrictMode>
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
 expect(screen.getByRole('status')).toHaveTextContent('正在提取网页正文')
 expect(screen.queryByText('订阅正文')).not.toBeInTheDocument()
 await userEvent.click(screen.getByRole('button',{name:'取消提取'}));expect(signal?.aborted).toBe(true)
 await act(async()=>release(extracted))
 expect(screen.getByText('订阅正文')).toBeInTheDocument();expect(screen.queryByText('提取后的正文')).not.toBeInTheDocument()
 expect(screen.getByRole('alert')).toHaveTextContent('提取已取消')
 expect(document.querySelector('[data-slot="skeleton"]')).not.toBeInTheDocument()
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

it('默认打开阅读模式时显示提取骨架屏，严格模式只发一次请求',async()=>{
 let complete!: (detail: Detail) => void
 api.extract.mockReturnValue(new Promise<Detail>(resolve => { complete = resolve }))
 const {container}=setup({reader_open_mode:'full'})
 expect(await screen.findByRole('status')).toHaveTextContent('正在提取网页正文')
 expect(container.querySelector('.reader-prose')).toHaveAttribute('aria-busy','true')
 expect(container.querySelector('[data-slot="skeleton"]')).toHaveClass('animate-pulse')
 expect(screen.queryByText('订阅正文')).not.toBeInTheDocument()
 complete(extracted)
 expect(await screen.findByText('提取后的正文')).toBeInTheDocument()
 expect(container.querySelector('[data-slot="skeleton"]')).not.toBeInTheDocument()
 expect(api.extract).toHaveBeenCalledTimes(1)
})

it('默认常驻五项，更多中的分类对话框关闭后回到触发器', async () => {
 setup()
 await screen.findByText('订阅正文')
 const toolbar = document.querySelector('.reader-toolbar-actions')!
 expect(Array.from(toolbar.querySelectorAll('button'), button => button.getAttribute('aria-label'))).toEqual([
  '重新提取正文', '收藏', '翻译', '生成摘要', 'AI 助手', '更多',
 ])
 expect(screen.getByRole('button',{name:'翻译'})).toBeEnabled()
 await selectMoreItem('归类')
 expect(await screen.findByRole('dialog',{name:'文章分类'})).toBeInTheDocument()
 await userEvent.keyboard('{Escape}')
 await waitFor(() => expect(screen.getByRole('button',{name:'更多'})).toHaveFocus())
})

it('支持段落行内双语对照、仅看译文和仅看原文切换，且不重复目录标题', async () => {
 const user = userEvent.setup()
 api.get.mockResolvedValue({
  ...feed,
  html: `<h2 data-tr-block="b0">Overview</h2><p data-tr-block="b1">Hello English paragraph.<img src="/media/inline/${imageIDs[0]}" alt="插图"></p><ul><li data-tr-block="b2"><em>List item</em></li></ul><table><tbody><tr><td data-tr-block="b3">Cell</td></tr></tbody></table><p data-tr-block="b4">Untranslated paragraph</p><pre><code>const keep = true;</code></pre>`,
 })
 translationMock.get.mockResolvedValue({
  articleId: 'a-0',
  targetLang: 'zh-CN',
  sourceMode: 'feed',
  contentHash: 'h1',
  detectedLang: 'en',
  status: 'partial',
  translatedTitle: '中文译文标题',
  translatedSummary: '',
  blocks: [
   { id: 'b0', sourceHash: 's0', translatedHtml: '概览' },
   { id: 'b1', sourceHash: 's1', translatedHtml: '你好英文段落。' },
   { id: 'b2', sourceHash: 's2', translatedHtml: '<em>列表项</em>' },
   { id: 'b3', sourceHash: 's3', translatedHtml: '单元格' },
  ],
  translatedBlocks: 4,
  totalBlocks: 5,
  modelId: 'm1',
  errorCode: '',
  updatedAt: 1,
 })
 setup({ reader_translation_display: 'bilingual', reader_outline: 'fixed' })

 expect(await screen.findByText('你好英文段落。')).toBeInTheDocument()
 expect(screen.getByText('Hello English paragraph.')).toBeInTheDocument()
 expect(screen.getByText('中文译文标题')).toBeInTheDocument()
 expect(document.querySelector('pre code')?.textContent).toContain('const keep = true;')
 expect(document.querySelectorAll('.reader-outline-panel li')).toHaveLength(1)
 const image = screen.getByRole('button', { name: '查看图片：插图' })

 await user.click(screen.getByRole('button', { name: '翻译' }))
 await user.click(await screen.findByRole('menuitemradio', { name: '仅看译文' }))
 await waitFor(() => expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('中文译文标题'))
 expect(screen.getByRole('heading', { name: '概览', level: 2 })).toBeVisible()
 expect(screen.getByText('你好英文段落。')).toBeVisible()
 expect(screen.getByText('Hello English paragraph.')).not.toBeVisible()
 expect(screen.getByText('List item')).not.toBeVisible()
 expect(screen.getByText('Cell')).not.toBeVisible()
 expect(screen.getByText('列表项').closest('li')).toBeVisible()
 expect(screen.getByRole('cell', { name: '单元格' })).toBeVisible()
 expect(screen.getByText('Untranslated paragraph')).toBeVisible()
 expect(screen.getByRole('button', { name: '查看图片：插图' })).toBe(image)

 await user.click(screen.getByRole('button', { name: '翻译' }))
 await user.click(await screen.findByRole('menuitemradio', { name: '仅看原文' }))
 await waitFor(() => expect(screen.queryByText('你好英文段落。')).not.toBeInTheDocument())
 expect(screen.getByText('Hello English paragraph.')).toBeVisible()
 expect(screen.getByText('List item')).toBeVisible()
 expect(screen.getByText('Cell')).toBeVisible()
 expect(translationMock.translate).not.toHaveBeenCalled()
 await user.click(screen.getByRole('button', { name: '翻译' }))
 await user.click(await screen.findByRole('menuitem', { name: '继续翻译' }))
 expect(translationMock.translate).toHaveBeenCalledExactlyOnceWith({ articleId: 'a-0', targetLang: 'zh-CN', sourceMode: 'feed', force: false })
})

it('目标语言可以键盘搜索，显示与语言选择只读缓存，只有正文动作启动翻译', async () => {
 const user = userEvent.setup()
 translationMock.get.mockImplementation(async (articleId: string, targetLang: string, sourceMode: string) => ({
  articleId, targetLang, sourceMode, status: 'none', translatedTitle: targetLang === 'zh-CN' ? '已有标题译文' : '', blocks: [],
 }))
 setup({ reader_translation_display: 'original' })
 await screen.findByText('订阅正文')
 const trigger = screen.getByRole('button', { name: '翻译' })
 await user.click(trigger)
 await user.click(await screen.findByRole('menuitemradio', { name: '仅看译文' }))
 expect(await screen.findByRole('heading', { name: '已有标题译文' })).toBeVisible()
 expect(translationMock.translate).not.toHaveBeenCalled()

 await user.click(trigger)
 expect(await screen.findByRole('menuitem', { name: '翻译正文' })).toBeInTheDocument()
 await user.click(await screen.findByRole('menuitem', { name: '目标语言（简体中文）' }))
 const dialog = await screen.findByRole('dialog', { name: '目标语言' })
 await user.click(within(dialog).getByRole('button', { name: '目标语言' }))
 const input = await screen.findByRole('combobox', { name: '搜索目标语言' })
 expect(input).toHaveFocus()
 await user.keyboard('french{Enter}')
 await waitFor(() => expect(translationMock.get).toHaveBeenCalledWith('a-0', 'fr', 'feed'))
 expect(translationMock.translate).not.toHaveBeenCalled()
 await user.keyboard('{Escape}')
 await waitFor(() => expect(trigger).toHaveFocus())

 await user.click(trigger)
 const action = await screen.findByRole('menuitem', { name: '翻译正文' })
 expect(action).not.toHaveAttribute('data-disabled')
 await user.click(action)
 expect(translationMock.translate).toHaveBeenCalledExactlyOnceWith({ articleId: 'a-0', targetLang: 'fr', sourceMode: 'feed', force: false })
})

it('订阅开启可独立于全局触发打开时翻译，并保留仅原文偏好', async () => {
 const { rerender, renderTree } = setup({ reader_translation_display: 'original' })
 await screen.findByText('订阅正文')
 expect(translationMock.translate).not.toHaveBeenCalled()
 rerender(renderTree(1, false, 'on'))
 await waitFor(() => expect(translationMock.translate).toHaveBeenCalledTimes(1))
 expect(screen.queryByText('译文标题')).not.toBeInTheDocument()
 await userEvent.click(screen.getByRole('button', { name: '翻译' }))
 expect(await screen.findByRole('menuitemradio', { name: '仅看原文', checked: true })).toBeInTheDocument()
})

it('未配置翻译模型时提示可关闭打开时自动翻译并提供对应设置入口', async () => {
 const user = userEvent.setup()
 translationMock.translate.mockRejectedValueOnce(new Error('ConfigurationError'))
 translationPreferences.translateOnOpen = true
 try {
  setup()
  const alert = await screen.findByRole('alert')
  expect(alert).toHaveTextContent('请先在「设置 → 服务与模型」的用途分配中为「翻译」选择已启用的模型；若不想看到此提示，可在「设置 → 文章 AI」关闭「打开文章时自动翻译正文」。')
  await user.click(screen.getByRole('button', { name: '配置模型' }))
  expect(useWorkspace.getState().settingsTab).toBe('ai')
  await user.click(screen.getByRole('button', { name: '文章 AI 设置' }))
  expect(useWorkspace.getState().settingsTab).toBe('article-ai')
 } finally {
  translationPreferences.translateOnOpen = false
 }
})

it('订阅关闭覆盖全局打开时翻译', async () => {
 translationPreferences.translateOnOpen = true
 try {
  const { rerender, renderTree } = setup()
  rerender(renderTree(1, false, 'off'))
  await screen.findByText('订阅正文')
  await waitFor(() => expect(translationMock.get).toHaveBeenCalled())
  expect(translationMock.translate).not.toHaveBeenCalled()
 } finally {
  translationPreferences.translateOnOpen = false
 }
})

it('摘要与目标语言一致时打开正文不启动翻译，仍可手动翻译', async () => {
 const user = userEvent.setup()
 translationMock.get.mockResolvedValue({ status: 'skipped_same_lang', blocks: [], updatedAt: 1 })
 translationPreferences.translateOnOpen = true
 try {
  setup()
  await screen.findByText('订阅正文')
  await user.click(screen.getByRole('button', { name: '翻译' }))
  const manual = await screen.findByRole('menuitem', { name: '仍要翻译' })
  expect(translationMock.translate).not.toHaveBeenCalled()
  expect(screen.queryByText('正在生成首批译文…')).not.toBeInTheDocument()
  await user.click(manual)
  expect(translationMock.translate).toHaveBeenCalledExactlyOnceWith({
   articleId: 'a-0', targetLang: 'zh-CN', sourceMode: 'feed', force: true,
  })
 } finally {
  translationPreferences.translateOnOpen = false
 }
})

it('长文优先发送阅读位置，旧版和乱序进度不会覆盖，已读位置随布局变化保持', async () => {
 const user = userEvent.setup()
 api.get.mockResolvedValue({ ...feed, source_version: 'current-source',
  html: '<p data-tr-block="b0">第一段原文</p><p data-tr-block="b1">当前阅读段落</p>' })
 const initial: TranslationRecord = { articleId: 'a-0', targetLang: 'zh-CN', sourceMode: 'feed',
  contentHash: 'article', sourceVersion: 'current-source', detectedLang: '', status: 'none',
  translatedTitle: '', translatedSummary: '', blocks: [], translatedBlocks: 0, totalBlocks: 66,
  modelId: '', errorCode: '', updatedAt: 0 }
 translationMock.get.mockResolvedValue(initial)
 let finish: (value: TranslationRecord) => void = () => undefined
 translationMock.translate.mockImplementation(() => new Promise<TranslationRecord>(resolve => { finish = resolve }))
 const { container } = setup()
 const current = await screen.findByText('当前阅读段落')
 const first = screen.getByText('第一段原文')
 const viewport = container.querySelector<HTMLElement>('.reader-content')!
 viewport.scrollTop = 100
 vi.spyOn(viewport, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 480, 200))
 vi.spyOn(first, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, -80, 480, 40))
 vi.spyOn(current, 'getBoundingClientRect').mockImplementation(() =>
  new DOMRect(0, 20 + (document.querySelector('.reader-tr-block') ? 40 : 0) - (viewport.scrollTop - 100), 480, 20))
 await user.click(screen.getByRole('button', { name: '翻译' }))
 await user.click(await screen.findByRole('menuitem', { name: '翻译正文' }))
 expect(translationMock.translate).toHaveBeenCalledExactlyOnceWith({
  articleId: 'a-0', targetLang: 'zh-CN', sourceMode: 'feed', sourceVersion: 'current-source', startBlockId: 'b1', force: false,
 })
 expect(screen.getByText('正在生成首批译文…')).toBeVisible()
 const publish = async (record: TranslationRecord) => {
  await act(async () => translationEvents.forEach(callback => callback({ data: { record } })))
 }
 const progress: TranslationRecord = { ...initial, status: 'translating', translatedBlocks: 1, updatedAt: 5,
  blocks: [{ id: 'b0', sourceHash: 'current-block', translatedHtml: '第一段译文' }] }
 await publish({ ...progress, sourceVersion: 'old-source', updatedAt: 10 })
 expect(screen.queryByText('第一段译文')).toBeNull()
 await publish(progress)
 expect(await screen.findByText('已翻译 1/66 段')).toBeVisible()
 const existingTranslation = screen.getByText('第一段译文')
 expect(viewport.scrollTop).toBe(140)
 await publish({ ...initial, status: 'translating', updatedAt: 4 })
 expect(screen.getByText('第一段译文')).toBe(existingTranslation)
 await act(async () => finish({ ...progress, status: 'partial', errorCode: 'InvalidModelOutput', updatedAt: 6 }))
 expect(screen.getByText('第一段译文')).toBe(existingTranslation)
 expect(screen.getByRole('alert')).toHaveTextContent('翻译格式无效')
 expect(screen.getByRole('button', { name: '继续翻译' })).toBeVisible()
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
 expect(proseScroll.closest('.reader-stage')).toHaveAttribute('inert')
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

it('放大后按住左键双轴拖动图片，松手后保持看图，右键打开菜单', async () => {
 api.get.mockResolvedValue(illustrated)
 setup()
 fireEvent.click(await screen.findByRole('button', { name: '查看图片：第一张' }))
 const viewer = screen.getByRole('region', { name: '看图模式' })
 const picture = within(viewer).getByRole('img', { name: '第一张' })
 const scroll = viewer.querySelector<HTMLDivElement>('.reader-image-scroll')!
 Object.defineProperties(scroll, { scrollWidth: { value: 900 }, scrollHeight: { value: 1200 } })
 fireEvent.wheel(picture, { deltaY: -40 })
 await waitFor(() => expect(scroll).toHaveAttribute('data-fitted', 'false'))

 scroll.scrollLeft = 80
 scroll.scrollTop = 90
 const left = { button: 0, pointerId: 1, clientX: 100, clientY: 100 }
 fireEvent.pointerDown(picture, left)
 expect(scroll).toHaveAttribute('data-dragging', 'true')
 fireEvent.pointerMove(scroll, { ...left, clientX: 130, clientY: 75 })
 expect(scroll.scrollLeft).toBe(50)
 expect(scroll.scrollTop).toBe(115)
 fireEvent.pointerUp(scroll, left)
 expect(scroll).not.toHaveAttribute('data-dragging')
 fireEvent.click(viewer.querySelector('.reader-image-canvas')!)
 expect(viewer).toBeInTheDocument()
 expect(screen.queryByRole('menu')).not.toBeInTheDocument()

 const right = { button: 2, pointerId: 2, clientX: 100, clientY: 100 }
 fireEvent.pointerDown(picture, right)
 expect(scroll).not.toHaveAttribute('data-dragging')
 fireEvent.contextMenu(picture, right)
 expect(await screen.findByRole('menu', { name: '看图操作' })).toBeInTheDocument()
})

it('左键拖动取消或失去捕获时清除抓手，空白处单击仍可退出', async () => {
 api.get.mockResolvedValue(illustrated)
 setup()
 fireEvent.click(await screen.findByRole('button', { name: '查看图片：第一张' }))
 const viewer = screen.getByRole('region', { name: '看图模式' })
 const picture = within(viewer).getByRole('img', { name: '第一张' })
 const scroll = viewer.querySelector<HTMLDivElement>('.reader-image-scroll')!
 Object.defineProperties(scroll, { scrollWidth: { value: 900 }, scrollHeight: { value: 1200 } })
 fireEvent.wheel(picture, { deltaY: -40 })
 await waitFor(() => expect(scroll).toHaveAttribute('data-fitted', 'false'))

 const left = { button: 0, pointerId: 1, clientX: 100, clientY: 100 }
 for (const finish of [fireEvent.pointerCancel, fireEvent.lostPointerCapture]) {
  fireEvent.pointerDown(picture, left)
  expect(scroll).toHaveAttribute('data-dragging', 'true')
  fireEvent.pointerMove(scroll, { ...left, clientX: 130, clientY: 75 })
  finish(scroll, left)
  expect(scroll).not.toHaveAttribute('data-dragging')
  const position = { left: scroll.scrollLeft, top: scroll.scrollTop }
  fireEvent.pointerMove(scroll, { ...left, clientX: 160, clientY: 50 })
  expect(scroll.scrollLeft).toBe(position.left)
  expect(scroll.scrollTop).toBe(position.top)
 }
 const canvas = viewer.querySelector('.reader-image-canvas')!
 fireEvent.pointerDown(canvas, left)
 fireEvent.pointerUp(canvas, left)
 fireEvent.click(canvas)
 expect(screen.queryByRole('region', { name: '看图模式' })).not.toBeInTheDocument()
})

it('捏合只改变图片比例，放大后双轴滚动保留给图片视口', async () => {
 api.get.mockResolvedValue(illustrated)
 setup({ reader_image_wheel: 'navigate' })
 fireEvent.click(await screen.findByRole('button', { name: '查看图片：第一张' }))
 const viewer = screen.getByRole('region', { name: '看图模式' })
 const scale = within(viewer).getByRole('slider', { name: '图片缩放' })
 const picture = within(viewer).getByRole('img', { name: '第一张' })
 const scroll = viewer.querySelector('.reader-image-scroll')!
 Object.defineProperties(scroll, { scrollWidth: { value: 900 }, scrollHeight: { value: 1200 } })
 const gesture = (type: string, scale?: number) =>
  fireEvent(picture, Object.assign(new Event(type, { bubbles: true, cancelable: true }), { scale }))

 expect(gesture('gesturestart')).toBe(false)
 expect(gesture('gesturechange', 2)).toBe(false)
 expect(scale).toHaveAttribute('aria-valuenow', '200')
 gesture('gesturechange', 1.5)
 gesture('gestureend')
 expect(scale).toHaveAttribute('aria-valuenow', '150')
 expect(fireEvent.wheel(picture, { deltaX: 18, deltaY: 25 })).toBe(true)
 expect(scale).toHaveAttribute('aria-valuenow', '150')

 expect(fireEvent.wheel(picture, { ctrlKey: true, deltaY: 20 })).toBe(false)
 await waitFor(() => expect(Number(scale.getAttribute('aria-valuenow'))).toBeLessThan(150))
 expect(picture).toBeInTheDocument()
})

it('图片超出视口时，连续像素滚轮仍能继续放大和缩小', async () => {
 api.get.mockResolvedValue(illustrated)
 setup()
 fireEvent.click(await screen.findByRole('button', { name: '查看图片：第一张' }))
 const viewer = screen.getByRole('region', { name: '看图模式' })
 const scale = within(viewer).getByRole('slider', { name: '图片缩放' })
 const picture = within(viewer).getByRole('img', { name: '第一张' })
 const scroll = viewer.querySelector('.reader-image-scroll')!
 Object.defineProperties(scroll, { scrollWidth: { value: 900 }, scrollHeight: { value: 1200 } })
 const mouseWheel = (deltaY: number) => {
  const event = new WheelEvent('wheel', { bubbles: true, cancelable: true, deltaY })
  Object.defineProperty(event, 'wheelDeltaY', { value: -Math.sign(deltaY) * 75 })
  expect(fireEvent(picture, event)).toBe(false)
 }
 const zoom = () => Number(scale.getAttribute('aria-valuenow'))

 mouseWheel(-40)
 await waitFor(() => expect(zoom()).toBeGreaterThan(106))
 expect(scroll).toHaveAttribute('data-fitted', 'false')
 mouseWheel(-40)
 await waitFor(() => expect(zoom()).toBeGreaterThan(113))
 mouseWheel(40)
 await waitFor(() => expect(zoom()).toBeLessThan(112))
})

it('AppKit 滑动只在溢出的图片上平移，普通滚轮和捏合分别缩放', async () => {
 const postMessage = vi.fn()
 const previous = Object.getOwnPropertyDescriptor(window, 'webkit')
 Object.defineProperty(window, 'webkit', {
  configurable: true, value: { messageHandlers: { fealdhordReaderTrackpad: { postMessage } } },
 })
 try {
  api.get.mockResolvedValue(illustrated)
  const { unmount } = setup()
  fireEvent.click(await screen.findByRole('button', { name: '查看图片：第一张' }))
  const viewer = screen.getByRole('region', { name: '看图模式' })
  const scale = within(viewer).getByRole('slider', { name: '图片缩放' })
  const picture = within(viewer).getByRole('img', { name: '第一张' })
  const scroll = viewer.querySelector('.reader-image-scroll')!
  Object.defineProperties(scroll, { scrollWidth: { value: 900 }, scrollHeight: { value: 1200 } })
  expect(postMessage).toHaveBeenLastCalledWith([0, 0, 0, 0])
  const pan = (window as Window & { fealdhordReaderPan?: (x: number, y: number, dx: number, dy: number) => void }).fealdhordReaderPan
  expect(pan).toBeTypeOf('function')
  Object.defineProperty(document, 'elementFromPoint', {
   configurable: true, value: (x: number) => x === 10 ? picture : document.body,
  })
  pan?.(10, 10, 18, 25)
  expect(scroll.scrollTop).toBe(0)
  expect(scale).toHaveAttribute('aria-valuenow', '100')

  fireEvent.wheel(picture, { deltaY: -40 })
  await waitFor(() => expect(scroll).toHaveAttribute('data-fitted', 'false'))
  await waitFor(() => expect(scale).toHaveAttribute('aria-valuenow', String(100 * Math.exp(40 * .0018))))
  scroll.scrollLeft = 200
  scroll.scrollTop = 200
  const before = Number(scale.getAttribute('aria-valuenow'))
  for (const [dx, dy] of [[0, 27], [18, 25], [-31, 12], [22, -28], [-9, -13]]) {
   const left = scroll.scrollLeft
   const top = scroll.scrollTop
   pan?.(10, 10, dx, dy)
   expect(scroll.scrollLeft).toBe(left + dx)
   expect(scroll.scrollTop).toBe(top + dy)
   expect(scale).toHaveAttribute('aria-valuenow', String(before))
  }
  const top = scroll.scrollTop
  pan?.(30, 10, 18, 25)
  expect(scroll.scrollTop).toBe(top)
  expect(fireEvent.wheel(picture, { deltaX: 18, deltaY: 25 })).toBe(false)
  await waitFor(() => expect(Number(scale.getAttribute('aria-valuenow'))).toBeLessThan(before))
  fireEvent(picture, new Event('gesturestart', { bubbles: true, cancelable: true }))
  fireEvent(picture, Object.assign(new Event('gesturechange', { bubbles: true, cancelable: true }), { scale: 2 }))
  expect(Number(scale.getAttribute('aria-valuenow'))).toBeGreaterThan(before)
  unmount()
  expect(postMessage).toHaveBeenLastCalledWith(null)
 } finally {
  Reflect.deleteProperty(document, 'elementFromPoint')
  if (previous) Object.defineProperty(window, 'webkit', previous)
  else Reflect.deleteProperty(window, 'webkit')
 }
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

it('看图隐藏阅读工具栏，缩放滑杆旁的关闭按钮播放退出动画并恢复正文焦点', async () => {
 api.get.mockResolvedValue(illustrated)
 const previousObserver = ResizeObserver
 const previousMotion = document.documentElement.dataset.motion
 const geometry = installGeometry()
 document.documentElement.dataset.motion = 'full'
 try {
  const {container}=setup()
  const source = await screen.findByRole('button', { name: '查看图片：第一张' })
  const toolbar = container.querySelector('.reader-toolbar')
  const viewport = container.querySelector<HTMLElement>('.reader-content')!
  viewport.getBoundingClientRect = () => new DOMRect(0, 50, 360, 550)
  viewport.scrollTop = 120
  source.getBoundingClientRect = () => new DOMRect(50, 160, 180, 120)
  fireEvent.click(source)
  const viewer = screen.getByRole('region', { name: '看图模式' })
  expect(toolbar).toHaveAttribute('inert')
  expect(toolbar).toHaveAttribute('aria-hidden', 'true')
  expect(screen.queryByRole('button', { name: '重新提取正文' })).not.toBeInTheDocument()
  const close = within(viewer).getByRole('button', { name: '退出看图' })
  const slider = within(viewer).getByRole('slider', { name: '图片缩放' })
  expect(slider.closest('.reader-image-scale')?.nextElementSibling).toBe(close)
  expect(viewer.parentElement).toBe(toolbar?.parentElement)
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
  slider.focus()
  await userEvent.tab()
  expect(close).toHaveFocus()
  await userEvent.keyboard('{Enter}')
  expect(viewer).toHaveAttribute('data-leaving', 'true')
  expect(close).toBeDisabled()
  expect(viewer).toBeInTheDocument()
  expect(toolbar).toHaveAttribute('inert')
  expect(animate.mock.calls[1][0][1]).toEqual({ transform: 'translate(30px, 60px) scale(0.5, 0.5)', borderRadius: '6px' })
  act(() => exit.onfinish?.())
  expect(viewer).not.toBeInTheDocument()
  expect(toolbar).not.toHaveAttribute('inert')
  expect(screen.getByRole('button', { name: '重新提取正文' })).toBeInTheDocument()
  expect(viewport.scrollTop).toBe(120)
  await waitFor(() => expect(source).toHaveFocus())
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

function summaryResponse(status: string, text = '') {
  return Object.assign(Promise.resolve({ status, text }), { cancel: vi.fn(async () => undefined) })
}

it.each([
  ['Timeout', '生成摘要超时'],
  ['ProviderConnectionError', '无法连接摘要模型服务'],
])('阅读器区分摘要错误 %s 并保留重试入口', async (code, message) => {
  const user = userEvent.setup()
  const document = { ...extracted, source_version: 'web-v1' }
  api.get.mockResolvedValue(document)
  summaryMock.get.mockImplementation(() => summaryResponse('none'))
  summaryMock.generate
    .mockImplementationOnce(() => Object.assign(Promise.reject(new Error(code)), { cancel: vi.fn() }))
    .mockImplementationOnce(() => summaryResponse('generated', '本次更新将在下月发布。'))
  setup({ reader_open_mode: 'full' })
  await screen.findByText('提取后的正文')

  await user.click(screen.getByRole('button', { name: '生成摘要' }))
  expect(await screen.findByRole('alert')).toHaveTextContent(message)
  expect(screen.getByText('提取后的正文')).toBeVisible()
  await user.click(screen.getByRole('button', { name: '重试摘要' }))
  expect(await screen.findByText('本次更新将在下月发布。')).toBeVisible()
})

it('手动摘要等待阅读模式正文，生成中可取消并支持键盘折叠与复制', async () => {
  const user = userEvent.setup()
  let finishSummary!: (value: { status: string; text: string }) => void
  const cancelSummary = vi.fn(async () => undefined)
  summaryMock.get.mockImplementation(() => summaryResponse('none'))
  summaryMock.generate.mockImplementation(() => Object.assign(
    new Promise<{ status: string; text: string }>(resolve => { finishSummary = resolve }),
    { cancel: cancelSummary },
  ))
  const copy = vi.spyOn(native, 'copy').mockResolvedValue(undefined)
  const document = { ...extracted, source_version: 'web-v1' }
  api.extract.mockResolvedValue(document)
  api.get.mockImplementation(async (_id: string, mode: string) => mode === 'reader' ? document : feed)
  try {
    const { container } = setup({ reader_open_mode: 'feed' })
    await screen.findByText('订阅正文')
    expect(summaryMock.generate).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: '生成摘要' }))
    await waitFor(() => expect(screen.getByText('提取后的正文')).toBeVisible())
    await waitFor(() => expect(summaryMock.generate).toHaveBeenCalled())
    expect(container.querySelector('.reader-summary[data-state="pending"]')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('正在生成文章摘要…')

    await act(async () => finishSummary({ status: 'generated', text: '- 下个月发布。\n- 新增导读卡样式。' }))
    expect(await screen.findByText('下个月发布。')).toBeVisible()
    expect(screen.getByText('新增导读卡样式。').closest('li')).toBeInTheDocument()
    expect(summaryMock.generate).toHaveBeenCalledExactlyOnceWith({
      source: { articleId: 'a-0', mode: 'reader', version: 'web-v1' }, automatic: false,
    })

    await user.click(screen.getByRole('button', { name: '复制摘要' }))
    expect(copy).toHaveBeenCalledExactlyOnceWith('- 下个月发布。\n- 新增导读卡样式。')
    expect(await screen.findByRole('status')).toHaveTextContent('摘要已复制')

    const toggle = screen.getByRole('button', { name: '摘要' })
    toggle.focus()
    await user.keyboard('{Enter}')
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getByText('下个月发布。')).not.toBeVisible()
  } finally {
    copy.mockRestore()
  }
})

it('开启自动摘要后跳过订阅预览，重开同一正文复用已缓存摘要', async () => {
  summaryMock.settings.onOpen = true
  summaryMock.get.mockImplementation(() => summaryResponse('none'))
  summaryMock.generate.mockImplementation(() => summaryResponse('generated', '本次更新将在下月发布。'))
  const document = { ...extracted, source_version: 'web-v1' }
  api.extract.mockResolvedValue(document)
  api.get.mockImplementation(async (_id: string, mode: string) => mode === 'reader' ? document : feed)
  const first = setup({ reader_open_mode: 'feed' })
  await screen.findByText('订阅正文')
  expect(summaryMock.generate).not.toHaveBeenCalled()
  await selectReaderMode('阅读模式')
  await waitFor(() => expect(summaryMock.get).toHaveBeenCalled())
  await waitFor(() => expect(summaryMock.generate).toHaveBeenCalled())
  expect(await screen.findByText('本次更新将在下月发布。')).toBeVisible()
  expect(summaryMock.generate).toHaveBeenCalledExactlyOnceWith({
    source: { articleId: 'a-0', mode: 'reader', version: 'web-v1' }, automatic: true,
  })
  first.unmount()

  summaryMock.get.mockImplementation(() => summaryResponse('generated', '本次更新将在下月发布。'))
  api.get.mockImplementation(async (_id: string, mode: string) => mode === 'reader'
    ? { ...extracted, source_version: 'web-v1' }
    : { ...feed, has_cached: true })
  setup({ reader_open_mode: 'full' })
  expect(await screen.findByText('本次更新将在下月发布。')).toBeVisible()
  expect(summaryMock.generate).toHaveBeenCalledTimes(1)
})
