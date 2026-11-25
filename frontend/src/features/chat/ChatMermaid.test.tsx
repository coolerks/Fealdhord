import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { ChatMermaid } from './ChatMermaid'

const saveMock = vi.fn().mockResolvedValue(true)

vi.mock('../../../bindings/github.com/coolerks/Fealdhord/internal/chat/api', () => ({
  SaveMermaidImage: (...args: unknown[]) => saveMock(...args),
}))

vi.mock('mermaid', () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(async (_id: string, code: string) => {
      if (code.includes('INVALID')) throw new Error('syntax error')
      return { svg: '<svg viewBox="0 0 120 60"><rect width="120" height="60" /></svg>' }
    }),
  },
}))

vi.mock('@/lib/notifications', () => ({ notify: vi.fn() }))

describe('ChatMermaid', () => {
  it('renders mermaid SVG and exports SVG', async () => {
    saveMock.mockClear()
    const user = userEvent.setup()
    render(<TooltipProvider><ChatMermaid code={'graph TD\nA-->B'} streaming={false} /></TooltipProvider>)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: '下载 SVG' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: '下载 PNG' })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: '下载 SVG' }))
    await waitFor(() => {
      expect(saveMock).toHaveBeenCalledWith(expect.stringMatching(/\.svg$/), 'svg', expect.stringContaining('<svg'))
    })
  })

  it('shows error fallback when mermaid syntax is invalid', async () => {
    render(<TooltipProvider><ChatMermaid code="INVALID" streaming={false} /></TooltipProvider>)
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('syntax error')
    })
  })
})
