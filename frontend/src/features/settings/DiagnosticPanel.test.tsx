import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { OperationSnapshot } from '../../lib/api'

const api = vi.hoisted(() => ({ info: vi.fn(), start: vi.fn(), cancel: vi.fn(), snapshot: vi.fn(), subscribe: vi.fn() }))
vi.mock('../../lib/api', () => ({ desktopAPI: api }))
import App from './DiagnosticPanel'

const running: OperationSnapshot = { operation_id: 'probe_1', status: 'running', completed: 0, total: 20, sequence: 1 }
let receive: (snapshot: OperationSnapshot) => void
let unsubscribe: ReturnType<typeof vi.fn>

beforeEach(() => {
  vi.resetAllMocks()
  unsubscribe = vi.fn()
  api.info.mockResolvedValue({ name: '折栈 Fealdhord', version: '0.1.0-dev', wails_version: '3.0.0', platform: 'darwin', architecture: 'arm64' })
  api.subscribe.mockImplementation(listener => { receive = listener; return unsubscribe })
  api.start.mockResolvedValue(running)
  api.cancel.mockResolvedValue(running)
  api.snapshot.mockResolvedValue(running)
})

describe('桌面工程调用链', () => {
  it('读取真实边界形状的应用信息后允许开始诊断', async () => {
    render(<App />)
    expect(await screen.findByText('桌面服务已连接')).toBeInTheDocument()
    expect(screen.getByText('darwin / arm64')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '开始检查' }))
    expect(api.start).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('status')).toHaveTextContent('正在检查')
    expect(screen.getByRole('button', { name: '开始检查' })).toBeDisabled()
  })

  it('取消后使用终态，忽略更旧和其他操作的事件', async () => {
    render(<App />)
    await screen.findByText('桌面服务已连接')
    await userEvent.click(screen.getByRole('button', { name: '开始检查' }))
    await userEvent.click(screen.getByRole('button', { name: '取消检查' }))
    expect(api.cancel).toHaveBeenCalledWith('probe_1')
    act(() => { receive({ ...running, status: 'cancelled', sequence: 3 }) })
    act(() => { receive({ ...running, completed: 1, sequence: 2 }) })
    act(() => { receive({ ...running, operation_id: 'old', sequence: 99 }) })
    expect(screen.getByRole('status')).toHaveTextContent('已取消')
    expect(screen.getByRole('button', { name: '开始检查' })).toBeEnabled()
  })

  it('保留 Start 返回前已经到达的进度', async () => {
    let resolveStart!: (value: OperationSnapshot) => void
    api.start.mockImplementation(() => new Promise<OperationSnapshot>(resolve => { resolveStart = resolve }))
    render(<App />)
    await screen.findByText('桌面服务已连接')
    await userEvent.click(screen.getByRole('button', { name: '开始检查' }))
    act(() => { receive({ ...running, completed: 2, sequence: 3 }) })
    act(() => { receive({ ...running, completed: 1, sequence: 2 }) })
    await act(async () => { resolveStart(running) })
    expect(screen.getByText('2 / 20')).toBeInTheDocument()
  })

  it('启动失败显示可理解错误并恢复按钮', async () => {
    api.start.mockRejectedValue(new Error('应用正在关闭，请稍后重试。'))
    render(<App />)
    await screen.findByText('桌面服务已连接')
    await userEvent.click(screen.getByRole('button', { name: '开始检查' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('应用正在关闭')
    await waitFor(() => { expect(screen.getByRole('button', { name: '开始检查' })).toBeEnabled() })
  })

  it('卸载清理事件订阅', () => {
    const { unmount } = render(<App />)
    unmount()
    expect(unsubscribe).toHaveBeenCalledOnce()
  })
})
