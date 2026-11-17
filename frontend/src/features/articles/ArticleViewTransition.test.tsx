import { act, fireEvent, render, screen } from '@testing-library/react'
import { createRef } from 'react'
import { expect, it, vi } from 'vitest'
import { ArticleViewTransition } from './ArticleViewTransition'

it('双向切换保留旧视口到淡出结束，关闭动效时直接切换', () => {
  const bounds = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 800, 600))
  try {
    const host = createRef<HTMLDivElement>()
    const tree = (view: string, motion: 'full' | 'off' = 'full') => <>
      <ArticleViewTransition view={view} motion={motion} host={host}>
        <div className="article-scroll"><button id="article" data-article-trigger="a">{view}</button></div>
      </ArticleViewTransition>
      <div ref={host} aria-hidden="true" />
    </>
    const { container, rerender } = render(tree('列表'))
    act(() => { container.querySelector('.article-scroll')!.scrollTop = 180 })
    rerender(tree('瀑布流'))
    const outgoing = host.current?.firstElementChild as HTMLElement
    expect(outgoing).toHaveTextContent('列表')
    expect(outgoing.inert).toBe(true)
    expect(outgoing.querySelector('.article-scroll')?.scrollTop).toBe(180)
    expect(outgoing.querySelector('#article, [data-article-trigger]')).toBeNull()
    expect(screen.getByRole('button', { name: '瀑布流' })).toBeInTheDocument()
    fireEvent.animationEnd(outgoing)
    expect(host.current).toBeEmptyDOMElement()

    rerender(tree('列表'))
    expect(host.current?.firstElementChild).toHaveTextContent('瀑布流')
    rerender(tree('瀑布流', 'off'))
    expect(host.current).toBeEmptyDOMElement()
    expect(container.querySelector('[data-view-entering]')).toBeNull()
  } finally {
    bounds.mockRestore()
  }
})

it('铺满阅读时旧浏览区仍淡出，返回列表时浏览区淡入', () => {
  const bounds = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 800, 600))
  try {
    const host = createRef<HTMLDivElement>()
    const tree = (view: string, reading: boolean) => <>
      <section inert={reading}><ArticleViewTransition view={view} motion="full" host={host}>{view}</ArticleViewTransition></section>
      <div ref={host} aria-hidden="true" />
    </>
    const { container, rerender } = render(tree('列表', false))
    rerender(tree('瀑布流', true))
    const outgoing = host.current!.firstElementChild!
    expect(outgoing).toHaveTextContent('列表')
    expect(container.querySelector('[data-view-entering]')).toBeNull()
    fireEvent.animationEnd(outgoing)

    rerender(tree('列表', false))
    expect(host.current).toBeEmptyDOMElement()
    const incoming = container.querySelector('[data-view-entering]')!
    expect(incoming).toHaveTextContent('列表')
    fireEvent.animationEnd(incoming)
    expect(container.querySelector('[data-view-entering]')).toBeNull()
  } finally {
    bounds.mockRestore()
  }
})
