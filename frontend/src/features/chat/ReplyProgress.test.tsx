import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import type { Message } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'
import { ReplyBody } from './ReplyProgress'

afterEach(() => {
  vi.useRealTimers()
})

function makeMessage(patch: Partial<Message> = {}): Message {
  return {
    id: 'm1',
    role: 'assistant',
    text: '',
    modelName: '测试模型',
    status: 'running',
    errorCode: '',
    createdAt: 0,
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

it('轮次调用工具中只展示当前工具进行中摘要且无旋转圆圈，跨轮次推理期间稳定显示正在思考不闪烁', () => {
  vi.useFakeTimers()
  const runningToolMessage = makeMessage({
    details: {
      durationMs: 0,
      rounds: 1,
      reasoningEffort: '',
      work: {
        durationMs: 1200,
        reasoningObserved: false,
        tools: [
          { name: 'article_list', status: 'completed', arguments: '{}', result: '[]' },
          { name: 'article_get', status: 'running', arguments: '{"id":"1"}' },
        ],
        positions: [
          { offset: 0, summaryEnd: 0, reasoningEnd: 0, toolEnd: 2, durationMs: 1200 },
        ],
      },
    },
  })

  const { container, rerender } = render(<ReplyBody message={runningToolMessage} markdown />)
  expect(screen.queryByRole('button', { name: /用时|正在调用工具/ })).not.toBeInTheDocument()
  expect(container.querySelector('.chat-spin')).toBeNull()
  const [roundBtn] = screen.getAllByRole('button', { name: '读取文章中' })
  expect(roundBtn).toHaveAttribute('aria-expanded', 'true')
  expect(roundBtn.querySelector('.chat-thinking-label')).toHaveTextContent('读取文章中')
  const toolsList = screen.getByRole('list', { name: '工具活动' })
  expect(toolsList).toHaveTextContent('列出文章已完成')
  const runningStep = within(toolsList).getByRole('button', { name: '读取文章中' })
  expect(runningStep.querySelector('.chat-thinking-label')).toHaveTextContent('读取文章中')

  // 本轮工具全部完成后、下一轮尚未输出任何 token：短于 250ms 的并行工具间隙不闪烁
  const betweenRoundsMessage = makeMessage({
    details: {
      durationMs: 0,
      rounds: 1,
      reasoningEffort: '',
      work: {
        durationMs: 2400,
        reasoningObserved: false,
        tools: [
          { name: 'article_list', status: 'completed', arguments: '{}', result: '[]' },
          { name: 'article_get', status: 'completed', arguments: '{"id":"1"}', result: 'ok' },
        ],
        positions: [
          { offset: 0, summaryEnd: 0, reasoningEnd: 0, toolEnd: 2, durationMs: 2400 },
        ],
      },
    },
  })
  rerender(<ReplyBody message={betweenRoundsMessage} markdown />)
  expect(screen.queryByText('正在思考')).not.toBeInTheDocument()

  // 超过 250ms 后稳定出现下一行「正在思考」
  act(() => { vi.advanceTimersByTime(260) })
  const completedRoundBtn = screen.getByRole('button', { name: '查看了文章列表，读取了文章' })
  expect(completedRoundBtn.querySelector('.chat-thinking-label')).toBeNull()
  const nextThinking = screen.getByRole('status')
  expect(nextThinking).toHaveTextContent('正在思考')
  expect(nextThinking).toHaveClass('chat-thinking-label')
  expect(completedRoundBtn.compareDocumentPosition(nextThinking) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

  // 下一轮开始输出隐藏 reasoning 时，下一行「正在思考」保持稳定，不会闪烁消失
  const nextRoundReasoningMessage = makeMessage({
    details: {
      durationMs: 0,
      rounds: 2,
      reasoningEffort: '',
      work: {
        durationMs: 3000,
        reasoningObserved: true,
        reasoningText: '分析结果中...',
        tools: betweenRoundsMessage.details!.work!.tools,
        positions: [
          { offset: 0, summaryEnd: 0, reasoningEnd: 0, toolEnd: 2, durationMs: 2400 },
          { offset: 0, summaryEnd: 0, reasoningEnd: 8, toolEnd: 2, durationMs: 3000 },
        ],
      },
    },
  })
  rerender(<ReplyBody message={nextRoundReasoningMessage} markdown />)
  expect(screen.getByRole('status')).toHaveTextContent('正在思考')

  // 下一轮输出可见正文首 token 后立即隐藏「正在思考」
  rerender(<ReplyBody message={{ ...nextRoundReasoningMessage, text: '最终结论' }} markdown />)
  expect(screen.queryByText('正在思考')).not.toBeInTheDocument()
  expect(screen.getByText('最终结论')).toBeInTheDocument()

  // 整个回答完成后展示用时并默认折叠工作过程
  rerender(<ReplyBody message={{
    ...nextRoundReasoningMessage,
    text: '最终结论',
    status: 'complete',
    details: { ...nextRoundReasoningMessage.details!, durationMs: 4000 },
  }} markdown />)
  const summaryBtn = screen.getByRole('button', { name: '用时 4 秒' })
  expect(summaryBtn).toHaveAttribute('aria-expanded', 'false')
  expect(screen.queryByRole('button', { name: '查看了文章列表，读取了文章' })).not.toBeInTheDocument()
  fireEvent.click(summaryBtn)
  expect(screen.getByRole('button', { name: '查看了文章列表，读取了文章' })).toBeVisible()
})

it('流式失败自动重试时展示带掠光的重试进度提示', () => {
  render(
    <ReplyBody
      message={makeMessage({
        details: {
          durationMs: 0,
          rounds: 1,
          reasoningEffort: '',
          work: {
            durationMs: 1200,
            reasoningObserved: false,
            retryAttempt: 2,
            retryMax: 5,
            tools: [{ name: 'article_list', status: 'completed' }],
            positions: [{ offset: 0, summaryEnd: 0, reasoningEnd: 0, toolEnd: 1, durationMs: 1200 }],
          },
        },
      })}
      markdown
    />,
  )
  const retry = screen.getByRole('status')
  expect(retry).toHaveTextContent('网络或服务异常，正在重试（第 2/5 次）…')
  expect(retry).toHaveClass('chat-thinking-label')
})
