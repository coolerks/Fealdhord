import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { QueryClient } from '@tanstack/react-query'
import { articleFixtures, preferencesFixture } from '@/test/fixtures'
import { installGeometry } from '@/test/geometry'
import { useWorkspace } from '@/stores/workspace'
import { initialLibraryData,libraryArticles } from '@/lib/library-query'
import type { LibraryData } from '@/lib/library-query'
import { articleQueryKey } from '@/lib/library'
import type { LibraryPage } from '@/lib/library'

const host = vi.hoisted(()=>({isMac:vi.fn(()=>false),isWindows:vi.fn(()=>false),isMaximized:vi.fn(async()=>false),onMaximizeChange:vi.fn<(_listener: (value: boolean) => void) => () => void>(()=>()=>{}),onCommand:vi.fn(()=>()=>{}),copy:vi.fn(async()=>{}),paste:vi.fn(async()=>'粘贴文本'),close:vi.fn(async()=>{}),minimize:vi.fn(async()=>{}),maximize:vi.fn(async()=>{}),fullscreen:vi.fn(),openURL:vi.fn(async()=>{}),openDevTools:vi.fn(async()=>{}),quit:vi.fn()}))
vi.mock('@/lib/native',()=>({native:host}))
const authorize = vi.hoisted(() => vi.fn<() => Promise<boolean>>())
vi.mock('../bindings/github.com/coolerks/Fealdhord/internal/desktop/notifier', () => ({ Authorize: authorize, Send: vi.fn() }))
const api = vi.hoisted(() => ({ get: vi.fn(), update: vi.fn() }))
vi.mock('@/lib/preferences', () => ({ preferencesAPI: api }))
const library=vi.hoisted(()=>({list:vi.fn<() => Promise<LibraryPage>>(async()=>({items:[],hasMore:false})),text:vi.fn(async()=>''),setRead:vi.fn(async()=>{}),setFavorite:vi.fn(async()=>{})}))
vi.mock('@/lib/library-api',()=>({libraryAPI:library}))
vi.mock('@/lib/api', () => ({ desktopAPI: { info: vi.fn(async () => ({ name: '折栈 Fealdhord', version: '0.1.0-alpha.15', platform: 'windows', architecture: 'amd64', wails_version: '3.0.0' })), subscribe: vi.fn(() => () => {}), start: vi.fn(), cancel: vi.fn(), snapshot: vi.fn() } }))
vi.mock('@/lib/rss-api',()=>({feedsQueryKey:['rss','feeds'],rssAPI:{list:vi.fn(async()=>({items:[],categories:[]})),current:vi.fn(async()=>({generation:0,operation_id:'',status:'completed',completed:0,total:0,sequence:0,results:[]})),subscribeRefresh:vi.fn(()=>()=>{}),subscribeLibrary:vi.fn(()=>()=>{})}}))
vi.mock('@/lib/navigation-api',()=>({navigationQueryKey:['navigation'],navigationAPI:{list:vi.fn(async()=>({categories:[],feeds:[],totals:{total:0,unread:0,favorites:0}}))}}))
const reader=vi.hoisted(()=>({get:vi.fn(),extract:vi.fn(),open:vi.fn()}))
vi.mock('@/lib/reader-api',()=>({readerAPI:reader}))
const copyImage = vi.hoisted(() => vi.fn())
vi.mock('../bindings/github.com/coolerks/Fealdhord/internal/media/api', () => ({ CopyInlineImage: copyImage }))
import App from './App'

let geometry: ReturnType<typeof installGeometry>
beforeEach(() => {
  vi.clearAllMocks()
  host.isMac.mockReturnValue(false)
  host.isWindows.mockReturnValue(false)
  geometry = installGeometry()
  useWorkspace.setState({ preferences: null, revision: 0, version: 0, savedVersion: 0, saveError: null, saveStatus: 'saved', search: '', sort: 'newest',readFilter:'all',favoriteFilter:'all', settingsOpen: false, settingsTab: 'appearance', paletteOpen: false, searchOpen: false, aiOpen: false, helpOpen:false,diagnosticsOpen:false,commandError:null })
  reader.get.mockResolvedValue({article_id:'a-0',html:'',text:'',mode:'feed',has_cached:false,has_original:false})
  api.get.mockResolvedValue({ preferences: preferencesFixture(), revision: 0 })
  api.update.mockImplementation(async (preferences, revision) => ({ preferences, revision: revision + 1 }))
})
afterEach(() => { geometry.cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals() })

function setup(withArticles = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(articleQueryKey, initialLibraryData({items:withArticles ? articleFixtures(20) : [],hasMore:false}))
  return { client, ...render(<App client={client} />) }
}

