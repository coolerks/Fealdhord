import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { QueryClient } from '@tanstack/react-query'
import { articleFixtures, preferencesFixture } from '@/test/fixtures'
import { installGeometry } from '@/test/geometry'
import { useWorkspace } from '@/stores/workspace'
import { articleQueryKey } from '@/lib/library'

const host = vi.hoisted(()=>({isMac:vi.fn(()=>false),onCommand:vi.fn(()=>()=>{}),copy:vi.fn(async()=>{}),paste:vi.fn(async()=>'粘贴文本'),close:vi.fn(),minimize:vi.fn(),maximize:vi.fn(),fullscreen:vi.fn(),quit:vi.fn()}))
vi.mock('@/lib/native',()=>({native:host}))
const api = vi.hoisted(() => ({ get: vi.fn(), update: vi.fn() }))
vi.mock('@/lib/preferences', () => ({ preferencesAPI: api }))
vi.mock('@/lib/api', () => ({ desktopAPI: { info: vi.fn(), subscribe: vi.fn(() => () => {}), start: vi.fn(), cancel: vi.fn(), snapshot: vi.fn() } }))
import App from './App'

let geometry: ReturnType<typeof installGeometry>
beforeEach(() => {
  vi.clearAllMocks()
  host.isMac.mockReturnValue(false)
  geometry = installGeometry()
  useWorkspace.setState({ preferences: null, revision: 0, version: 0, savedVersion: 0, saveError: null, saveStatus: 'saved', search: '', sort: 'newest', settingsOpen: false, settingsTab: 'appearance', paletteOpen: false, searchOpen: false, aiOpen: false, helpOpen:false,commandError:null })
  api.get.mockResolvedValue({ preferences: preferencesFixture(), revision: 0 })
  api.update.mockImplementation(async (preferences, revision) => ({ preferences, revision: revision + 1 }))
})
afterEach(() => { geometry.cleanup(); vi.restoreAllMocks() })

function setup(withArticles = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  client.setQueryData(articleQueryKey, withArticles ? articleFixtures(20) : [])
  return { client, ...render(<App client={client} />) }
}

