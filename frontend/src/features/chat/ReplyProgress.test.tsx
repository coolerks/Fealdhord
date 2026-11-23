import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it } from 'vitest'
import type { Message } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'
import { ReplyBody } from './ReplyProgress'

function makeMessage(patch: Partial<Message> = {}): Message {
  return {
    id: 'm1',
    role: 'assistant',
    text: '',
    modelName: '测试模型',
    status: 'running',
    errorCode: '',
    ...patch,
  }
}

it('首字前显示带掠光的正在思考，收到首字后隐藏，完成后才展示用时', () => {
  const { rerender } = render(<ReplyBody message={makeMessage()} markdown />)
  const thinking = screen.getByRole('status')
  expect(thinking).toHaveTextContent('正在思考')
  expect(thinking).toHaveClass('chat-thinking-label')

  rerender(<ReplyBody message={makeMessage({ text: '你好' })} markdown />)
  expect(screen.queryByText('正在思考')).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: /用时/ })).not.toBeInTheDocument()
  expect(screen.getByText('你好')).toBeInTheDocument()
})

it('轮次调用工具中只展示当前工具进行中摘要且无旋转圆圈，轮次完成后展示完整摘要并在下一行显示正在思考', async () => {
  const user = userEvent.setup()
  const runningToolMessage = makeMessage({
    details: {
      durationMs: 0,
      rounds: 1,
      reasoningEffort: '',
      work: {
        durationMs: 1200,
        reasoningObserved: false,
        tools: [
          { name: 'article.list', status: 'completed', arguments: '{}', result: '[]' },
          { name: 'article.get', status: 'running', arguments: '{"id":"1"}' },
        ],
        positions: [
          { offset: 0, summaryEnd: 0, reasoningEnd: 0, toolEnd: 2, durationMs: 1200 },
        ],
      },
    },
  })

  const { container, rerender } = render(<ReplyBody message={runningToolMessage} markdown />)
  // 不展示外层「用时」或「正在调用工具」
  expect(screen.queryByRole('button', { name: /用时|正在调用工具/ })).not.toBeInTheDocument()
  // 无旋转圆圈
  expect(container.querySelector('.chat-spin')).toBeNull()
  // 轮次摘要仅显示当前运行中的工具摘要，带掠光
  const [roundBtn] = screen.getAllByRole('button', { name: '读取文章正文中' })
  expect(roundBtn).toHaveAttribute('aria-expanded', 'true')
  expect(roundBtn.querySelector('.chat-thinking-label')).toHaveTextContent('读取文章正文中')
  // 工具记录里：已完成工具显示「工具名 + 已完成」，进行中工具显示进行中摘要且带掠光
  const toolsList = screen.getByRole('list', { name: '工具活动' })
  expect(toolsList).toHaveTextContent('列出文章已完成')
  const runningStep = within(toolsList).getByRole('button', { name: '读取文章正文中' })
  expect(runningStep.querySelector('.chat-thinking-label')).toHaveTextContent('读取文章正文中')

  // 本轮工具全部完成后、下一轮尚未输出任何 token
  const betweenRoundsMessage = makeMessage({
    details: {
      durationMs: 0,
      rounds: 1,
      reasoningEffort: '',
      work: {
        durationMs: 2400,
        reasoningObserved: false,
        tools: [
          { name: 'article.list', status: 'completed', arguments: '{}', result: '[]' },
          { name: 'article.get', status: 'completed', arguments: '{"id":"1"}', result: 'ok' },
        ],
        positions: [
          { offset: 0, summaryEnd: 0, reasoningEnd: 0, toolEnd: 2, durationMs: 2400 },
        ],
      },
    },
  })
  rerender(<ReplyBody message={betweenRoundsMessage} markdown />)
  // 本轮显示完整工作摘要，不再带掠光
  const completedRoundBtn = screen.getByRole('button', { name: '查看了文章列表，读取了文章' })
  expect(completedRoundBtn.querySelector('.chat-thinking-label')).toBeNull()
  // 下一行显示正在思考（带掠光）
  const nextThinking = screen.getByRole('status')
  expect(nextThinking).toHaveTextContent('正在思考')
  expect(nextThinking).toHaveClass('chat-thinking-label')
  expect(completedRoundBtn.compareDocumentPosition(nextThinking) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

  // 下一轮输出首 token 后隐藏正在思考
  rerender(<ReplyBody message={{ ...betweenRoundsMessage, text: '最终结论' }} markdown />)
  expect(screen.queryByText('正在思考')).not.toBeInTheDocument()
  expect(screen.getByText('最终结论')).toBeInTheDocument()

  // 整个回答完成后展示用时并默认折叠工作过程
  rerender(<ReplyBody message={{
    ...betweenRoundsMessage,
    text: '最终结论',
    status: 'complete',
    details: { ...betweenRoundsMessage.details!, durationMs: 4000 },
  }} markdown />)
  const summaryBtn = screen.getByRole('button', { name: '用时 4 秒' })
  expect(summaryBtn).toHaveAttribute('aria-expanded', 'false')
  expect(screen.queryByRole('button', { name: '查看了文章列表，读取了文章' })).not.toBeInTheDocument()
  await user.click(summaryBtn)
  expect(screen.getByRole('button', { name: '查看了文章列表，读取了文章' })).toBeVisible()
})