describe('Phase 1 阅读 Shell', () => {
  it('精简空态保留导航与双视图的可访问操作，不混入示例文章', async () => {
    setup()
    expect(await screen.findByRole('heading', { name: '全部文章' })).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: '阅读视图' })).toBeInTheDocument()
    expect(screen.getByRole('complementary',{name:'阅读面板'})).toHaveTextContent('选择文章')
    expect(screen.getByRole('heading',{name:'订阅'})).toBeInTheDocument()
    expect(screen.getByRole('button',{name:'管理订阅'})).toBeInTheDocument()
    expect(screen.queryByText('测试文章 0')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('radio', { name: '瀑布流视图' }))
    expect(screen.getByRole('radio',{name:'瀑布流视图'})).toHaveAttribute('aria-checked','true')
    expect(screen.queryByText('一目了然，快速浏览')).not.toBeInTheDocument()
    expect(screen.queryByText('本地优先，安心阅读')).not.toBeInTheDocument()
    expect(api.update).toHaveBeenCalledWith(expect.objectContaining({ article_view_mode: 'masonry' }), expect.any(Number))
  })

  it('视图切换保留选择与共享收藏状态，搜索不会标记文章已读', async () => {
    api.get.mockResolvedValue({preferences:preferencesFixture({reader_layout:'split'}),revision:0})
    const { client } = setup(true)
    await screen.findByRole('button', { name: '打开文章：测试文章 0' })
    await userEvent.click(screen.getByRole('button', { name: '打开文章：测试文章 0' }))
    expect(screen.getByRole('heading', { name: '测试文章 0' })).toBeInTheDocument()
    fireEvent.keyDown(window, { key: 'f' })
    expect(libraryArticles(client.getQueryData<LibraryData>(articleQueryKey))[0].isFavorite).toBe(false)
    await userEvent.click(screen.getByRole('radio', { name: '瀑布流视图' }))
    await act(async () => { geometry.resize() })
    expect(useWorkspace.getState().preferences?.selected_article_id).toBe('a-0')
    expect(screen.getByRole('heading', { name: '测试文章 0' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '搜索文章' }))
    const search = screen.getByRole('textbox', { name: '搜索全部文章' })
    await userEvent.type(search, '不存在')
    expect(await screen.findByText('没有找到匹配文章')).toBeInTheDocument()
    fireEvent.keyDown(search, { key: 'm' })
    expect(libraryArticles(client.getQueryData<LibraryData>(articleQueryKey))[0].isRead).toBe(true)
  })

  it('主题和动画即时生效，设置关闭后恢复触发点焦点', async () => {
    setup()
    const trigger = await screen.findByRole('button', { name: '打开设置' })
    await userEvent.click(trigger)
    await screen.findByRole('dialog', { name: '设置' })
    await userEvent.click(within(screen.getByRole('radiogroup', { name: '主题' })).getByRole('radio', { name: '深色' }))
    expect(document.documentElement).toHaveClass('dark')
    await userEvent.click(within(screen.getByRole('radiogroup', { name: '动画' })).getByRole('radio', { name: '关闭' }))
    expect(document.documentElement.dataset.motion).toBe('off')
    await userEvent.click(screen.getByRole('button', { name: /^关闭$/ }))
    await waitFor(() => { expect(trigger).toHaveFocus() })
  })

  it('阅读排版滑块具备名称，并按原步长响应方向键', async () => {
    setup()
    await userEvent.click(await screen.findByRole('button', { name: '打开设置' }))
    await userEvent.click(screen.getByRole('button', { name: '阅读排版' }))
    for (const [label, previous, next] of [
      ['正文字号', '17', '18'], ['行高', '1.7', '1.8'], ['正文宽度', '720', '740'],
    ]) {
      const slider = screen.getByRole('slider', { name: label })
      expect(slider).toHaveAttribute('aria-valuenow', previous)
      fireEvent.keyDown(slider, { key: 'ArrowRight' })
      expect(slider).toHaveAttribute('aria-valuenow', next)
    }
  })

  it('阅读排版保存看图操作与目录显示方式', async () => {
    const user = userEvent.setup()
    setup()
    await user.click(await screen.findByRole('button', { name: '打开设置' }))
    await user.click(screen.getByRole('button', { name: '阅读排版' }))
    const wheel = within(screen.getByRole('radiogroup', { name: '看图时滚动鼠标' }))
    const outline = within(screen.getByRole('radiogroup', { name: '文章目录' }))
    const backdropClose = screen.getByRole('switch', { name: '点击空白处退出看图' })
    expect(wheel.getByRole('radio', { name: '缩放' })).toBeChecked()
    expect(outline.getByRole('radio', { name: '刻度' })).toBeChecked()
    expect(backdropClose).toBeChecked()
    await user.click(wheel.getByRole('radio', { name: '切换图片' }))
    await user.click(outline.getByRole('radio', { name: '自适应' }))
    await user.click(backdropClose)
    await waitFor(() => expect(api.update).toHaveBeenLastCalledWith(expect.objectContaining({
      reader_image_wheel: 'navigate', reader_image_backdrop_close: false, reader_outline: 'auto',
    }), expect.any(Number)))
  })

  it('隐藏侧栏后保留标题栏按钮和键盘焦点', async () => {
    setup()
    const toggle = await screen.findByRole('button', { name: '折叠侧栏' })
    await userEvent.click(toggle)
    expect(screen.queryByRole('complementary', { name: '主导航' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '展开侧栏', expanded: false })).toBe(toggle)
    expect(toggle).toHaveFocus()
    await userEvent.keyboard('{Enter}')
    expect(screen.getByRole('complementary', { name: '主导航' })).toBeVisible()
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
  })

  it('分类与订阅滚过固定导航的边缘时才显示分隔线', async () => {
    const { container } = setup()
    await screen.findByRole('navigation', { name: '阅读视图' })
    const scroller = container.querySelector<HTMLElement>('.sidebar-organize')!
    const edge = scroller.parentElement
    expect(edge).toHaveAttribute('data-scrolled', 'false')
    scroller.scrollTop = 12
    fireEvent.scroll(scroller)
    expect(edge).toHaveAttribute('data-scrolled', 'true')
    scroller.scrollTop = 0
    fireEvent.scroll(scroller)
    expect(edge).toHaveAttribute('data-scrolled', 'false')
  })

  it('分隔条可用键盘调整并保存，命令面板可打开设置', async () => {
    setup()
    const separator = await screen.findByRole('separator', { name: '调整侧栏宽度' })
    fireEvent.keyDown(separator, { key: 'ArrowRight' })
    expect(useWorkspace.getState().preferences?.sidebar_width).toBe(256)
    fireEvent.keyDown(separator, { key: 'End' })
    expect(useWorkspace.getState().preferences?.sidebar_width).toBe(360)
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    expect(await screen.findByRole('dialog', { name: '命令面板' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('option', { name: '打开设置' }))
    expect(await screen.findByRole('dialog', { name: '设置' })).toBeInTheDocument()
  })

  it('侧栏拖到最小宽度后继续向左可收起，取消拖动不保存', async () => {
    setup()
    const separator = await screen.findByRole('separator', { name: '调整侧栏宽度' })
    const start = { pointerId: 1, button: 0, clientX: 248 }
    fireEvent.pointerDown(separator, start)
    fireEvent.pointerMove(separator, { pointerId: 1, clientX: 220 })
    expect(separator).toHaveAttribute('aria-valuenow', '220')
    fireEvent.pointerCancel(separator, { pointerId: 1 })
    expect(separator).toHaveAttribute('aria-valuenow', '248')
    fireEvent.pointerDown(separator, start)
    fireEvent.pointerMove(separator, { pointerId: 1, clientX: 185 })
    fireEvent.pointerUp(separator, { pointerId: 1, clientX: 185 })
    expect(screen.getByRole('button', { name: '展开侧栏' })).toHaveFocus()
    expect(useWorkspace.getState().preferences?.sidebar_width).toBe(248)
    await userEvent.keyboard('{Enter}')
    expect(screen.getByRole('separator', { name: '调整侧栏宽度' })).toHaveAttribute('aria-valuenow', '248')
  })

  it('Reader 保存滚动位置，抽屉搜索返回浏览区，J/K 同步焦点', async () => {
    api.get.mockResolvedValue({preferences:preferencesFixture({reader_layout:'split'}),revision:0})
    const { container } = setup(true)
    const first = await screen.findByRole('button', { name: '打开文章：测试文章 0' })
    await userEvent.click(first)
    const reader = container.querySelector<HTMLElement>('.reader-content')!
    reader.scrollTop = 430
    fireEvent.scroll(reader)
    fireEvent.keyDown(window, { key: 'j' })
    await waitFor(() => expect(screen.getByRole('button', { name: '打开文章：测试文章 1' })).toHaveFocus())
    expect(useWorkspace.getState().preferences?.scroll_anchors?.['reader|a-0']?.offset).toBe(430)
    await userEvent.click(first)
    expect(container.querySelector('.reader-content')?.scrollTop).toBe(430)
    await userEvent.click(screen.getByRole('radio', { name: '瀑布流视图' }))
    expect(container.querySelector('.reader-content')?.scrollTop).toBe(430)
    fireEvent.keyDown(window, { key: '/' })
    await waitFor(() => expect(screen.getByRole('textbox', { name: '搜索全部文章' })).toHaveFocus())
    expect(useWorkspace.getState().preferences?.reader_open).toBe(false)
  })

  it('Windows 菜单与全局右键菜单执行同一视图命令并阻止浏览器菜单', async()=>{
    host.isWindows.mockReturnValue(true)
    setup()
    await screen.findByRole('heading',{name:'全部文章'})
    const menu=screen.getByRole('menubar',{name:'应用菜单栏'})
    const bar = menu.closest('.application-menu') as HTMLElement
    const toggle = within(bar).getByRole('button', { name: '折叠侧栏' })
    await userEvent.click(toggle)
    expect(toggle).toHaveAccessibleName('展开侧栏')
    const controls = within(bar).getByRole('group', { name: '窗口控制' })
    await userEvent.click(within(controls).getByRole('button', { name: '最小化窗口' }))
    await userEvent.click(within(controls).getByRole('button', { name: '最大化窗口' }))
    await userEvent.click(within(controls).getByRole('button', { name: '关闭窗口' }))
    expect(host.minimize).toHaveBeenCalledOnce()
    expect(host.maximize).toHaveBeenCalledOnce()
    const onMaximizeChange = host.onMaximizeChange.mock.calls[0]?.[0]
    act(() => onMaximizeChange?.(true))
    expect(within(controls).getByRole('button', { name: '还原窗口' })).toBeInTheDocument()
    expect(host.close).toHaveBeenCalledOnce()
    await userEvent.click(within(menu).getByRole('menuitem',{name:'显示'}))
    await userEvent.click(await screen.findByRole('menuitem',{name:'瀑布流视图'}))
    expect(useWorkspace.getState().preferences?.article_view_mode).toBe('masonry')
    const event=new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:200,clientY:200})
    act(()=>screen.getByRole('heading',{name:'全部文章'}).dispatchEvent(event))
    expect(event.defaultPrevented).toBe(true)
    await userEvent.click(await screen.findByRole('menuitem',{name:'列表视图'}))
    expect(useWorkspace.getState().preferences?.article_view_mode).toBe('list')
  })

  it('macOS 使用主机菜单事件，默认 Reader 为聚焦面板；图标只在完整动态交互时启动', async()=>{
    host.isMac.mockReturnValue(true)
    api.get.mockResolvedValue({preferences:preferencesFixture(),revision:0})
    const {container}=setup(true)
    await userEvent.click(await screen.findByRole('radio',{name:'瀑布流视图'}))
    const trigger=await screen.findByRole('button',{name:'打开文章：测试文章 0'})
    expect(screen.queryByRole('menubar')).not.toBeInTheDocument()
    await userEvent.click(trigger)
    expect(container.querySelector('.reader-fill')).toBeInTheDocument()
    expect(screen.queryByRole('region',{name:'文章浏览'})).not.toBeInTheDocument()
    act(()=>useWorkspace.getState().patch({reader_layout:'drawer'}))
    expect(container.querySelector('.reader-drawer')).toBeInTheDocument()
    const settings=screen.getByRole('button',{name:'打开设置'})
    await userEvent.hover(settings)
    expect(settings.querySelector('[data-animating=true]')).toBeInTheDocument()
    act(()=>useWorkspace.getState().patch({animation:'off'}))
    expect(settings.querySelector('[data-animating=true]')).not.toBeInTheDocument()
    host.isMac.mockReturnValue(false)
  })

  it.each([
    { button: '浏览分类 科技', heading: '科技', kind: 'category', id: 'tech' },
    { button: '浏览订阅 博客', heading: '博客', kind: 'feed', id: 'blog' },
    { button: '未读', heading: '未读', kind: 'unread', id: '' },
  ])('瀑布流铺满阅读时点击 $button 返回目标范围', async ({ button, heading, kind, id }) => {
    const user = userEvent.setup()
    api.get.mockResolvedValue({
      preferences: preferencesFixture({ article_view_mode: 'masonry', reader_layout: 'fill', reader_auto_read: false }),
      revision: 0,
    })
    const { client, container } = setup(true)
    client.setQueryData(['navigation'], {
      categories: [{ id: 'tech', parent_id: '', name: '科技', icon: 'folder', unread: 1 }],
      feeds: [{ id: 'blog', title: '博客', unread: 1 }],
      totals: { total: 20, unread: 10, favorites: 3 },
    })
    library.list.mockResolvedValueOnce({
      items: [{ ...articleFixtures(1)[0], id: 'target', title: '目标范围的文章', isRead: false }],
      hasMore: false,
    })

    await user.click(await screen.findByRole('button', { name: '打开文章：测试文章 0' }))
    expect(container.querySelector('.reader-fill')).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: '文章浏览' })).not.toBeInTheDocument()

    const destination = within(screen.getByRole('complementary', { name: '主导航' })).getByRole('button', { name: button })
    await user.click(destination)
    const browse = screen.getByRole('region', { name: '文章浏览' })
    expect(within(browse).getByRole('heading', { name: heading })).toBeInTheDocument()
    expect(await within(browse).findByRole('button', { name: '打开文章：目标范围的文章' })).toBeInTheDocument()
    await waitFor(() => expect(container.querySelector('.reader-fill')).not.toBeInTheDocument())
    expect(destination).toHaveAttribute('aria-current', 'page')
    expect(library.list).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({
      scope: id ? { kind, id } : { kind },
    }), expect.any(AbortSignal))
    await waitFor(() => expect(api.update).toHaveBeenLastCalledWith(expect.objectContaining({
      selected_scope: kind, selected_scope_id: id, reader_open: false,
    }), expect.any(Number)))
  })

  it('阅读模式从工具栏重新提取正文', async () => {
    reader.extract.mockResolvedValue({article_id:'a-0',html:'',text:'已提取正文',mode:'reader'})
    const {client} = setup(true)
    const items = articleFixtures(20).map(article => ({...article, url:'https://example.test/article'}))
    client.setQueryData(articleQueryKey, initialLibraryData({items, hasMore:false}))
    await userEvent.click(await screen.findByRole('button',{name:'打开文章：测试文章 0'}))
    const refresh=within(screen.getByRole('complementary',{name:'阅读面板'})).getByRole('button',{name:'重新提取正文'})
    await userEvent.click(refresh)
    await waitFor(() => expect(refresh).toBeEnabled())
    expect(reader.extract).toHaveBeenCalledTimes(1)
    await userEvent.click(refresh)
    expect(reader.extract).toHaveBeenCalledTimes(2)
  })

  it('正文和看图模式使用各自的右键菜单，均可复制当前图片', async () => {
    const user = userEvent.setup()
    const id = 'a'.repeat(64)
    reader.get.mockResolvedValue({
      article_id: 'a-0', html: `<img src="/media/inline/${id}" alt="文章配图">`, text: '', mode: 'feed',
      has_cached: false, has_original: false,
    })
    copyImage.mockResolvedValue({ ok: true })
    const { client } = setup()
    const articles = articleFixtures(1).map(article => ({ ...article, contentText: '' }))
    act(() => client.setQueryData(articleQueryKey, initialLibraryData({ items: articles, hasMore: false })))
    await user.click(await screen.findByRole('button', { name: '打开文章：测试文章 0' }))
    const image = await screen.findByRole('button', { name: '查看图片：文章配图' })
    fireEvent.contextMenu(image)
    const articleMenu = await screen.findByRole('menu', { name: '阅读上下文菜单' })
    await user.click(within(articleMenu).getByRole('menuitem', { name: '复制图片' }))
    expect(copyImage).toHaveBeenCalledWith(id)
    await user.click(image)
    const viewer = screen.getByRole('region', { name: '看图模式' })
    fireEvent.contextMenu(viewer)
    const imageMenu = await screen.findByRole('menu', { name: '看图操作' })
    expect(screen.queryByRole('menu', { name: '阅读上下文菜单' })).not.toBeInTheDocument()
    await user.click(within(imageMenu).getByRole('menuitem', { name: '复制图片' }))
    expect(copyImage.mock.calls).toEqual([[id], [id]])
  })

  it('通知设置使用分段选择，授权通过后保存方式，独立保存订阅通知自动收起', async () => {
    const user = userEvent.setup()
    authorize.mockResolvedValue(true)
    setup()
    await user.click(await screen.findByRole('button', { name: '打开设置' }))
    const dialog = within(screen.getByRole('dialog', { name: '设置' }))
    await user.click(dialog.getByRole('button', { name: '通知' }))
    const mode = within(dialog.getByRole('radiogroup', { name: '通知方式' }))
    expect(mode.getByRole('radio', { name: '应用内通知' })).toBeChecked()
    const autoDismiss = dialog.getByRole('switch', { name: '自动收起应用内订阅更新通知' })
    expect(autoDismiss).not.toBeChecked()

    await user.click(autoDismiss)
    expect(autoDismiss).toBeChecked()
    await user.click(mode.getByRole('radio', { name: '系统通知' }))
    expect(authorize).toHaveBeenCalledTimes(1)
    expect(mode.getByRole('radio', { name: '系统通知' })).toBeChecked()
    await waitFor(() => expect(api.update).toHaveBeenLastCalledWith(expect.objectContaining({
      notification_mode: 'native',
      refresh_toast_auto_dismiss: true,
    }), expect.any(Number)))

    await user.click(dialog.getByRole('button', { name: '订阅' }))
    expect(dialog.queryByRole('radiogroup', { name: '通知方式' })).not.toBeInTheDocument()
  })

  it('通过更多打开工具栏设置，搜索并保存常驻操作', async () => {
    const user = userEvent.setup()
    setup(true)
    await user.click(await screen.findByRole('button',{name:'打开文章：测试文章 0'}))
    const pane = within(screen.getByRole('complementary',{name:'阅读面板'}))
    await user.click(pane.getByRole('button',{name:'更多'}))
    const menuCategory = await screen.findByRole('menuitem', { name: '归类' })
    expect(menuCategory.querySelector('[data-meaning=folder]')).toBeInTheDocument()
    await user.click(await screen.findByRole('menuitem',{name:'工具栏设置…'}))
    const settings = await screen.findByRole('dialog',{name:'设置'})
    await user.click(within(settings).getByRole('button',{name:'常驻工具栏按钮'}))
    await user.type(screen.getByRole('combobox'), '文章分类')
    const categoryOption = screen.getByRole('option', { name: '归类' })
    expect(categoryOption.querySelector('[data-meaning=folder]')).toBeInTheDocument()
    expect(categoryOption).toHaveAttribute('aria-checked', 'false')
    await user.click(categoryOption)
    expect(categoryOption).toHaveAttribute('aria-checked', 'true')
    await user.clear(screen.getByRole('combobox'))
    await user.type(screen.getByRole('combobox'), '收藏')
    await user.click(screen.getByRole('option',{name:'收藏'}))
    await user.keyboard('{Escape}')
    await user.keyboard('{Escape}')
    expect(pane.getByRole('button',{name:'归类'}).querySelector('[data-meaning=folder]')).toBeInTheDocument()
    expect(pane.queryByRole('button',{name:'收藏'})).not.toBeInTheDocument()
    await waitFor(() => expect(api.update).toHaveBeenLastCalledWith(expect.objectContaining({
      reader_toolbar_pinned: ['refresh', 'translate', 'ai', 'categories'],
    }), expect.any(Number)))
  })

  it('输入框右键保留剪切与粘贴，受控搜索状态随编辑更新', async()=>{
    setup()
    await userEvent.click(await screen.findByRole('button',{name:'搜索文章'}))
    const field=screen.getByRole('textbox',{name:'搜索全部文章'}) as HTMLInputElement
    await userEvent.type(field,'abcdef')
    field.setSelectionRange(1,3)
    await act(async()=>{fireEvent.contextMenu(field);await new Promise(resolve=>setTimeout(resolve,0))})
    expect(document.querySelector('.field-selection mark')).toHaveTextContent('bc')
    let copied:()=>void=()=>{}
    host.copy.mockImplementationOnce(()=>new Promise<void>(resolve=>{copied=resolve}))
    await userEvent.click(await screen.findByRole('menuitem',{name:'剪切'}))
    await act(async()=>{copied();await new Promise(resolve=>setTimeout(resolve,0))})
    expect(host.copy).toHaveBeenCalledWith('bc')
    expect(document.querySelector('.field-selection')).toBeNull()
    await waitFor(()=>expect(useWorkspace.getState().search).toBe('adef'))
    field.setSelectionRange(1,1)
    await act(async()=>{fireEvent.contextMenu(field);await new Promise(resolve=>setTimeout(resolve,0))})
    let pasted:(text:string)=>void=()=>{}
    host.paste.mockImplementationOnce(()=>new Promise<string>(resolve=>{pasted=resolve}))
    await userEvent.click(await screen.findByRole('menuitem',{name:'粘贴'}))
    await act(async()=>{pasted('粘贴文本');await new Promise(resolve=>setTimeout(resolve,0))})
    await waitFor(()=>expect(useWorkspace.getState().search).toBe('a粘贴文本def'))
  })

  it('真实数据读取失败提供重试，正文按选择单独读取',async()=>{
    library.list.mockRejectedValueOnce(new Error('数据库不可读'))
    const client=new QueryClient({defaultOptions:{queries:{retry:false}}})
    render(<App client={client}/>)
    expect(await screen.findByText('暂时无法读取文章')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button',{name:'重新读取文章'}))
    expect(await screen.findByText('暂无文章')).toBeInTheDocument()
    const articles=articleFixtures(1).map(article=>({...article,contentText:''}))
    act(()=>client.setQueryData(articleQueryKey,initialLibraryData({items:articles,hasMore:false})))
    reader.get.mockResolvedValueOnce({article_id:'a-0',html:'<p>按需读取的完整正文</p><pre><code class="language-python">def hello(): return &quot;&lt;script&gt;&quot;</code></pre>',text:'按需读取的完整正文',mode:'feed',has_cached:false,has_original:false})
    await userEvent.click(await screen.findByRole('button',{name:'打开文章：测试文章 0'}))
    expect(await screen.findByText('按需读取的完整正文')).toBeInTheDocument()
    expect(reader.get).toHaveBeenCalledWith('a-0','feed',expect.any(AbortSignal))
    await waitFor(() => expect(document.querySelector('code .hljs-keyword')).toHaveTextContent('def'))
    expect(document.querySelector('code')).toHaveTextContent('<script>')
    expect(document.querySelector('code script')).toBeNull()
  })

  it('命令面板名称与行为一致，精简设置仍保留功能状态和焦点', async()=>{
    setup()
    const trigger=await screen.findByRole('button',{name:'命令面板'})
    expect(trigger).toHaveAttribute('aria-keyshortcuts','Control+K')
    await userEvent.click(trigger)
    const dialog=await screen.findByRole('dialog',{name:'命令面板'})
    expect(within(dialog).getByRole('combobox',{name:'搜索命令'})).toBeInTheDocument()
    const settingsCommand = within(dialog).getByRole('option', { name: '打开设置' })
    const shortcut = settingsCommand.querySelector('[data-slot="command-shortcut"]')
    expect(shortcut).toHaveTextContent('Ctrl+,')
    expect(shortcut?.querySelectorAll('kbd')).toHaveLength(2)

    await userEvent.click(screen.getByRole('option',{name:'订阅管理…'}))
    expect(await screen.findByRole('textbox',{name:'订阅或网页地址'})).toBeInTheDocument()
    expect(screen.queryByText(/RSS 阶段/)).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button',{name:/^关闭$/}))
    await waitFor(()=>expect(screen.getByRole('button',{name:'打开设置'})).toHaveFocus())
  })

  it.each(['list', 'masonry'] as const)('%s 接近底部自动载入，跳过空页，失败后可重试', async mode => {
    const observations: { root: Element | Document | null | undefined; target: Element; intersect: (visible: boolean) => void }[] = []
    vi.stubGlobal('IntersectionObserver', class {
      constructor(private callback: IntersectionObserverCallback, private options: IntersectionObserverInit) {}
      observe(target: Element) {
        observations.push({
          root: this.options.root, target,
          intersect: visible => this.callback([{ target, isIntersecting: visible } as IntersectionObserverEntry], this as unknown as IntersectionObserver),
        })
      }
      disconnect() {}
    })
    api.get.mockResolvedValue({ preferences: preferencesFixture({ article_view_mode: mode }), revision: 0 })
    library.list
      .mockResolvedValueOnce({ items: [], hasMore: true, nextCursor: 'third' })
      .mockRejectedValueOnce(new Error('读取失败'))
      .mockResolvedValueOnce({ items: [{ ...articleFixtures(1)[0], id: 'last' }], hasMore: false })

    const { client, container } = setup()
    await screen.findByRole('heading',{name:'全部文章'})
    act(() => client.setQueryData(articleQueryKey, initialLibraryData({ items: articleFixtures(20), hasMore: true, nextCursor: 'second' })))
    await waitFor(() => expect(observations).toHaveLength(1))
    const viewport = container.querySelector('.article-scroll')
    const first = observations[0]
    expect(first.root).toBe(viewport)
    expect(first.target.previousElementSibling).toBe(viewport?.querySelector('.virtual-articles'))
    expect(screen.queryByRole('button', { name: '载入更多文章' })).not.toBeInTheDocument()
    act(() => first.intersect(false))
    expect(library.list).not.toHaveBeenCalled()
    act(() => { first.intersect(true); first.intersect(true) })
    await waitFor(() => expect(observations).toHaveLength(2))
    expect(library.list).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ cursor: 'second' }), expect.any(AbortSignal))

    act(() => observations[1].intersect(true))
    const retry = await screen.findByRole('button', { name: '重试' })
    expect(library.list).toHaveBeenLastCalledWith(expect.objectContaining({ cursor: 'third' }), expect.any(AbortSignal))
    expect(observations).toHaveLength(2)
    await userEvent.click(retry)
    await waitFor(() => expect(libraryArticles(client.getQueryData<LibraryData>(articleQueryKey))).toHaveLength(21))
    expect(library.list).toHaveBeenCalledTimes(3)
    expect(screen.queryByRole('button', { name: '重试' })).not.toBeInTheDocument()
  })

  it('偏好保存失败仍然可见',async()=>{
    setup()
    await screen.findByRole('heading',{name:'全部文章'})
    act(()=>useWorkspace.setState({saveStatus:'error',saveError:'保存失败'}))
    expect(await screen.findByText('偏好保存失败')).toBeInTheDocument()
  })

  it('设置载入失败显示重试，不能冒充已恢复成功', async () => {
    api.get.mockRejectedValueOnce(new Error('数据库不可用'))
    setup()
    expect(await screen.findByText('暂时无法读取本地设置')).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: '阅读视图' })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '重新载入' }))
    expect(await screen.findByRole('heading', { name: '全部文章' })).toBeInTheDocument()
  })
})

