import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { QueryClient } from '@tanstack/react-query'
import { articleFixtures, preferencesFixture } from '@/test/fixtures'
import { installGeometry } from '@/test/geometry'
import * as workspaceMotion from '@/lib/workspace-motion'
import { useWorkspace } from '@/stores/workspace'
import { initialLibraryData,libraryArticles } from '@/lib/library-query'
import type { LibraryData } from '@/lib/library-query'
import { articleQueryKey } from '@/lib/library'
import { feedsQueryKey, rssAPI } from '@/lib/rss-api'
import type { Feed } from '@/lib/rss-api'
import type { LibraryPage } from '@/lib/library'
import type { Detail } from '@/lib/reader-api'
import { useChatTarget } from '@/features/chat/chat-scope'
import { FealdhordChatUpdate } from '../bindings/events'
import type { Message, SendInput, UpdateEvent } from '../bindings/github.com/coolerks/Fealdhord/internal/chat/models'

import type { Task } from '../bindings/github.com/coolerks/Fealdhord/internal/notification/models'

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
const chat = vi.hoisted(() => ({ List: vi.fn(), Create: vi.fn(), Messages: vi.fn(), Send: vi.fn(), Cancel: vi.fn(), Model: vi.fn(), ContextStatus: vi.fn() }))
vi.mock('../bindings/github.com/coolerks/Fealdhord/internal/chat/api', () => chat)
const listAIConfiguration = vi.hoisted(() => vi.fn())
const saveAIModel = vi.hoisted(() => vi.fn())
vi.mock('../bindings/github.com/coolerks/Fealdhord/internal/aiconfig/api', () => ({
  List: listAIConfiguration, SaveModel: saveAIModel, Brands: async () => [],
  FetchModels: () => Object.assign(Promise.resolve([]), { cancel: vi.fn() }),
}))
const notificationAPI = vi.hoisted(() => ({
  ListLocalNotices: vi.fn().mockResolvedValue([]),
  ListChannels: vi.fn().mockResolvedValue([]),
  ListRules: vi.fn().mockResolvedValue([]),
  ListTasks: vi.fn().mockResolvedValue([]),
  ListRuns: vi.fn().mockResolvedValue([]),
  SaveTask: vi.fn(),
  DeleteTask: vi.fn(),
  SaveRule: vi.fn().mockResolvedValue(undefined),
  SaveChannel: vi.fn().mockResolvedValue(undefined),
  SendArticle: vi.fn().mockResolvedValue(undefined),
  StartWeChatBinding: vi.fn(),
  PollWeChatBinding: vi.fn(),
  CancelWeChatBinding: vi.fn(),
  StartRecipientPairing: vi.fn(),
  RecipientPairingStatus: vi.fn(),
  CancelRecipientPairing: vi.fn(),
}))
vi.mock('../bindings/github.com/coolerks/Fealdhord/internal/notification/api', () => notificationAPI)
const listSkills = vi.hoisted(() => vi.fn())
const mcpGrants = vi.hoisted(() => ({
  Grants: vi.fn().mockResolvedValue([]),
  MCPServer: vi.fn().mockResolvedValue({ url: '', config: { port: 8769, tools: [] } }),
  SaveGrant: vi.fn(),
  UpdateGrant: vi.fn().mockResolvedValue(undefined),
  RotateGrantToken: vi.fn(),
}))
vi.mock('../bindings/github.com/coolerks/Fealdhord/internal/appagent/api', () => ({
  PendingActions: async () => [], ToolList: async () => [], Skills: listSkills, MCPClients: async () => [],
  ...mcpGrants,
}))
const copyImage = vi.hoisted(() => vi.fn())
vi.mock('../bindings/github.com/coolerks/Fealdhord/internal/media/api', () => ({ CopyInlineImage: copyImage }))
import App from './App'

