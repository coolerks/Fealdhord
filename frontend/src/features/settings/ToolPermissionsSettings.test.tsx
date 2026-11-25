import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { ToolPermissionsSettings } from './ToolPermissionsSettings'
import type { ToolPermission, ToolPermissionChange } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/models'

const apiMock = vi.hoisted(() => ({
  ToolPermissions: vi.fn(),
  SaveToolPermission: vi.fn(),
  SaveToolPermissions: vi.fn(),
  ShellSettings: vi.fn(),
  SaveDefaultShell: vi.fn(),
}))

vi.mock('../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api', () => apiMock)

function renderSettings() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <TooltipProvider><ToolPermissionsSettings /></TooltipProvider>
    </QueryClientProvider>,
  )
}

describe('ToolPermissionsSettings', () => {
  let stored: ToolPermission[]

  beforeEach(() => {
    vi.clearAllMocks()
    const shellSettings = {
      defaultShell: 'auto',
      options: [
        { id: 'auto', label: 'sh', path: '/bin/sh' },
        { id: 'bash', label: 'Bash', path: '/bin/bash' },
        { id: 'fish', label: 'Fish' },
      ],
    }
    apiMock.ShellSettings.mockResolvedValue(shellSettings)
    apiMock.SaveDefaultShell.mockImplementation(async (defaultShell: string) => ({ ...shellSettings, defaultShell }))
    stored = [
      { id: 'builtin:article.search', name: 'article.search', description: '搜索本地文章', available: true, requiresApproval: false, mode: '' },
      { id: 'builtin:feed.add', name: 'feed.add', description: '添加新订阅源', available: true, requiresApproval: true, mode: '' },
      { id: 'mcp:alpha:1', name: 'get_quote', description: '查询实时行情', connection: 'Alpha', available: true, requiresApproval: true, mode: '' },
      { id: 'mcp:alpha:2', name: 'list_positions', description: '查看持仓列表', connection: 'Alpha', available: true, requiresApproval: true, mode: 'bypass' },
      { id: 'mcp:legacy:1', name: 'old_tool', connection: 'Legacy', available: false, requiresApproval: true, mode: 'deny' },
    ]
    apiMock.ToolPermissions.mockImplementation(async () => stored.map(item => ({ ...item })))
    apiMock.SaveToolPermission.mockImplementation(async (id: string, mode: string) => {
      const target = stored.find(item => item.id === id)
      if (!target) return
      target.mode = mode
      if (!target.available && !mode) {
        stored = stored.filter(item => item.id !== id)
      }
    })
    apiMock.SaveToolPermissions.mockImplementation(async (changes: ToolPermissionChange[]) => {
      for (const change of changes) {
        const target = stored.find(item => item.id === change.id)
        if (!target) continue
        target.mode = change.mode
      }
      stored = stored.filter(item => item.available || Boolean(item.mode))
    })
  })

  it('可搜索并用键盘保存已安装的默认 Shell', async () => {
    const user = userEvent.setup()
    renderSettings()

    const shell = await screen.findByRole('button', { name: '默认 Shell' })
    await waitFor(() => expect(shell).toHaveTextContent('应用默认（sh）'))
    await user.click(shell)
    const input = screen.getByRole('combobox', { name: '搜索默认 Shell' })
    expect(input).toHaveFocus()
    expect(screen.getByRole('option', { name: 'Fish · 不可用' })).toHaveAttribute('aria-disabled', 'true')
    await user.type(input, '/bin/bash')
    await user.keyboard('{Enter}')

    await waitFor(() => expect(apiMock.SaveDefaultShell).toHaveBeenCalledWith('bash'))
    await waitFor(() => expect(shell).toHaveTextContent('Bash'))
  })

  it('默认折叠分组，展开后通过四段切换修改单条工具权限', async () => {
    const user = userEvent.setup()
    renderSettings()

    const articleGroup = await screen.findByRole('button', { name: /^文章/ })
    expect(articleGroup).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByText('搜索文章')).not.toBeInTheDocument()

    await user.click(articleGroup)
    expect(articleGroup).toHaveAttribute('aria-expanded', 'true')
    const searchControl = screen.getByRole('radiogroup', { name: '搜索文章 调用权限' })
    await user.click(within(searchControl).getByRole('radio', { name: '审批' }))

    await waitFor(() => expect(apiMock.SaveToolPermission).toHaveBeenCalledWith('builtin:article.search', 'ask'))
    expect(within(searchControl).getByRole('radio', { name: '审批' })).toHaveAttribute('aria-checked', 'true')
  })

  it('搜索关键词自动展开匹配分组，并支持按已自定义筛选与清除不可用残留规则', async () => {
    const user = userEvent.setup()
    renderSettings()

    const searchInput = await screen.findByRole('textbox', { name: '筛选工具' })
    await user.type(searchInput, '行情')
    expect(await screen.findByText('get_quote')).toBeInTheDocument()
    expect(screen.queryByText('搜索文章')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '清空搜索' }))
    const statusFilter = screen.getByRole('radiogroup', { name: '筛选权限状态' })
    await user.click(within(statusFilter).getByRole('radio', { name: /已自定义 2/ }))

    expect(await screen.findByText('list_positions')).toBeInTheDocument()
    expect(screen.getByText('old_tool')).toBeInTheDocument()
    expect(screen.queryByText('get_quote')).not.toBeInTheDocument()

    const legacyControl = screen.getByRole('radiogroup', { name: 'old_tool 调用权限' })
    expect(within(legacyControl).getByRole('radio', { name: '放行' })).toBeDisabled()
    await user.click(within(legacyControl).getByRole('radio', { name: '默认' }))
    await waitFor(() => expect(apiMock.SaveToolPermission).toHaveBeenCalledWith('mcp:legacy:1', ''))
  })

  it('支持整组批量设置与恢复全部默认二次确认', async () => {
    const user = userEvent.setup()
    renderSettings()

    const batchTrigger = await screen.findByRole('button', { name: '批量设置 Alpha' })
    await user.click(batchTrigger)
    await user.click(await screen.findByRole('menuitem', { name: '整组拒绝' }))

    await waitFor(() => expect(apiMock.SaveToolPermissions).toHaveBeenCalledWith([
      { id: 'mcp:alpha:1', mode: 'deny' },
      { id: 'mcp:alpha:2', mode: 'deny' },
    ]))

    await user.click(screen.getByRole('button', { name: '恢复全部默认' }))
    const confirm = await screen.findByRole('dialog', { name: '恢复全部工具为默认权限？' })
    await user.click(within(confirm).getByRole('button', { name: '确认恢复' }))

    await waitFor(() => expect(apiMock.SaveToolPermissions).toHaveBeenLastCalledWith([
      { id: 'mcp:alpha:1', mode: '' },
      { id: 'mcp:alpha:2', mode: '' },
      { id: 'mcp:legacy:1', mode: '' },
    ]))
  })
})
