import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createRef } from 'react'
import { expect, it, vi } from 'vitest'
import type { Message } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'
import { buildChatTurns, ChatTimelineRail } from './ChatTimelineRail'

function makeMessages(): Message[] {
  return [
    { id: 'u1', role: 'user', text: '第一轮提问：看看今天 V2EX 的讨论', status: 'complete', errorCode: '', modelName: '' },
    { id: 'a1', role: 'assistant', text: '## 今日热点\n- 话题一\n- 话题二', status: 'complete', errorCode: '', modelName: '模型' },
    { id: 'u2', role: 'user', text: '第二轮提问：最近美团发表过什么文章', status: 'complete', errorCode: '', modelName: '' },
    { id: 'a2', role: 'assistant', text: '美团技术团队近期发布了两篇架构文章。', status: 'complete', errorCode: '', modelName: '模型' },
  ]
}

it('按用户提问与 AI 回复配对生成轮次摘要，并清理 Markdown 符号', () => {
  const turns = buildChatTurns(makeMessages())
  expect(turns).toHaveLength(2)
  expect(turns[0]).toMatchObject({
    id: 'u1',
    question: '第一轮提问：看看今天 V2EX 的讨论',
    answerPreview: '今日热点\n• 话题一\n• 话题二',
  })
  expect(turns[1]).toMatchObject({
    id: 'u2',
    question: '第二轮提问：最近美团发表过什么文章',
    answerPreview: '美团技术团队近期发布了两篇架构文章。',
  })
})

it('悬停刻度展示 HoverCard 预览并形成波浪宽度，点击刻度可滚动跳转到对应消息', async () => {
  const user = userEvent.setup()
  const viewportRef = createRef<HTMLDivElement>()
  const messages = makeMessages()

  render(
    <div>
      <div ref={viewportRef}>
        {messages.map(message => (
          <section key={message.id} data-message-id={message.id}>{message.text}</section>
        ))}
      </div>
      <ChatTimelineRail messages={messages} viewport={viewportRef} />
    </div>,
  )

  const scrollTo = vi.fn()
  Object.defineProperty(viewportRef.current!, 'scrollTo', { value: scrollTo, configurable: true })

  const rail = screen.getByRole('navigation', { name: '对话轮次导航' })
  expect(rail).toHaveClass('chat-timeline-rail')

  const firstTick = screen.getByRole('button', { name: /跳转到第 1 轮/ })
  const secondTick = screen.getByRole('button', { name: /跳转到第 2 轮/ })

  await user.hover(firstTick)
  expect(firstTick.querySelector('.chat-rail-tick')).toHaveStyle({ width: '26px' })
  expect(secondTick.querySelector('.chat-rail-tick')).toHaveStyle({ width: '18px' })
  await waitFor(() => expect(screen.getByText('第一轮提问：看看今天 V2EX 的讨论', { selector: 'p' })).toBeVisible())

  await user.click(firstTick)
  expect(scrollTo).toHaveBeenCalledTimes(1)
})
