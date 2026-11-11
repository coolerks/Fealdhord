import { beforeEach, expect, it, vi } from 'vitest'
import { useWorkspace } from '@/stores/workspace'
import { preferencesFixture } from '@/test/fixtures'
import { notify } from './notifications'

const { add, send } = vi.hoisted(() => ({ add: vi.fn(), send: vi.fn() }))
vi.mock('@/components/ui/toast-manager', () => ({ toast: { add } }))
vi.mock('../../bindings/github.com/coolerks/Fealdhord/internal/desktop/notifier', () => ({ Send: send }))

beforeEach(() => {
  vi.resetAllMocks()
  useWorkspace.setState({ preferences: preferencesFixture() })
})

it('订阅结果默认常驻并复用同一条通知，普通通知沿用自动关闭时间', async () => {
  await notify('刷新完成', '2/2 个订阅', false, 'subscription-refresh')
  expect(add).toHaveBeenLastCalledWith(expect.objectContaining({ id: 'subscription-refresh', timeout: 0 }))

  await notify('刷新完成', '3/3 个订阅', false, 'subscription-refresh')
  expect(add).toHaveBeenLastCalledWith(expect.objectContaining({ id: 'subscription-refresh', timeout: 0, description: '3/3 个订阅' }))

  await notify('连接成功')
  expect(add).toHaveBeenLastCalledWith(expect.objectContaining({ id: undefined, timeout: undefined }))
})

it('开启自动收起后，同一条订阅通知恢复默认关闭时间', async () => {
  useWorkspace.setState({ preferences: preferencesFixture({ refresh_toast_auto_dismiss: true }) })
  await notify('刷新完成', undefined, false, 'subscription-refresh')
  expect(add).toHaveBeenLastCalledWith(expect.objectContaining({ id: 'subscription-refresh', timeout: undefined }))
})

it('系统通知发送成功不显示 Toast，失败回退时沿用订阅通知设置', async () => {
  useWorkspace.setState({ preferences: preferencesFixture({ notification_mode: 'native' }) })
  await notify('刷新完成', '1/1 个订阅', false, 'subscription-refresh')
  expect(send).toHaveBeenCalledWith(expect.stringMatching(/^fealdhord-/), '刷新完成', '1/1 个订阅')
  expect(add).not.toHaveBeenCalled()

  send.mockRejectedValueOnce(new Error('不可用'))
  await notify('刷新结束', '1 个失败', true, 'subscription-refresh')
  expect(add).toHaveBeenCalledWith(expect.objectContaining({
    type: 'error',
    timeout: 0,
    description: '1 个失败 · 系统通知无法送达',
  }))
})