let geometry: ReturnType<typeof installGeometry>
beforeEach(() => {
  vi.clearAllMocks()
  notificationAPI.ListChannels.mockResolvedValue([])
  notificationAPI.ListTasks.mockResolvedValue([])
  notificationAPI.ListRuns.mockResolvedValue([])
  mcpGrants.Grants.mockResolvedValue([])
  mcpGrants.MCPServer.mockResolvedValue({ url: '', config: { port: 8769, tools: [] } })
  useChatTarget.setState({ target: null })
  listSkills.mockResolvedValue([])
  chat.ContextStatus.mockResolvedValue({
    budget: { context: 32768, inputLimit: 24064, outputReserve: 8192, estimated: true },
    history: 0, used: 0, contextAfter: 0, summary: '', pins: [], lastInput: 0, autoCompact: false, lastModel: '',
  })
  listAIConfiguration.mockResolvedValue({ providers: [], models: [], assignments: [] })
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
  it('全屏设置替换两栏并返回阅读，保存对话框为下次打开方式', async () => {
    const animate = vi.spyOn(workspaceMotion, 'animateWorkspaceEntry')
    const user = userEvent.setup()
    host.isWindows.mockReturnValue(true)
    api.get.mockResolvedValue({ preferences: preferencesFixture({ settings_open_mode: 'full' }), revision: 0 })
    setup(true)
    await user.click(await screen.findByRole('button', { name: '打开文章：测试文章 0' }))
    const trigger = screen.getByRole('button', { name: '打开设置' })
    await user.click(trigger)
    const sidebar = await screen.findByRole('complementary', { name: '设置侧栏' }, { timeout: 3000 })
    const workspace = screen.getByRole('main', { name: '设置' })
    expect(animate).toHaveBeenLastCalledWith([sidebar, workspace], 'full', 6)
    expect(screen.getByRole('group', { name: '窗口控制' })).toBeInTheDocument()
    expect(screen.queryByRole('dialog', { name: '设置' })).not.toBeInTheDocument()
    expect(screen.queryByRole('complementary', { name: '主导航' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '测试文章 0' })).not.toBeInTheDocument()
    const back = within(sidebar).getByRole('button', { name: '返回' })
    expect(back).toHaveFocus()
    const modes = within(workspace).getByRole('radiogroup', { name: '设置打开方式' })
    expect(within(modes).getByRole('radio', { name: '全屏' })).toBeChecked()
    await user.click(within(modes).getByRole('radio', { name: '对话框' }))
    await waitFor(() => expect(api.update).toHaveBeenLastCalledWith(
      expect.objectContaining({ settings_open_mode: 'dialog' }), expect.any(Number),
    ))
    expect(workspace).toBeInTheDocument()
    await user.click(back)
    expect(screen.getByRole('heading', { name: '测试文章 0' })).toBeInTheDocument()
    expect(animate).toHaveBeenLastCalledWith([
      screen.getByRole('complementary', { name: '主导航' }), screen.getByRole('main'),
    ], 'full', -6)
    await waitFor(() => expect(trigger).toHaveFocus())
    await user.click(trigger)
    expect(await screen.findByRole('dialog', { name: '设置' })).toBeInTheDocument()
    expect(animate).toHaveBeenCalledTimes(2)
  })

  it('从对话框展开设置时保留正在编辑的内容，退出后恢复 AI 草稿', async () => {
    const animate = vi.spyOn(workspaceMotion, 'animateWorkspaceEntry')
    const user = userEvent.setup()
    api.get.mockResolvedValue({ preferences: preferencesFixture({ animation: 'subtle' }), revision: 0 })
    setup()
    chat.List.mockResolvedValue([])
    await user.click(await screen.findByRole('button', { name: 'AI 助手' }))
    const chatInput = await screen.findByRole('textbox', { name: '消息' })
    await user.type(chatInput, '未发送草稿')
    await user.click(screen.getByRole('button', { name: 'AI 设置' }))
    const dialog = await screen.findByRole('dialog', { name: '设置' })
    expect(animate).not.toHaveBeenCalled()
    await user.click(within(dialog).getByRole('button', { name: '订阅' }))
    await user.type(await within(dialog).findByRole('textbox', { name: '订阅或网页地址' }), 'https://draft.example')
    await user.click(within(dialog).getByRole('button', { name: '全屏设置' }))
    const sidebar = await screen.findByRole('complementary', { name: '设置侧栏' })
    const workspace = screen.getByRole('main', { name: '设置' })
    expect(animate).toHaveBeenLastCalledWith([sidebar, workspace], 'subtle', 6)
    expect(within(sidebar).getByRole('button', { name: '订阅' })).toHaveAttribute('aria-current', 'page')
    expect(within(workspace).getByRole('textbox', { name: '订阅或网页地址' })).toHaveValue('https://draft.example')
    expect(screen.queryByRole('main', { name: 'AI 助手工作区' })).not.toBeInTheDocument()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('main', { name: '设置' })).not.toBeInTheDocument())
    expect(screen.getByRole('textbox', { name: '消息' })).toHaveValue('未发送草稿')
    expect(animate).toHaveBeenLastCalledWith([
      screen.getByRole('complementary', { name: 'AI 会话导航' }), screen.getByRole('main', { name: 'AI 助手工作区' }),
    ], 'subtle', -6)
    expect(useWorkspace.getState().preferences?.settings_open_mode).toBe('dialog')
  })

  it.each(['full', 'dialog'] as const)('键盘关闭添加服务弹窗后，焦点回到触发按钮（%s）', async settings_open_mode => {
    const user = userEvent.setup()
    api.get.mockResolvedValue({ preferences: preferencesFixture({ settings_open_mode }), revision: 0 })
    setup()
    await user.click(await screen.findByRole('button', { name: '打开设置' }))
    const nav = await screen.findByRole('navigation', { name: '设置分区' })
    await user.click(within(nav).getByRole('button', { name: '服务与模型' }))
    const trigger = await screen.findByRole('button', { name: '添加服务' })
    trigger.focus()
    await user.keyboard('{Enter}')
    await screen.findByRole('dialog', { name: '添加服务' })
    await user.keyboard('{Escape}')
    await waitFor(() => expect(trigger).toHaveFocus())
  })

  it('最后一个订阅恢复后仍可切回全部订阅', async () => {
    const feed = { id: 'f', title: '测试订阅', url: 'https://example.org/feed', last_error: '请求失败', archived: false } as Feed
    const list = vi.spyOn(rssAPI, 'list').mockResolvedValue({ items: [feed], categories: [] })
    const user = userEvent.setup()
    api.get.mockResolvedValue({ preferences: preferencesFixture({ settings_open_mode: 'full' }), revision: 0 })
    const { client } = setup()
    await user.click(await screen.findByRole('button', { name: '打开设置' }))
    const sidebar = await screen.findByRole('complementary', { name: '设置侧栏' })
    await user.click(within(sidebar).getByRole('button', { name: '订阅' }))
    await user.click(await screen.findByRole('radio', { name: '异常' }))
    list.mockResolvedValue({ items: [{ ...feed, last_error: '' }], categories: [] })
    await act(async () => { await client.invalidateQueries({ queryKey: feedsQueryKey }) })
    await screen.findByText('没有匹配的订阅')
    await user.click(screen.getByRole('radio', { name: '全部' }))
    expect(screen.getByText('测试订阅')).toBeInTheDocument()
  })

  it('编辑 MCP 授权仅保存权限，确认更换后才显示该客户端的新令牌', async () => {
    const grant = { name: 'Reader', scope: { kind: '', id: '' }, tools: ['article.get'] }
    mcpGrants.Grants.mockResolvedValue([grant])
    mcpGrants.MCPServer.mockResolvedValue({ url: '', config: { port: 8769, tools: grant.tools } })
    mcpGrants.RotateGrantToken.mockResolvedValue('test-replacement')
    const user = userEvent.setup()
    setup()
    await user.click(await screen.findByRole('button', { name: '打开设置' }))
    await user.click(await screen.findByRole('button', { name: 'MCP 与权限' }))
    await user.click(await screen.findByRole('button', { name: '编辑' }))
    const editor = await screen.findByRole('dialog', { name: '编辑客户端授权' })
    await user.click(await within(editor).findByRole('button', { name: '保存授权' }))
    await waitFor(() => expect(editor).not.toBeInTheDocument())
    expect(mcpGrants.UpdateGrant).toHaveBeenCalledWith(grant)
    expect(mcpGrants.SaveGrant).not.toHaveBeenCalled()
    expect(mcpGrants.RotateGrantToken).not.toHaveBeenCalled()
    const rotate = await screen.findByRole('button', { name: '更换令牌' })
    await user.click(rotate)
    const confirm = await screen.findByRole('dialog', { name: '更换“Reader”的令牌？' })
    expect(mcpGrants.RotateGrantToken).not.toHaveBeenCalled()
    await user.click(within(confirm).getByRole('button', { name: '确认更换' }))
    expect(await screen.findByRole('textbox', { name: 'Reader的客户端令牌' })).toHaveValue('test-replacement')
    expect(mcpGrants.RotateGrantToken).toHaveBeenCalledWith('Reader')
  })

  it('从 AI 侧栏管理定时任务，返回后保留聊天草稿', async () => {
    const user = userEvent.setup()
    let tasks: Task[] = []
    notificationAPI.ListTasks.mockImplementation(async () => tasks)
    notificationAPI.SaveTask.mockImplementation(async (task: Task) => {
      const saved = { ...task, id: 'daily', revision: task.revision + 1, next_at: Date.now() + 60_000 }
      tasks = [saved]
      return saved
    })
    notificationAPI.ListRuns.mockResolvedValue([{ occurrence: 1700000000000, state: 'complete', result: '已完成一次阅读提醒' }])
    notificationAPI.DeleteTask.mockImplementation(async () => { tasks = [] })

    setup()
    chat.List.mockResolvedValue([])
    await user.click(await screen.findByRole('button', { name: 'AI 助手' }))
    await user.type(await screen.findByRole('textbox', { name: '消息' }), '未发送的草稿')
    const sidebar = within(await screen.findByRole('complementary', { name: 'AI 会话导航' }))
    const taskEntry = sidebar.getByRole('button', { name: '定时任务' })
    await user.click(taskEntry)
    expect(taskEntry).toHaveAttribute('aria-current', 'page')
    const manager = within(await screen.findByRole('region', { name: '定时任务' }))
    expect(screen.queryByRole('textbox', { name: '消息' })).not.toBeInTheDocument()
    await user.click(manager.getByRole('button', { name: '添加任务' }))
    const create = within(await screen.findByRole('dialog', { name: '添加任务' }))
    await user.type(create.getByRole('textbox', { name: '任务名称' }), '阅读提醒')
    await user.type(create.getByRole('textbox', { name: '任务正文' }), '读一篇文章')
    await user.click(create.getByRole('button', { name: '保存任务' }))
    expect(await manager.findByRole('heading', { name: '阅读提醒' })).toBeInTheDocument()

    await user.click(manager.getByRole('button', { name: '暂停' }))
    expect(await manager.findByRole('button', { name: '恢复' })).toBeInTheDocument()
    expect(notificationAPI.SaveTask).toHaveBeenLastCalledWith(expect.objectContaining({ id: 'daily', enabled: false, revision: 1 }))
    await user.click(manager.getByRole('button', { name: '恢复' }))
    expect(await manager.findByRole('button', { name: '暂停' })).toBeInTheDocument()
    await user.click(manager.getByRole('button', { name: '编辑' }))
    const edit = within(await screen.findByRole('dialog', { name: '编辑任务' }))
    const name = edit.getByRole('textbox', { name: '任务名称' })
    await user.clear(name)
    await user.type(name, '晚间阅读')
    await user.click(edit.getByRole('button', { name: '保存任务' }))
    expect(await manager.findByRole('heading', { name: '晚间阅读' })).toBeInTheDocument()
    await user.click(manager.getByRole('button', { name: '运行记录' }))
    expect(await manager.findByText('已完成一次阅读提醒')).toBeInTheDocument()
    await user.click(manager.getByRole('button', { name: '删除' }))
    const confirmation = within(await screen.findByRole('dialog', { name: '删除 晚间阅读？' }))
    await user.click(confirmation.getByRole('button', { name: '删除任务' }))
    expect(await manager.findByText('还没有定时任务')).toBeInTheDocument()
    expect(notificationAPI.DeleteTask).toHaveBeenCalledWith('daily', 4)
    await user.click(manager.getByRole('button', { name: '返回对话' }))
    const input = screen.getByRole('textbox', { name: '消息' })
    expect(input).toHaveValue('未发送的草稿')
    await user.clear(input)
    await user.type(input, '/schedule')
    await user.keyboard('{Enter}')
    expect(await screen.findByRole('region', { name: '定时任务' })).toBeInTheDocument()
    expect(chat.Send).not.toHaveBeenCalled()
  })

  it('用斜杠选择技能和模型，不将命令或路径误当成对方', async () => {
    const user = userEvent.setup()
    setup()
    chat.List.mockResolvedValue([{ id: 'slash-chat', title: '命令测试' }])
    chat.Messages.mockResolvedValue([])
    chat.Send.mockResolvedValue(undefined)
    listSkills.mockResolvedValue([{ name: 'reading-notes', description: '整理阅读笔记', enabled: true }])
    listAIConfiguration.mockResolvedValue({
      providers: [{ id: 'provider', name: '本地服务', enabled: true }],
      models: [
        { id: 'first', providerId: 'provider', name: 'first-model', displayName: '默认模型', enabled: true },
        { id: 'second', providerId: 'provider', name: 'second-model', displayName: '备用模型', enabled: true },
      ],
      assignments: [],
    })
    await user.click(await screen.findByRole('button', { name: 'AI 助手' }))
    const input = await screen.findByRole('textbox', { name: '消息' })
    await user.type(input, '/skills')
    expect(screen.getByRole('option', { name: '技能' })).toBeInTheDocument()
    await user.keyboard('{Enter}')
    const skill = await screen.findByRole('option', { name: 'reading-notes' })
    expect(input).toHaveFocus()
    expect(document.getElementById(input.getAttribute('aria-controls')!)).toBeInTheDocument()
    await user.click(skill)
    expect(screen.getByRole('button', { name: '移除技能 reading-notes' })).toBeInTheDocument()
    expect(input).toHaveValue('')
    expect(chat.Send).not.toHaveBeenCalled()

    await user.type(input, '/model')
    await user.keyboard('{Enter}')
    await user.keyboard('{ArrowDown}')
    const highlighted = document.getElementById(input.getAttribute('aria-activedescendant')!)
    expect(highlighted).toHaveAccessibleName('备用模型')
    await user.keyboard('{Enter}')
    expect(screen.getByRole('button', { name: '聊天模型' })).toHaveTextContent('备用模型')
    await user.type(input, '/tmp/article.txt')
    expect(screen.queryByRole('listbox', { name: '斜杠命令' })).not.toBeInTheDocument()
    await user.keyboard('{Enter}')
    await waitFor(() => expect(chat.Send).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({
      conversationId: 'slash-chat', modelId: 'second', text: '/tmp/article.txt', skills: ['reading-notes'],
    })))
  })

  it('会话列表读取失败时不添加提示或触发再次读取', async () => {
    const { client } = setup()
    chat.List.mockRejectedValue(new Error('StorageError'))
    await userEvent.click(await screen.findByRole('button', { name: 'AI 助手' }))
    const sidebar = await screen.findByRole('complementary', { name: 'AI 会话导航' })
    await waitFor(() => expect(client.getQueryState(['chat', 'conversations', 'global', ''])?.status).toBe('error'))
    expect(sidebar).not.toHaveTextContent('会话暂时无法读取')
    expect(within(sidebar).queryByRole('button', { name: '重新读取历史' })).not.toBeInTheDocument()
    expect(chat.List).toHaveBeenCalledTimes(1)
  })

  it('悬停上下文圆环查看用量并在回复结束后更新', async () => {
    const user = userEvent.setup()
    setup()
    chat.List.mockResolvedValue([{ id: 'context-chat', title: '会话记录' }])
    chat.Messages.mockResolvedValue([])
    const status = {
      budget: { context: 2048, inputLimit: 1500, outputReserve: 500, estimated: false },
      history: 1, used: 375, contextAfter: 0, summary: '', pins: [], lastInput: 0, autoCompact: false, lastModel: '',
    }
    chat.ContextStatus.mockResolvedValue(status)
    listAIConfiguration.mockResolvedValue({
      providers: [{ id: 'provider', name: '本地服务', enabled: true }],
      models: [{ id: 'model', providerId: 'provider', name: 'model', displayName: '模型', enabled: true }],
      assignments: [],
    })
    await user.click(await screen.findByRole('button', { name: 'AI 助手' }))
    const sidebar = await screen.findByRole('complementary', { name: 'AI 会话导航' })
    expect(await within(sidebar).findByRole('button', { name: '会话记录' })).toBeInTheDocument()
    expect(within(sidebar).queryByRole('button', { name: '重新读取历史' })).not.toBeInTheDocument()
    const ring = screen.getByRole('button', { name: '会话上下文' })
    await waitFor(() => expect(ring).toHaveAccessibleDescription('已占用输入预算 25%（估算）'))
    expect(ring).toHaveTextContent(/^$/)
    expect(ring.querySelector('circle[pathLength]')).toHaveAttribute('stroke-dasharray', '25 100')
    await user.hover(ring)
    expect(await screen.findByText('375 / 1,500')).toBeInTheDocument()
    chat.ContextStatus.mockResolvedValue({ ...status, used: 750 })
    const runtime: { dispatchWailsEvent: (event: { name: string; data: UpdateEvent }) => void } = Reflect.get(window, '_wails')
    act(() => runtime.dispatchWailsEvent({ name: FealdhordChatUpdate, data: { conversationId: 'context-chat', messageId: 'reply', offset: 0, delta: '', done: true } }))
    expect(await screen.findByText('750 / 1,500')).toBeInTheDocument()
    expect(ring.querySelector('circle[pathLength]')).toHaveAttribute('stroke-dasharray', '50 100')
    await user.click(ring)
    expect(await screen.findByRole('dialog', { name: '会话上下文' })).toBeInTheDocument()
    expect(screen.queryByText('750 / 1,500')).not.toBeInTheDocument()
  })

  it('全局 AI 工作区切换会话和本轮模型，停止后返回原阅读位置', async () => {
    const user = userEvent.setup()
    const { client, container } = setup(true)
    await user.click(await screen.findByRole('button', { name: '打开文章：测试文章 0' }))
    const prose = container.querySelector<HTMLElement>('.reader-content')!
    prose.scrollTop = 430
    fireEvent.scroll(prose)
    const opener = within(screen.getByRole('complementary', { name: '主导航' })).getByRole('button', { name: 'AI 助手' })
    const rows: Message[] = []
    const publish = (data: UpdateEvent) => {
      const runtime: { dispatchWailsEvent: (event: { name: string; data: UpdateEvent }) => void } = Reflect.get(window, '_wails')
      act(() => runtime.dispatchWailsEvent({ name: FealdhordChatUpdate, data }))
    }
    chat.List.mockResolvedValue([{ id: 'history', title: '已有会话' }])
    chat.Create.mockImplementation(async () => {
      const created = { id: 'new', title: '第一条' }
      chat.List.mockResolvedValue([{ id: 'history', title: '已有会话' }, created])
      return created
    })
    chat.Messages.mockImplementation(async (id: string) => id === 'new' ? [...rows] : [{
      id: 'old', role: 'assistant', text: '历史回复', modelName: '默认模型', status: 'complete',
    }])
    chat.Send.mockImplementation(async ({ modelId, text }: SendInput) => {
      const base = { errorCode: '', tools: [] }
      rows.push({ ...base, id: text, role: 'user', text, modelName: '', status: 'complete' })
      rows.push({ ...base, id: `${text}-reply`, role: 'assistant', text: `${text}回复`, modelName: modelId ?? '', status: 'running' })
    })
    chat.Cancel.mockImplementation(async () => { rows[3] = { ...rows[3], status: 'cancelled' } })
    listAIConfiguration.mockResolvedValue({
      providers: [{ id: 'provider', name: '本地服务', enabled: true }],
      models: [
        { id: 'first', providerId: 'provider', name: 'default-model', displayName: '默认模型', enabled: true,
          reasoningLevels: [{ label: '推理增强', value: 'deep-custom' }] },
        { id: 'second', providerId: 'provider', name: 'alternate-model', displayName: '备用模型', enabled: true,
          reasoningLevels: [{ label: '轻量', value: 'lite' }] },
        { id: 'third', providerId: 'provider', name: 'simple-model', displayName: '基础模型', enabled: true },
      ],
      assignments: [{ task: 'chat', modelId: 'first' }],
    })

    await user.click(opener)
    const sidebar = await screen.findByRole('complementary', { name: 'AI 会话导航' })
    expect(screen.queryByRole('complementary', { name: '主导航' })).not.toBeInTheDocument()
    expect(screen.queryByRole('dialog', { name: 'AI 助手' })).not.toBeInTheDocument()
    expect(await screen.findByText('历史回复')).toBeInTheDocument()
    expect(within(sidebar).getByRole('button', { name: '新对话' }))
      .toHaveAttribute('aria-keyshortcuts', 'Control+Shift+N')
    await user.keyboard('{Control>}{Shift>}N{/Shift}{/Control}')
    expect(screen.queryByText('历史回复')).not.toBeInTheDocument()
    expect(chat.Create).not.toHaveBeenCalled()
    const input = screen.getByRole('textbox', { name: '消息' })
    expect(input).toHaveFocus()
    const model = screen.getByRole('button', { name: '聊天模型' })
    expect(model).toHaveTextContent('默认模型')
    expect(model).not.toHaveTextContent('本地服务')
    expect(screen.queryByRole('button', { name: '思考强度' })).not.toBeInTheDocument()
    await user.click(model)
    const modelSearch = screen.getByRole('combobox', { name: '搜索聊天模型' })
    await user.keyboard('{ArrowRight}')
    const effortSearch = screen.getByRole('combobox', { name: '搜索默认模型的思考强度' })
    expect(effortSearch).toHaveFocus()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: '默认模型的思考强度' })).not.toBeInTheDocument()
    await waitFor(() => expect(modelSearch).toHaveFocus())
    await user.keyboard('{Enter}')
    await user.keyboard('{ArrowDown}{Enter}')
    expect(model).toHaveTextContent('默认模型推理增强')
    await user.type(input, '第一条')
    const send = screen.getByRole('button', { name: '发送' })
    expect(send).toHaveClass('rounded-full')
    expect(send).toHaveTextContent(/^$/)
    expect(send.parentElement).toBe(model.parentElement)
    await user.click(send)
    expect(await screen.findByText('第一条回复')).toBeInTheDocument()
    expect(chat.Send).toHaveBeenLastCalledWith(expect.objectContaining({ conversationId: 'new', modelId: 'first', reasoningEffort: 'deep-custom', text: '第一条' }))
    rows[1] = { ...rows[1], status: 'complete' }
    publish({ conversationId: 'new', messageId: rows[1].id, offset: rows[1].text.length, delta: '', done: true })
    await waitFor(() => expect(screen.queryByRole('button', { name: '停止回复' })).not.toBeInTheDocument())

    await user.click(model)
    await user.keyboard('alternate-model')
    const alternate = screen.getByRole('option', { name: '备用模型 · 本地服务 · alternate-model' })
    expect(alternate).toHaveTextContent(/^备用模型$/)
    await user.click(alternate)
    const alternateEffort = screen.getByRole('dialog', { name: '备用模型的思考强度' })
    const serviceDefault = within(alternateEffort).getByRole('option', { name: '使用服务默认' })
    expect(serviceDefault).toHaveAttribute('aria-checked', 'true')
    await user.click(serviceDefault)
    expect(model).toHaveTextContent(/^备用模型$/)
    await user.type(input, '第二条{Enter}')
    expect(await screen.findByText('第二条回复')).toBeInTheDocument()
    expect(screen.queryByText('正在回复…')).not.toBeInTheDocument()
    const timeline = screen.getByRole('region', { name: '对话消息' })
    Object.defineProperties(timeline, { scrollHeight: { value: 900 }, clientHeight: { value: 300 } })
    timeline.scrollTop = 100
    fireEvent.scroll(timeline)
    await user.click(screen.getByRole('button', { name: '回到最新' }))
    expect(timeline.scrollTop).toBe(900)
    expect(timeline).toHaveFocus()
    expect(screen.queryByRole('button', { name: '回到最新' })).not.toBeInTheDocument()
    const reply = rows[3]
    const update = { conversationId: 'new', messageId: reply.id, offset: reply.text.length, delta: '🙂继续', done: false }
    publish({ ...update, conversationId: 'history' })
    expect(screen.queryByText('第二条回复🙂继续')).not.toBeInTheDocument()
    publish(update)
    expect(screen.getByText('第二条回复🙂继续')).toBeInTheDocument()
    publish(update)
    act(() => client.setQueryData(['chat', 'messages', 'new'], [...rows]))
    expect(screen.getByText('第二条回复🙂继续')).toBeInTheDocument()
    rows[3] = { ...reply, text: reply.text + update.delta + '输出' }
    // Recover a dropped event from a snapshot, then apply an overlapping event.
    const outputOffset = rows[3].text.length
    publish({ ...update, offset: outputOffset, delta: '完成' })
    expect(screen.queryByText(/输出完成/)).not.toBeInTheDocument()
    act(() => client.setQueryData(['chat', 'messages', 'new'], [...rows]))
    publish({ ...update, offset: outputOffset - 2, delta: '输出完成' })
    expect(screen.getByText('第二条回复🙂继续输出完成')).toBeInTheDocument()
    expect(chat.Send).toHaveBeenLastCalledWith(expect.objectContaining({ conversationId: 'new', modelId: 'second', text: '第二条' }))
    expect(chat.Send.mock.lastCall?.[0]).not.toHaveProperty('reasoningEffort')
    expect(chat.Create).toHaveBeenCalledExactlyOnceWith({ kind: 'global', id: '' }, '第一条')
    await user.click(screen.getByRole('button', { name: '停止回复' }))
    expect(await screen.findByText('已取消')).toBeInTheDocument()
    expect(chat.Cancel).toHaveBeenCalledExactlyOnceWith('new')
    await user.click(model)
    await user.click(screen.getByRole('option', { name: '基础模型 · 本地服务 · simple-model' }))
    expect(model).toHaveTextContent(/^基础模型$/)
    await waitFor(() => expect(api.update).toHaveBeenLastCalledWith(expect.objectContaining({ chat_model_id: 'third' }), expect.any(Number)))
    expect(screen.queryByRole('dialog', { name: '聊天模型' })).not.toBeInTheDocument()
    await user.click(within(sidebar).getByRole('button', { name: '已有会话' }))
    expect(await screen.findByText('历史回复')).toBeInTheDocument()
    expect(within(sidebar).queryByRole('button', { name: '返回阅读' })).not.toBeInTheDocument()
    const reading = within(sidebar).getByRole('button', { name: '阅读' })
    expect(reading.closest('footer')).toBeInTheDocument()
    await user.click(reading)
    await waitFor(() => expect(opener).toHaveFocus())
    expect(container.querySelector('.reader-content')).toBe(prose)
    expect(prose.scrollTop).toBe(430)

    await user.click(opener)
    const workspace = await screen.findByRole('main', { name: 'AI 助手工作区' })
    expect(await within(workspace).findByRole('button', { name: '聊天模型' })).toHaveTextContent(/^基础模型$/)
    await user.click(screen.getByRole('button', { name: '折叠侧栏' }))
    expect(screen.queryByRole('complementary', { name: 'AI 会话导航' })).not.toBeInTheDocument()
    await user.click(within(workspace).getByRole('button', { name: '阅读' }))
    await waitFor(() => expect(screen.getByRole('button', { name: '展开侧栏' })).toHaveFocus())
  })

  it('阅读和 AI 双向切换时两栏同步淡入淡出，会话图标悬停有中间帧', async () => {
    api.get.mockResolvedValue({ preferences: preferencesFixture({ animation: 'full' }), revision: 0 })
    chat.List.mockResolvedValue([{ id: 'history', title: '已有会话' }])
    chat.Messages.mockResolvedValue([])
    const { container } = setup()
    const readingMain = await screen.findByRole('region', { name: '文章浏览' })
    fireEvent.click(screen.getByRole('button', { name: 'AI 助手' }))

    const main = await screen.findByRole('main', { name: 'AI 助手工作区' })
    const sidebar = screen.getByRole('complementary', { name: 'AI 会话导航' })
    const expectIntermediateOpacity = () => {
      for (const pane of [main, sidebar]) {
        expect(Number(pane.style.opacity)).toBeGreaterThan(0)
        expect(Number(pane.style.opacity)).toBeLessThan(1)
      }
    }
    await waitFor(expectIntermediateOpacity, { interval: 10 })
    expect(readingMain.closest('main')).toHaveAttribute('inert')
    await waitFor(() => expect(sidebar.style.opacity).toBe('1'))

    const row = await within(sidebar).findByRole('button', { name: '已有会话' })
    const glyph = row.querySelector('[data-meaning="message-square"]')
    fireEvent.mouseEnter(row)
    await waitFor(() => {
      const angle = glyph?.querySelector('svg')?.style.transform.match(/rotate\((-?[0-9.]+)deg\)/)?.[1]
      expect(Math.abs(Number(angle))).toBeGreaterThan(0.05)
    }, { interval: 10 })

    fireEvent.click(within(sidebar).getByRole('button', { name: '阅读' }))
    expect(main).toHaveAttribute('inert')
    expect(sidebar).toHaveAttribute('inert')
    expect(readingMain.closest('main')).not.toHaveAttribute('inert')
    await waitFor(expectIntermediateOpacity, { interval: 10 })
    await waitFor(() => expect(container.querySelector('.chat-workspace')).not.toBeInTheDocument())
    expect(sidebar).not.toBeInTheDocument()
  })

  it('新模型预填四档，删掉一档并添加自定义档位后可以重新编辑', async () => {
    const user = userEvent.setup()
    const providers = [{ id: 'provider', name: '本地服务', protocol: 'chat_completions', enabled: true, revision: 0 }]
    listAIConfiguration.mockResolvedValue({ providers, models: [], assignments: [] })
    saveAIModel.mockImplementation(async input => {
      const saved = { ...input, id: 'saved', revision: 1,
        metadata: { context: 256000, modalities: ['text', 'image'], source: 'Pi', promptPrice: 0, referencePrice: true },
      }
      listAIConfiguration.mockResolvedValue({ providers, models: [saved], assignments: [] })
      return saved
    })
    setup()
    await user.click(await screen.findByRole('button', { name: '打开设置' }))
    await user.click(await screen.findByRole('button', { name: '服务与模型' }))
    await user.click(await screen.findByRole('button', { name: '添加模型' }))
    const levels = within(screen.getByRole('group', { name: '思考档位' }))
    expect(levels.getAllByRole('textbox', { name: '参数值' }).map(input => (input as HTMLInputElement).value))
      .toEqual(['low', 'medium', 'high', 'xhigh'])
    await user.type(screen.getByRole('textbox', { name: '模型 ID' }), 'example-model')
    await user.type(screen.getByLabelText('显示名称', { selector: '#ai-model-display' }), '实验模型')
    await user.click(levels.getByRole('button', { name: '删除档位 中' }))
    await user.click(levels.getByRole('button', { name: '添加档位' }))
    await user.type(levels.getAllByRole('textbox', { name: '显示名称' }).at(-1)!, '深度')
    await user.type(levels.getAllByRole('textbox', { name: '参数值' }).at(-1)!, 'custom_max')
    await user.click(screen.getByRole('button', { name: '保存模型' }))
    expect(saveAIModel).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({
      reasoningLevels: [{ label: '低', value: 'low' }, { label: '高', value: 'high' },
        { label: '超高', value: 'xhigh' }, { label: '深度', value: 'custom_max' }],
    }))
    expect(screen.queryByText('已保存', { exact: true })).not.toBeInTheDocument()
    const models = screen.getByRole('region', { name: '模型' })
    expect(await within(models).findByText('实验模型')).toBeInTheDocument()
    const inputCapability = within(models).getByRole('img', { name: '图片' })
    expect(screen.queryByText('目录参考价 / 百万令牌')).not.toBeInTheDocument()
    await user.hover(inputCapability)
    expect(await screen.findByText('目录参考价 / 百万令牌')).toBeInTheDocument()
    expect(screen.getByText('Pi')).toBeInTheDocument()
    await user.click(await within(models).findByRole('button', { name: '编辑' }))
    expect(screen.getByRole('button', { name: '删除档位 深度' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '删除档位 中' })).not.toBeInTheDocument()
  })

  it('全局回复安全渲染 Markdown，聊天代码主题和文章主题独立保存', async () => {
    const user = userEvent.setup()
    api.get.mockResolvedValue({ preferences: preferencesFixture({ reader_code_theme: 'base16/monokai' }), revision: 0 })
    chat.List.mockResolvedValue([{ id: 'history', title: '历史会话' }])
    chat.Messages.mockResolvedValue([{
      id: 'reply', role: 'assistant', status: 'complete', modelName: '模型',
      text: ['## 小结', '', '| 输入 | 输出 |', '| --- | --- |', '| A | B |', '',
        '[文档](https://example.test/) [无效](javascript:alert%281%29)', '', '![配图](https://example.test/picture.png)',
        '', '<script>bad()</script>', '', '```js', 'const value = "<tag>";', '```'].join('\n'),
    }])
    const { container } = setup()
    await user.click(await screen.findByRole('button', { name: 'AI 助手' }))
    expect(await screen.findByRole('heading', { name: '小结' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: 'B' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '无效' })).not.toBeInTheDocument()
    expect(screen.queryByRole('img', { name: '配图' })).not.toBeInTheDocument()
    expect(container.querySelector('.chat-answer script')).toBeNull()
    await user.click(screen.getByRole('link', { name: '文档' }))
    expect(host.openURL).toHaveBeenCalledExactlyOnceWith('https://example.test/')
    await user.click(screen.getByRole('button', { name: '复制代码' }))
    expect(host.copy).toHaveBeenCalledExactlyOnceWith('const value = "<tag>";')
    await user.click(screen.getByRole('button', { name: 'AI 设置' }))
    await user.click(await screen.findByRole('button', { name: '聊天代码块主题' }))
    await user.type(screen.getByRole('combobox'), 'github-dark')
    await user.click(screen.getByRole('option', { name: 'Github Dark' }))
    await waitFor(() => expect(api.update).toHaveBeenLastCalledWith(expect.objectContaining({
      reader_code_theme: 'base16/monokai', chat_code_theme: 'github-dark',
    }), expect.any(Number)))
    await waitFor(() => expect(container.querySelector('.chat-session')).toHaveAttribute('data-code-theme', 'github-dark'))
  })

  it.each([['list', 'fill'], ['masonry', 'fill'], ['masonry', 'drawer']] as const)(
    '%s / %s 打开文章时从右侧推入，返回时向右推出并恢复焦点', async (article_view_mode, reader_layout) => {
    api.get.mockResolvedValue({ preferences: preferencesFixture({ article_view_mode, reader_layout, animation: 'full' }), revision: 0 })
    const { container } = setup(true)
    const trigger = await screen.findByRole('button', { name: '打开文章：测试文章 0' })
    fireEvent.click(trigger)
    const layer = container.querySelector<HTMLElement>(article_view_mode === 'list' ? '.reader-slot-content' : '.reader-overlay')!
    const offset = () => Number(layer.style.transform.match(/translateX\(([^)]+)%\)/)?.[1] ?? 0)
    const expectIntermediateFrame = () => {
      expect(offset()).toBeGreaterThan(0)
      expect(offset()).toBeLessThan(100)
      expect(layer.style.opacity).toBe('1')
    }
    expect(offset()).toBe(100)
    await waitFor(expectIntermediateFrame, { interval: 10 })
    const widthGetter = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetWidth')!.get!
    expect(vi.mocked(widthGetter).mock.contexts).toContain(layer)
    await waitFor(() => expect(offset()).toBe(0))
    const measure = vi.fn(() => article_view_mode === 'list' ? 400 : reader_layout === 'drawer' ? 800 : 1200)
    Object.defineProperty(layer, 'offsetWidth', { get: measure })
    fireEvent.click(within(layer).getByRole('button', { name: article_view_mode === 'list' ? '收起阅读面板' : '返回文章浏览' }))
    expect(layer).toHaveAttribute('inert')
    await waitFor(expectIntermediateFrame, { interval: 10 })
    expect(measure).toHaveBeenCalled()
    await waitFor(() => expect(layer).not.toBeInTheDocument())
    expect(trigger).toHaveFocus()
  })

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

  it('默认在后台运行，可保存退出应用或恢复后台运行', async () => {
    const user = userEvent.setup()
    setup()
    await user.click(await screen.findByRole('button', { name: '打开设置' }))
    const behavior = within(screen.getByRole('radiogroup', { name: '关闭窗口时' }))
    const background = behavior.getByRole('radio', { name: '在后台运行' })
    expect(background).toBeChecked()
    await user.click(behavior.getByRole('radio', { name: '退出应用' }))
    await waitFor(() => expect(api.update).toHaveBeenLastCalledWith(
      expect.objectContaining({ quit_on_window_close: true }), expect.any(Number),
    ))
    await user.click(background)
    await waitFor(() => expect(api.update).toHaveBeenLastCalledWith(
      expect.objectContaining({ quit_on_window_close: false }), expect.any(Number),
    ))
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

  it('阅读排版可搜索并保存 Base16 代码主题', async () => {
    const user = userEvent.setup()
    setup()
    await user.click(await screen.findByRole('button', { name: '打开设置' }))
    await user.click(screen.getByRole('button', { name: '阅读排版' }))
    const theme = screen.getByRole('button', { name: '代码块主题' })
    expect(theme).toHaveTextContent('跟随应用')
    await user.click(theme)
    await user.type(screen.getByRole('combobox'), 'base16/monokai')
    await user.click(screen.getByRole('option', { name: 'Base16 · Monokai' }))
    expect(theme).toHaveTextContent('Base16 · Monokai')
    await waitFor(() => expect(api.update).toHaveBeenLastCalledWith(
      expect.objectContaining({ reader_code_theme: 'base16/monokai' }), expect.any(Number),
    ))
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

  it('列表可保存 760 像素，缩窄窗口时适配可用宽度', async () => {
    let width = 1400
    vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockImplementation(function(this: HTMLElement) {
      return this.classList.contains('reading-workspace') ? width : 360
    })
    setup()
    const separator = await screen.findByRole('separator', { name: '调整文章列表宽度' })
    expect(separator).toHaveAttribute('aria-valuemax', '760')
    fireEvent.keyDown(separator, { key: 'End' })
    expect(separator).toHaveAttribute('aria-valuenow', '760')
    await waitFor(() => expect(api.update).toHaveBeenCalledWith(expect.objectContaining({ list_width: 760 }), expect.any(Number)))

    width = 1000
    act(() => geometry.resize())
    expect(separator).toHaveAttribute('aria-valuenow', '620')
    expect(useWorkspace.getState().preferences?.list_width).toBe(760)

    width = 450
    act(() => geometry.resize())
    expect(separator).toHaveAttribute('aria-disabled', 'true')
    fireEvent.keyDown(separator, { key: 'End' })
    expect(useWorkspace.getState().preferences?.list_width).toBe(760)
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

  it('工具栏重新提取时显示骨架屏，失败后恢复已提取正文', async () => {
    let complete!: (detail: Detail) => void
    let fail!: (error: Error) => void
    reader.extract
      .mockReturnValueOnce(new Promise<Detail>(resolve => { complete = resolve }))
      .mockReturnValueOnce(new Promise<Detail>((_resolve, reject) => { fail = reject }))
    reader.get.mockResolvedValue({article_id:'a-0',html:'',text:'订阅正文',mode:'feed',has_cached:false,has_original:true})
    const {client} = setup(true)
    const items = articleFixtures(20).map(article => ({...article, contentText:'', url:'https://example.test/article'}))
    client.setQueryData(articleQueryKey, initialLibraryData({items, hasMore:false}))
    await userEvent.click(await screen.findByRole('button',{name:'打开文章：测试文章 0'}))
    const panel = screen.getByRole('complementary',{name:'阅读面板'})
    const refresh = within(panel).getByRole('button',{name:'重新提取正文'})
    await within(panel).findByText('订阅正文')
    await userEvent.click(refresh)
    expect(refresh).toBeDisabled()
    expect(panel.querySelector('[data-slot="skeleton"]')).toHaveClass('animate-pulse')
    expect(within(panel).queryByText('订阅正文')).not.toBeInTheDocument()
    complete({article_id:'a-0',html:'',text:'已提取正文',mode:'reader',has_cached:true,has_original:true})
    expect(await within(panel).findByText('已提取正文')).toBeInTheDocument()

    await userEvent.click(refresh)
    expect(panel.querySelector('[data-slot="skeleton"]')).toBeInTheDocument()
    expect(within(panel).queryByText('已提取正文')).not.toBeInTheDocument()
    fail(new Error('重新提取失败'))
    expect(await within(panel).findByRole('alert')).toHaveTextContent('重新提取失败')
    expect(within(panel).getByText('已提取正文')).toBeInTheDocument()
    expect(panel.querySelector('[data-slot="skeleton"]')).not.toBeInTheDocument()
    expect(refresh).toBeEnabled()
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
    await user.click(within(viewer).getByRole('button', { name: '退出看图' }))
    expect(viewer).not.toBeInTheDocument()
    expect(screen.getByRole('complementary', { name: '阅读面板' })).toContainElement(image)
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

  it('文章推送规则可同时选择多个已验证渠道', async () => {
    const user = userEvent.setup()
    notificationAPI.ListChannels.mockResolvedValue([
      { id: 'telegram', name: 'Telegram 测试', enabled: true, verified: true },
      { id: 'slack', name: 'Slack 测试', enabled: true, verified: true },
    ])
    setup()
    await user.click(await screen.findByRole('button', { name: '打开设置' }))
    await user.click(await screen.findByRole('button', { name: '通知' }))
    const add = await screen.findByRole('button', { name: '添加规则' })
    expect(screen.queryByRole('region', { name: '定时任务' })).not.toBeInTheDocument()
    await waitFor(() => expect(add).toBeEnabled())
    await user.click(add)
    await user.type(screen.getByRole('textbox', { name: '规则名称' }), '科技文章')
    await user.click(screen.getByRole('button', { name: '接收渠道' }))
    await user.click(await screen.findByText('Telegram 测试', { selector: '[cmdk-item] span.truncate' }))
    await user.click(await screen.findByText('Slack 测试', { selector: '[cmdk-item] span.truncate' }))
    await user.keyboard('{Escape}')
    await user.click(screen.getByRole('button', { name: '保存规则' }))
    await waitFor(() => expect(notificationAPI.SaveRule).toHaveBeenCalledWith(expect.objectContaining({
      name: '科技文章', mode: 'instant', channels: ['telegram', 'slack'],
    })))
  })

  it('Telegram 通知可同时保存用户 ID 和群会话 ID', async () => {
    const user = userEvent.setup()
    setup()
    await user.click(await screen.findByRole('button', { name: '打开设置' }))
    await user.click(await screen.findByRole('button', { name: '通知' }))
    await user.click(await screen.findByRole('button', { name: '添加渠道' }))
    await user.type(screen.getByRole('textbox', { name: '渠道名称' }), 'Telegram 私聊')
    await user.type(screen.getByLabelText('Bot Token'), '1234:local-test')
    await user.click(screen.getByRole('button', { name: '下一步' }))
    await user.click(screen.getByRole('button', { name: '跳过配对' }))
    await user.type(screen.getByRole('textbox', { name: 'Telegram 用户 ID 或会话 ID' }), '123456{Enter}-100123456')
    await user.click(screen.getByRole('button', { name: '保存渠道' }))
    await waitFor(() => expect(notificationAPI.SaveChannel).toHaveBeenCalledWith(expect.objectContaining({
      platform: 'telegram', targets: ['123456', '-100123456'],
    })))
  })

  it('QQ 配对需要确认收件目标，最后才保存渠道', async () => {
    const pair = { id: 'qq-pair', state: 'listening', bot_name: '文章通知机器人', targets: ['c2c:friend', 'group:team', 'channel:42'] }
    const waiting = { ...pair, targets: [] }
    notificationAPI.StartRecipientPairing.mockResolvedValue(waiting)
    notificationAPI.RecipientPairingStatus.mockResolvedValue(waiting)
    const user = userEvent.setup()
    setup()
    await user.click(await screen.findByRole('button', { name: '打开设置' }))
    await user.click(await screen.findByRole('button', { name: '通知' }))
    await user.click(await screen.findByRole('button', { name: '添加渠道' }))
    await user.type(screen.getByRole('textbox', { name: '渠道名称' }), 'QQ 通知')
    await user.click(screen.getByRole('button', { name: '平台' }))
    await user.click(await screen.findByRole('option', { name: 'QQ' }))
    await user.click(screen.getByRole('button', { name: 'QQ 接入环境' }))
    await user.click(await screen.findByRole('option', { name: '沙箱（未上线测试）' }))
    await user.type(screen.getByLabelText('AppID'), '1234')
    await user.type(screen.getByLabelText('AppSecret'), 'example-secret')
    await user.click(screen.getByRole('button', { name: '下一步' }))
    await user.click(screen.getByRole('button', { name: '开始配对' }))
    expect(notificationAPI.StartRecipientPairing).toHaveBeenCalledWith(expect.objectContaining({ mode: 'sandbox' }), '')
    expect(await screen.findByText('QQ 网关已连接，尚未收到聊天消息')).toBeInTheDocument()
    expect(screen.getByText('QQ 机器人 · 文章通知机器人')).toBeInTheDocument()
    notificationAPI.RecipientPairingStatus.mockResolvedValue({ ...waiting, received_message: true })
    expect(await screen.findByText('已收到 QQ 消息。请单独发送「配对」。', {}, { timeout: 2000 })).toBeInTheDocument()
    notificationAPI.RecipientPairingStatus.mockResolvedValue({ ...pair, received_message: true })
    const group = await screen.findByRole('checkbox', { name: 'group:team' }, { timeout: 2000 })
    expect(group).not.toBeChecked()
    await user.click(group)
    await user.click(screen.getByRole('button', { name: '下一步' }))
    expect(notificationAPI.CancelRecipientPairing).toHaveBeenCalledWith(pair.id)
    expect(notificationAPI.SaveChannel).not.toHaveBeenCalled()
    expect(screen.getByRole('textbox', { name: 'QQ 私聊 OpenID、群 OpenID 或子频道 ID' })).toHaveValue('group:team')
    await user.click(screen.getByRole('button', { name: '保存渠道' }))
    await waitFor(() => expect(notificationAPI.SaveChannel).toHaveBeenCalledWith(expect.objectContaining({
      platform: 'qq', mode: 'sandbox', targets: ['group:team'],
    })))
  })

  it('从文章列表单选渠道，从阅读菜单发送到全部可用渠道', async () => {
    const user = userEvent.setup()
    notificationAPI.ListChannels.mockResolvedValue([
      { id: 'telegram', name: 'Telegram 通知', enabled: true, verified: true },
      { id: 'wechat', name: '微信通知', enabled: true, verified: true },
      { id: 'unverified', name: '待测试渠道', enabled: true, verified: false },
    ])
    const { client } = setup(true)
    client.setQueryData(articleQueryKey, initialLibraryData({
      items: [{ ...articleFixtures(1)[0], url: 'https://example.com/story' }], hasMore: false,
    }))
    const openArticle = await screen.findByRole('button', { name: '打开文章：测试文章 0' })
    const listSend = screen.getByRole('button', { name: '发送到' })
    await user.click(listSend)
    const first = await screen.findByRole('dialog', { name: '发送到' })
    const dialog = within(first)
    await user.click(await dialog.findByRole('button', { name: '通知渠道' }))
    await user.click(await screen.findByRole('option', { name: '微信通知' }))
    expect(screen.queryByRole('option', { name: '待测试渠道' })).not.toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(dialog.getByRole('checkbox', { name: '摘要' })).toBeChecked()
    await user.click(dialog.getByRole('button', { name: '发送' }))
    await waitFor(() => expect(listSend).toHaveFocus())
    expect(notificationAPI.SendArticle).toHaveBeenLastCalledWith({
      article_id: 'a-0', channel_ids: ['wechat'], title: true, summary: true, link: true, full_text: false, summary_limit: 50, message_limit: 1500, split_articles: false,
    })

    await user.click(openArticle)
    const more = within(await screen.findByRole('complementary', { name: '阅读面板' })).getByRole('button', { name: '更多' })
    await user.click(more)
    await user.click(await screen.findByRole('menuitem', { name: '发送到…' }))
    const second = within(await screen.findByRole('dialog', { name: '发送到' }))
    await user.click(await second.findByRole('checkbox', { name: '全部渠道' }))
    await user.click(second.getByRole('checkbox', { name: '标题' }))
    await user.click(second.getByRole('checkbox', { name: '链接' }))
    await user.click(second.getByRole('checkbox', { name: '摘要' }))
    await user.click(second.getByRole('checkbox', { name: '全文' }))
    await user.click(second.getByRole('button', { name: '发送' }))
    await waitFor(() => expect(more).toHaveFocus())
    expect(notificationAPI.SendArticle).toHaveBeenLastCalledWith({
      article_id: 'a-0', channel_ids: ['telegram', 'wechat'], title: false, summary: false, link: false, full_text: true, summary_limit: 50, message_limit: 1500, split_articles: false,
    })
  })

  it('列表工具栏按当前顺序发送前几篇并使用通知设置的摘要字数', async () => {
    const user = userEvent.setup()
    notificationAPI.ListChannels.mockResolvedValue([{ id: 'wechat', name: '微信通知', enabled: true, verified: true }])
    setup(true)
    const toolbar = within(await screen.findByRole('group', { name: '文章操作' }))
    const trigger = toolbar.getByRole('button', { name: '发送列表文章' })
    await user.click(trigger)
    const defaultDialog = within(await screen.findByRole('dialog', { name: '发送到' }))
    expect(defaultDialog.getByRole('spinbutton', { name: '发送前几篇' })).toHaveValue(10)
    await user.keyboard('{Escape}')
    await waitFor(() => expect(trigger).toHaveFocus())
    const settingsTrigger = screen.getByRole('button', { name: '打开设置' })
    await act(async () => {
      await user.click(settingsTrigger)
      await import('@/features/settings/SettingsDialog')
    })
    const settings = within(await screen.findByRole('dialog', { name: '设置' }))
    await user.click(settings.getByRole('button', { name: '通知' }))
    const count = await screen.findByRole('spinbutton', { name: '默认发送条数' })
    const summary = screen.getByRole('spinbutton', { name: '默认摘要字数' })
    expect(summary).toHaveValue(50)
    await user.clear(count)
    await user.type(count, '3')
    await user.clear(summary)
    await user.type(summary, '25{Enter}')
    await waitFor(() => expect(api.update).toHaveBeenLastCalledWith(expect.objectContaining({
      article_share_count: 3, article_share_summary_limit: 25,
    }), expect.any(Number)))
    await user.keyboard('{Escape}')
    await waitFor(() => expect(settingsTrigger).toHaveFocus())
    await user.click(trigger)
    const share = within(await screen.findByRole('dialog', { name: '发送到' }))
    expect(share.getByRole('spinbutton', { name: '发送前几篇' })).toHaveValue(3)
    for (const field of ['标题', '链接', '摘要']) {
      expect(share.getByRole('checkbox', { name: field })).toBeChecked()
    }
    const split = share.getByRole('switch', { name: '逐篇发送' })
    const limit = share.getByRole('spinbutton', { name: '单条消息字数上限' })
    expect(split).not.toBeChecked()
    expect(limit).toHaveValue(1500)
    await user.click(split)
    await user.clear(limit)
    await user.type(limit, '900')
    await user.click(await share.findByRole('checkbox', { name: '全部渠道' }))
    await user.click(share.getByRole('button', { name: '发送' }))
    await waitFor(() => expect(notificationAPI.SendArticle).toHaveBeenLastCalledWith({
      article_ids: ['a-0', 'a-1', 'a-2'], channel_ids: ['wechat'],
      title: true, link: true, summary: true, full_text: false, summary_limit: 25, message_limit: 900, split_articles: true,
    }))
  })

  it('微信绑定失败后显示具体原因，并停止自动轮询', async () => {
    notificationAPI.StartWeChatBinding.mockResolvedValue({ id: 'wechat-qr', status: 'wait' })
    notificationAPI.PollWeChatBinding.mockImplementation(() => Object.assign(
      Promise.reject(new Error('无法读取微信私聊消息')), { cancel: vi.fn() },
    ))
    const user = userEvent.setup()
    setup()
    await user.click(await screen.findByRole('button', { name: '打开设置' }))
    await user.click(await screen.findByRole('button', { name: '通知' }))
    await user.click(await screen.findByRole('button', { name: '添加渠道' }))
    await user.type(screen.getByRole('textbox', { name: '渠道名称' }), '微信测试')
    await user.click(screen.getByRole('button', { name: '平台' }))
    await user.click(await screen.findByRole('option', { name: '微信' }))
    await user.click(screen.getByRole('button', { name: '生成微信二维码' }))
    expect(await screen.findByText('无法读取微信私聊消息')).toBeInTheDocument()
    expect(screen.queryByText('用微信扫描二维码')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '重新生成二维码' })).toBeEnabled()
    await act(() => new Promise(resolve => window.setTimeout(resolve, 1350)))
    expect(notificationAPI.PollWeChatBinding).toHaveBeenCalledTimes(1)
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
    await user.keyboard('文章分类')
    const categoryOption = screen.getByRole('option', { name: '归类' })
    expect(categoryOption.querySelector('[data-meaning=folder]')).toBeInTheDocument()
    expect(categoryOption).toHaveAttribute('aria-checked', 'false')
    await user.click(categoryOption)
    expect(categoryOption).toHaveAttribute('aria-checked', 'true')
    await user.clear(screen.getByRole('combobox'))
    await user.keyboard('收藏')
    await user.click(screen.getByRole('option',{name:'收藏'}))
    await user.keyboard('{Escape}')
    await user.keyboard('{Escape}')
    expect(pane.getByRole('button',{name:'归类'}).querySelector('[data-meaning=folder]')).toBeInTheDocument()
    expect(pane.queryByRole('button',{name:'收藏'})).not.toBeInTheDocument()
    await waitFor(() => expect(api.update).toHaveBeenLastCalledWith(expect.objectContaining({
      reader_toolbar_pinned: ['refresh', 'translate', 'ai', 'categories'],
    }), expect.any(Number)))
  })

  it.each(['full', 'off'] as const)('从瀑布流阅读返回时清除所选正文（%s）', async animation => {
    const user = userEvent.setup()
    api.get.mockResolvedValue({ preferences: preferencesFixture({ article_view_mode: 'masonry', reader_layout: 'fill', animation }), revision: 0 })
    reader.get.mockResolvedValue({ article_id: 'a-0', html: '<p>返回前选中的正文文字</p>', text: '返回前选中的正文文字', mode: 'feed',
      has_cached: false, has_original: false })
    const { client } = setup()
    const articles = articleFixtures(1).map(article => ({ ...article, contentText: '' }))
    act(() => client.setQueryData(articleQueryKey, initialLibraryData({ items: articles, hasMore: false })))
    await user.click(await screen.findByRole('button', { name: '打开文章：测试文章 0' }))
    const text = await screen.findByText('返回前选中的正文文字')
    const selection = window.getSelection()!
    const range = document.createRange()
    range.selectNodeContents(text)
    const previousRects = Object.getOwnPropertyDescriptor(Range.prototype, 'getClientRects')
    Object.defineProperty(Range.prototype, 'getClientRects', { configurable: true, value: () => [new DOMRect(30, 40, 50, 16)] })
    const prose = text.closest('.reader-content')!
    Object.defineProperties(prose, { clientWidth: { value: 100 }, clientHeight: { value: 100 } })
    try {
      selection.removeAllRanges()
      selection.addRange(range)
      fireEvent(document, new Event('selectionchange'))
      await waitFor(() => expect(document.querySelector('.text-selection > span')).toBeInTheDocument())
      fireEvent.click(screen.getByRole('button', { name: '返回文章浏览' }))
      expect(selection.isCollapsed).toBe(true)
      expect(document.querySelector('.text-selection > span')).not.toBeInTheDocument()
      expect(document.documentElement).not.toHaveAttribute('data-text-selection-painted')
    } finally {
      selection.removeAllRanges()
      if (previousRects) Object.defineProperty(Range.prototype, 'getClientRects', previousRects)
      else Reflect.deleteProperty(Range.prototype, 'getClientRects')
    }
  })

  it('正文右键在浏览器清除选区后仍保留底色和复制内容', async () => {
    const user = userEvent.setup()
    reader.get.mockResolvedValue({ article_id: 'a-0', html: '<p>用于验证的正文文字</p>', text: '用于验证的正文文字', mode: 'feed',
      has_cached: false, has_original: false })
    const { client } = setup()
    const articles = articleFixtures(1).map(article => ({ ...article, contentText: '' }))
    act(() => client.setQueryData(articleQueryKey, initialLibraryData({ items: articles, hasMore: false })))
    await user.click(await screen.findByRole('button', { name: '打开文章：测试文章 0' }))
    const paragraph = await screen.findByText('用于验证的正文文字')
    const selection = window.getSelection()!
    const range = document.createRange()
    range.setStart(paragraph.firstChild!, 2)
    range.setEnd(paragraph.firstChild!, 4)
    selection.removeAllRanges()
    selection.addRange(range)
    const previousRects = Object.getOwnPropertyDescriptor(Range.prototype, 'getClientRects')
    const previousHitTest = Object.getOwnPropertyDescriptor(document, 'elementFromPoint')
    Object.defineProperty(Range.prototype, 'getClientRects', {
      configurable: true, value: () => [new DOMRect(30, 40, 50, 16), new DOMRect(10, 64, 35, 16)],
    })
    Object.defineProperty(document, 'elementFromPoint', { configurable: true, value: () => paragraph })
    try {
      const prose = paragraph.closest('.reader-content')!
      Object.defineProperties(prose, { clientWidth: { value: 100 }, clientHeight: { value: 100 } })
      const painted = () => [...document.querySelectorAll('.text-selection > span')]
      fireEvent(document, new Event('selectionchange'))
      await waitFor(() => expect(painted()).toHaveLength(2))
      expect(painted()[0]).toHaveStyle({ top: '40px', height: '20px' })
      expect(painted()[1]).toHaveStyle({ top: '60px', height: '20px' })
      const beforeMenu = painted().map(line => line.getAttribute('style'))
      fireEvent.contextMenu(paragraph)
      selection.removeAllRanges()
      fireEvent(document, new Event('selectionchange'))
      const copy = await screen.findByRole('menuitem', { name: '复制' })
      expect(painted().map(line => line.getAttribute('style'))).toEqual(beforeMenu)
      await user.click(copy)
      expect(host.copy).toHaveBeenCalledWith('验证')
      expect(painted()).toHaveLength(0)
    } finally {
      selection.removeAllRanges()
      if (previousRects) Object.defineProperty(Range.prototype, 'getClientRects', previousRects)
      else Reflect.deleteProperty(Range.prototype, 'getClientRects')
      if (previousHitTest) Object.defineProperty(document, 'elementFromPoint', previousHitTest)
      else Reflect.deleteProperty(document, 'elementFromPoint')
    }
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
  await user.click(settings.getByRole('button', { name: '复制版本' }))
  expect(host.copy).toHaveBeenCalledWith('折栈 Fealdhord 0.1.0-alpha.15')
  const versionToast = (await screen.findByText('已复制版本信息')).closest<HTMLElement>('[data-slot="toast"]')!
  // JSDOM 不加载应用样式，显式模拟 Toast 卡片的 pointer-events-auto。
  versionToast.style.pointerEvents = 'auto'
  await user.hover(versionToast)
  await user.click(within(versionToast).getByRole('button', { name: '关闭通知' }))
  await waitFor(() => expect(screen.queryByText('已复制版本信息')).not.toBeInTheDocument())
  expect(screen.getByRole('dialog', { name: '设置' })).toBeInTheDocument()
  expect(host.openURL).not.toHaveBeenCalled()
  await user.click(settings.getByRole('link', { name: '查看源码' }))
  expect(host.openURL).toHaveBeenCalledWith('https://github.com/coolerks/Fealdhord')
  const developerMode = settings.getByRole('switch', { name: '开发者模式' })
  expect(developerMode).not.toBeChecked()
  await user.click(developerMode)
  await waitFor(() => expect(api.update).toHaveBeenCalledWith(expect.objectContaining({ developer_mode: true }), expect.any(Number)))
  await user.keyboard('{Escape}')
  await waitFor(() => expect(screen.getByRole('button', { name: '打开设置' })).toHaveFocus())

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
