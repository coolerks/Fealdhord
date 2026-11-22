import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { pickGreeting, timeSlotForHour } from './ChatGreeting'
import { UserQuestionPrompt } from './UserQuestionPrompt'

const answerQuestion = vi.fn(async () => true)

vi.mock('../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api', () => ({
  PendingQuestions: async () => [],
  AnswerQuestion: (...args: unknown[]) => answerQuestion(...(args as [])),
}))

describe('ChatGreeting', () => {
  it('按时间段返回带问句的问候语', () => {
    expect(timeSlotForHour(8)).toBe('morning')
    expect(timeSlotForHour(12)).toBe('noon')
    expect(timeSlotForHour(15)).toBe('afternoon')
    expect(timeSlotForHour(18)).toBe('evening')
    expect(timeSlotForHour(21)).toBe('night')
    expect(timeSlotForHour(1)).toBe('lateNight')

    for (const hour of [8, 12, 15, 18, 21, 1]) {
      const text = pickGreeting(new Date(2026, 0, 1, hour, 0, 0), () => 0)
      expect(text).toContain('？')
    }
  })
})

describe('UserQuestionPrompt', () => {
  it('支持多题单选、多选以及补充输入并一次提交', async () => {
    answerQuestion.mockClear()
    const user = userEvent.setup()
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={client}>
        <UserQuestionPrompt
          pending={{
            id: 'q-1',
            conversationId: 'conv-1',
            questions: [
              {
                header: '范围',
                question: '需要搜索哪些文章？',
                multi_select: false,
                options: [{ label: '仅未读' }, { label: '全部文章' }],
              },
              {
                header: '输出',
                question: '希望包含哪些内容？',
                multi_select: true,
                options: [{ label: '标题' }, { label: '摘要' }, { label: '链接' }],
              },
            ],
          }}
        />
      </QueryClientProvider>,
    )

    expect(screen.getByText('1 / 2')).toBeInTheDocument()
    await user.click(screen.getByRole('radio', { name: '仅未读' }))
    await user.type(screen.getByRole('textbox', { name: '补充说明' }), '限定本周')
    await user.click(screen.getByRole('button', { name: '下一题' }))

    expect(screen.getByText('2 / 2')).toBeInTheDocument()
    await user.click(screen.getByRole('checkbox', { name: '标题' }))
    await user.click(screen.getByRole('checkbox', { name: '链接' }))
    await user.type(screen.getByRole('textbox', { name: '补充说明' }), '按时间倒序')
    await user.click(screen.getByRole('button', { name: '提交回答' }))

    expect(answerQuestion).toHaveBeenCalledExactlyOnceWith(
      'q-1',
      [
        { question: '需要搜索哪些文章？', selected: ['仅未读'], note: '限定本周' },
        { question: '希望包含哪些内容？', selected: ['标题', '链接'], note: '按时间倒序' },
      ],
      false,
    )
  })
})