describe('Phase 1 阅读 Shell', () => {
  it('真实空库显示导航、Reader 和两个可切换视图，不混入示例文章', async () => {
    setup()
    expect(await screen.findByRole('heading', { name: '全部文章' })).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: '阅读视图' })).toBeInTheDocument()
    expect(screen.queryByRole('complementary',{name:'阅读面板'})).not.toBeInTheDocument()
    expect(screen.getByText('还没有订阅')).toBeInTheDocument()
    expect(screen.queryByText('测试文章 0')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('radio', { name: '瀑布流视图' }))
    expect(screen.getByText('瀑布流 · 图文浏览')).toBeInTheDocument()
    expect(api.update).toHaveBeenCalledWith(expect.objectContaining({ article_view_mode: 'masonry' }), expect.any(Number))
  })

  it('视图切换保留选择与共享收藏状态，搜索不会标记文章已读', async () => {
    const { client } = setup(true)
    await screen.findByRole('button', { name: '打开文章：测试文章 0' })
    await userEvent.click(screen.getByRole('button', { name: '打开文章：测试文章 0' }))
    expect(screen.getByRole('heading', { name: '测试文章 0' })).toBeInTheDocument()
    fireEvent.keyDown(window, { key: 'f' })
    expect(client.getQueryData<{ id: string; isFavorite: boolean }[]>(articleQueryKey)?.[0].isFavorite).toBe(false)
    await userEvent.click(screen.getByRole('radio', { name: '瀑布流视图' }))
    await act(async () => { geometry.resize() })
    expect(useWorkspace.getState().preferences?.selected_article_id).toBe('a-0')
    expect(screen.getByRole('heading', { name: '测试文章 0' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '搜索文章' }))
    const search = screen.getByRole('textbox', { name: '搜索当前文章' })
    await userEvent.type(search, '不存在')
    expect(screen.getByText('没有找到匹配文章')).toBeInTheDocument()
    fireEvent.keyDown(search, { key: 'm' })
    expect(client.getQueryData<{ isRead: boolean }[]>(articleQueryKey)?.[0].isRead).toBe(true)
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

  it('分隔条可用键盘调整并保存，命令面板可打开设置', async () => {
    setup()
    const separator = await screen.findByRole('separator', { name: '调整侧栏宽度' })
    fireEvent.keyDown(separator, { key: 'ArrowRight' })
    expect(useWorkspace.getState().preferences?.sidebar_width).toBe(256)
    fireEvent.keyDown(separator, { key: 'End' })
    expect(useWorkspace.getState().preferences?.sidebar_width).toBe(280)
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    expect(await screen.findByRole('dialog', { name: '搜索与命令' })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('option', { name: '打开设置' }))
    expect(await screen.findByRole('dialog', { name: '设置' })).toBeInTheDocument()
  })

  it('Reader 保存滚动位置，抽屉搜索返回浏览区，J/K 同步焦点', async () => {
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
    await waitFor(() => expect(screen.getByRole('textbox', { name: '搜索当前文章' })).toHaveFocus())
    expect(useWorkspace.getState().preferences?.reader_open).toBe(false)
  })

  it('Windows 菜单与全局右键菜单执行同一视图命令并阻止浏览器菜单', async()=>{
    setup()
    await screen.findByRole('heading',{name:'全部文章'})
    const menu=screen.getByRole('menubar',{name:'应用菜单栏'})
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
    api.get.mockResolvedValue({preferences:preferencesFixture({reader_pinned:false}),revision:0})
    const {container}=setup(true)
    const trigger=await screen.findByRole('button',{name:'打开文章：测试文章 0'})
    expect(screen.queryByRole('menubar')).not.toBeInTheDocument()
    await userEvent.click(trigger)
    expect(container.querySelector('.reader-drawer')).toBeInTheDocument()
    const settings=screen.getByRole('button',{name:'打开设置'})
    await userEvent.hover(settings)
    expect(settings.querySelector('[data-animating=true]')).toBeInTheDocument()
    act(()=>useWorkspace.getState().patch({animation:'off'}))
    expect(settings.querySelector('[data-animating=true]')).not.toBeInTheDocument()
    host.isMac.mockReturnValue(false)
  })

  it('输入框右键保留剪切与粘贴，受控搜索状态随编辑更新', async()=>{
    setup()
    await userEvent.click(await screen.findByRole('button',{name:'搜索文章'}))
    const field=screen.getByRole('textbox',{name:'搜索当前文章'}) as HTMLInputElement
    await userEvent.type(field,'abcdef')
    field.setSelectionRange(1,3)
    await act(async()=>{fireEvent.contextMenu(field);await new Promise(resolve=>setTimeout(resolve,0))})
    let copied:()=>void=()=>{}
    host.copy.mockImplementationOnce(()=>new Promise<void>(resolve=>{copied=resolve}))
    await userEvent.click(await screen.findByRole('menuitem',{name:'剪切'}))
    await act(async()=>{copied();await new Promise(resolve=>setTimeout(resolve,0))})
    expect(host.copy).toHaveBeenCalledWith('bc')
    await waitFor(()=>expect(useWorkspace.getState().search).toBe('adef'))
    field.setSelectionRange(1,1)
    await act(async()=>{fireEvent.contextMenu(field);await new Promise(resolve=>setTimeout(resolve,0))})
    let pasted:(text:string)=>void=()=>{}
    host.paste.mockImplementationOnce(()=>new Promise<string>(resolve=>{pasted=resolve}))
    await userEvent.click(await screen.findByRole('menuitem',{name:'粘贴'}))
    await act(async()=>{pasted('粘贴文本');await new Promise(resolve=>setTimeout(resolve,0))})
    await waitFor(()=>expect(useWorkspace.getState().search).toBe('a粘贴文本def'))
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
