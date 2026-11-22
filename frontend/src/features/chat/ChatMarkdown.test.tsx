import { render, screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import { ChatMarkdown } from './ChatMarkdown'
import { addRevealBatch } from './stream-reveal'

it('只让新增的中文、字形和链接文本渐次出现，保留旧文本和代码节点', () => {
  const start = '旧文字 [文档](https://example.test/)\n\n```txt\n代码\n```\n\n'
  const next = start + '新段👩‍💻落'
  const at = performance.now()
  const first = [{ start: start.length, end: next.length, receivedAt: at }]
  const { container, rerender } = render(<ChatMarkdown text={next} streaming reveal={first} />)
  const reveal = () => Array.from(container.querySelectorAll('[data-chat-reveal]'))
  expect(screen.getByRole('link', { name: '文档' })).toHaveAttribute('href', 'https://example.test/')
  expect(container.querySelector('pre')).toHaveTextContent('代码')
  expect(reveal().map(span => span.textContent)).toEqual(['新', '段', '👩‍💻', '落'])
  const stable = reveal()[0]
  const tail = '继续'
  const batches = addRevealBatch(first, { start: next.length, end: next.length + tail.length, receivedAt: at + 32 })
  rerender(<ChatMarkdown text={next + tail} streaming reveal={batches} />)
  expect(reveal()[0]).toBe(stable)
  expect(reveal().map(span => span.textContent).join('')).toBe('新段👩‍💻落继续')
  rerender(<ChatMarkdown text={next + tail} streaming={false} reveal={batches} />)
  expect(reveal()).toHaveLength(0)
  expect(screen.getByText('新段👩‍💻落继续')).toBeInTheDocument()
})

it('相同片段更新时不重播，并在系统要求减少动效时跳过淡入', () => {
  const text = '新回复'
  const batch = [{ start: 0, end: text.length, receivedAt: performance.now() }]
  const animate = vi.fn(() => ({ currentTime: 0, cancel: vi.fn() }))
  const original = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'animate')
  Object.defineProperty(HTMLElement.prototype, 'animate', { configurable: true, value: animate })
  try {
    const { unmount, rerender } = render(<ChatMarkdown text={text} streaming reveal={batch} />)
    expect(animate).toHaveBeenCalledTimes(3)
    rerender(<ChatMarkdown text={text} streaming reveal={[...batch]} />)
    expect(animate).toHaveBeenCalledTimes(3)
    unmount()
    vi.stubGlobal('matchMedia', () => ({ matches: true }))
    render(<ChatMarkdown text={text} streaming reveal={batch} />)
    expect(animate).toHaveBeenCalledTimes(3)
  } finally {
    if (original) Object.defineProperty(HTMLElement.prototype, 'animate', original)
    else Reflect.deleteProperty(HTMLElement.prototype, 'animate')
    vi.unstubAllGlobals()
  }
})
