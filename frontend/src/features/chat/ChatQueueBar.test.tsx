import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { ChatQueueBar } from './ChatQueueBar'
import type { QueuedChatMessage } from './ChatQueueBar'

describe('ChatQueueBar', () => {
  const sample: QueuedChatMessage[] = [
    { id: 'q1', text: '先检查日志', attachments: [], planMode: false },
  ]

  it('supports editing, deleting, and switching to steer', async () => {
    const user = userEvent.setup()
    const onSteer = vi.fn()
    const onEdit = vi.fn()
    const onDelete = vi.fn()

    render(
      <TooltipProvider>
        <ChatQueueBar items={sample} busySteer={false} onSteer={onSteer} onEdit={onEdit} onDelete={onDelete} />
      </TooltipProvider>,
    )

    expect(screen.getByText('先检查日志')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '编辑排队消息' }))
    const input = screen.getByRole('textbox', { name: '编辑排队消息' })
    await user.clear(input)
    await user.type(input, '先检查错误日志{Enter}')
    expect(onEdit).toHaveBeenCalledWith('q1', '先检查错误日志')

    await user.click(screen.getByRole('button', { name: '切换到引导' }))
    expect(onSteer).toHaveBeenCalledWith(sample[0])

    await user.click(screen.getByRole('button', { name: '删除排队消息' }))
    expect(onDelete).toHaveBeenCalledWith('q1')
  })
})
