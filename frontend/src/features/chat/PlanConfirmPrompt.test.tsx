import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { PlanConfirmPrompt } from './PlanConfirmPrompt'

const resolvePlanMock = vi.fn().mockResolvedValue(true)

vi.mock('../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api', () => ({
  PendingPlans: vi.fn().mockResolvedValue([]),
  ResolvePlan: (...args: unknown[]) => resolvePlanMock(...args),
}))

describe('PlanConfirmPrompt', () => {
  const pending = {
    id: 'plan-1',
    conversationId: 'conv-1',
    title: '重构聊天模块',
    summary: '拆分渲染器并补充测试',
    steps: ['新增组件', '补充单测'],
  }

  function renderPrompt(onDecision = vi.fn()) {
    const client = new QueryClient()
    return render(
      <QueryClientProvider client={client}>
        <PlanConfirmPrompt pending={pending} onDecision={onDecision} onStop={vi.fn()} stopping={false} />
      </QueryClientProvider>,
    )
  }

  it('approves plan on confirm click', async () => {
    resolvePlanMock.mockClear()
    const onDecision = vi.fn()
    const user = userEvent.setup()
    renderPrompt(onDecision)

    expect(screen.getByText('重构聊天模块')).toBeInTheDocument()
    expect(screen.getByText('新增组件')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '确认并执行' }))
    await waitFor(() => {
      expect(resolvePlanMock).toHaveBeenCalledWith('plan-1', 'approve', '')
      expect(onDecision).toHaveBeenCalledWith('approve')
    })
  })

  it('rejects plan when user declines', async () => {
    resolvePlanMock.mockClear()
    const onDecision = vi.fn()
    const user = userEvent.setup()
    renderPrompt(onDecision)

    await user.click(screen.getByRole('button', { name: '不执行计划' }))
    await waitFor(() => {
      expect(resolvePlanMock).toHaveBeenCalledWith('plan-1', 'reject', '')
      expect(onDecision).toHaveBeenCalledWith('reject')
    })
  })

  it('submits revision feedback', async () => {
    resolvePlanMock.mockClear()
    const onDecision = vi.fn()
    const user = userEvent.setup()
    renderPrompt(onDecision)

    await user.click(screen.getByRole('button', { name: '修改计划' }))
    await user.type(screen.getByRole('textbox', { name: '修改意见' }), '加上回滚步骤{Enter}')
    await waitFor(() => {
      expect(resolvePlanMock).toHaveBeenCalledWith('plan-1', 'revise', '加上回滚步骤')
      expect(onDecision).toHaveBeenCalledWith('revise')
    })
  })
})