describe('菜单图标与快捷键',()=>{
 it('关于显示真实版本和源码入口，开发者模式控制帮助菜单', async () => {
  const user = userEvent.setup()
  host.isWindows.mockReturnValue(true)
  setup()
  const help = await screen.findByRole('menuitem', { name: '帮助' })
  await user.click(help)
  expect(screen.queryByRole('menuitem', { name: '开发者工具' })).not.toBeInTheDocument()
  await user.click(await screen.findByRole('menuitem', { name: '关于折栈' }))
  const settings = within(await screen.findByRole('dialog', { name: '设置' }))
  expect(await settings.findByRole('heading', { name: '折栈 Fealdhord' })).toBeInTheDocument()
  expect(await settings.findByText('0.1.0-alpha.15')).toBeInTheDocument()
  await user.click(settings.getByRole('button', { name: '检查更新' }))
  expect(await screen.findByText('检查更新暂未开放')).toBeInTheDocument()
  await user.click(settings.getByRole('button', { name: '访问官网' }))
  const websiteToast = (await screen.findByText('官网暂未开放')).closest<HTMLElement>('[data-slot="toast"]')!
  // JSDOM 不加载应用样式，显式模拟 Toast 卡片的 pointer-events-auto。
  websiteToast.style.pointerEvents = 'auto'
  await user.hover(websiteToast)
  await user.click(within(websiteToast).getByRole('button', { name: '关闭通知' }))
  await waitFor(() => expect(screen.queryByText('官网暂未开放')).not.toBeInTheDocument())
  expect(screen.getByRole('dialog', { name: '设置' })).toBeInTheDocument()
  expect(screen.getByText('检查更新暂未开放')).toBeInTheDocument()
  expect(host.openURL).not.toHaveBeenCalled()
  await user.click(settings.getByRole('link', { name: '查看源码' }))
  expect(host.openURL).toHaveBeenCalledWith('https://github.com/coolerks/Fealdhord')
  const developerMode = settings.getByRole('switch', { name: '开发者模式' })
  expect(developerMode).not.toBeChecked()
  await user.click(developerMode)
  await waitFor(() => expect(api.update).toHaveBeenCalledWith(expect.objectContaining({ developer_mode: true }), expect.any(Number)))
  await user.keyboard('{Escape}')

  await user.click(help)
  await user.click(await screen.findByRole('menuitem', { name: '开发者工具' }))
  expect(host.openDevTools).toHaveBeenCalledOnce()
  await user.click(help)
  await user.click(await screen.findByRole('menuitem', { name: '连接诊断…' }))
  expect(await screen.findByRole('dialog', { name: '连接诊断' })).toBeInTheDocument()
  expect(await screen.findByText('桌面服务已连接')).toBeInTheDocument()
 })
 it('右键按上下文分组，快捷键不冒充另一个文章目标，菜单整行播放图标',async()=>{
  const {client}=setup(true)
  const article=await screen.findByRole('button',{name:'打开文章：测试文章 1'})
  act(()=>useWorkspace.getState().patch({selected_article_id:'a-0',reader_open:false}))
  await act(async()=>{fireEvent.contextMenu(article);await new Promise(resolve=>setTimeout(resolve,0))})
  const menu=screen.getByRole('menu',{name:'阅读上下文菜单'})
  expect(within(menu).getByRole('group',{name:'文章操作'})).toBeInTheDocument()
  expect(within(menu).getByRole('group',{name:'视图'})).toBeInTheDocument()
  expect(within(menu).getByRole('group',{name:'应用'})).toBeInTheDocument()
  expect(within(menu).queryByRole('group',{name:'文本编辑'})).not.toBeInTheDocument()
  expect(menu.querySelectorAll('[role=separator]')).toHaveLength(2)
  for(const item of within(menu).getAllByRole('menuitem')) expect(item.querySelector('.animated-glyph svg')).toBeInTheDocument()
  const settings=within(menu).getByRole('menuitem',{name:'设置…'})
  expect(settings).toHaveAttribute('aria-keyshortcuts','Control+,')
  expect(settings.lastElementChild).toHaveTextContent('Ctrl+,')
  const favorite=within(menu).getByRole('menuitem',{name:'收藏／取消收藏'})
  expect(favorite).not.toHaveAttribute('aria-keyshortcuts')
  await userEvent.hover(favorite)
  expect(favorite.querySelector('[data-animating=true]')).toBeInTheDocument()
  await userEvent.click(favorite)
  await waitFor(()=>expect(libraryArticles(client.getQueryData<LibraryData>(articleQueryKey))[1].isFavorite).toBe(true))
  expect(useWorkspace.getState().preferences?.selected_article_id).toBe('a-0')
 })
 it('macOS 右键显示系统格式，关闭动态时 Portal 也保持静态',async()=>{
  host.isMac.mockReturnValue(true)
  api.get.mockResolvedValue({preferences:preferencesFixture({animation:'off'}),revision:0})
  setup()
  const heading=await screen.findByRole('heading',{name:'全部文章'})
  await act(async()=>{fireEvent.contextMenu(heading);await new Promise(resolve=>setTimeout(resolve,0))})
  const menu=screen.getByRole('menu',{name:'阅读上下文菜单'})
  expect(within(menu).getAllByRole('group')).toHaveLength(2)
  const settings=within(menu).getByRole('menuitem',{name:'设置…'})
  expect(settings.lastElementChild).toHaveTextContent('⌘+,')
  expect(settings).toHaveAttribute('aria-keyshortcuts','Meta+,')
  await userEvent.hover(settings)
  expect(menu.querySelector('[data-animating=true]')).not.toBeInTheDocument()
 })
 it('只读文本菜单保留复制和全选，禁用修改项；Windows 编辑菜单有相同快捷键',async()=>{
  setup()
  await userEvent.click(await screen.findByRole('button',{name:'搜索文章'}))
  const field=screen.getByRole('textbox',{name:'搜索全部文章'}) as HTMLInputElement
  await userEvent.type(field,'abcdef')
  field.readOnly=true;field.setSelectionRange(0,2)
  await act(async()=>{fireEvent.contextMenu(field);await new Promise(resolve=>setTimeout(resolve,0))})
  const group=screen.getByRole('group',{name:'文本编辑'})
  expect(within(group).getByRole('menuitem',{name:'剪切'})).toHaveAttribute('data-disabled')
  expect(within(group).getByRole('menuitem',{name:'粘贴'})).toHaveAttribute('data-disabled')
  const copy=within(group).getByRole('menuitem',{name:'复制'})
  expect(copy).toHaveAttribute('aria-keyshortcuts','Control+C')
  expect(copy.lastElementChild).toHaveTextContent('Ctrl+C')
  await userEvent.click(copy)
  expect(host.copy).toHaveBeenCalledWith('ab')
  await userEvent.click(screen.getByRole('menuitem',{name:'编辑'}))
  const menu=await screen.findByRole('menu',{name:'编辑'})
  const paste=within(menu).getByRole('menuitem',{name:'粘贴'})
  expect(paste.querySelector('.animated-glyph svg')).toBeInTheDocument()
  expect(paste.lastElementChild).toHaveTextContent('Ctrl+V')
 })
})


it('从不加载图片时立即隐藏已有封面，不等待后端 Query 重取', async()=>{
  const prefs=preferencesFixture();prefs.remote_images='never'
  api.get.mockResolvedValue({preferences:prefs,revision:0})
  setup(true)
  await screen.findByRole('button',{name:'打开文章：测试文章 0'})
  expect(document.querySelector('.article-cover img')).toBeNull()
})
