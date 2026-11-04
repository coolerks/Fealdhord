import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { articleFixtures, preferencesFixture } from '@/test/fixtures'
import { installGeometry } from '@/test/geometry'
import { ArticleCollection } from './ArticleCollection'
import { ArticleItem } from './ArticleItem'
import { columnCount, coverHeight, findAnchor } from './layout'

let geometry: ReturnType<typeof installGeometry>
beforeEach(() => { geometry = installGeometry() })
afterEach(() => { geometry.cleanup(); vi.restoreAllMocks() })

describe('真实虚拟双视图', () => {
  it('一万条数据仅挂载可见范围，切换多列后 DOM 仍为逻辑顺序', async () => {
    const articles = articleFixtures(10000)
    const actions = { onOpen: vi.fn(), onFavorite: vi.fn(), onRead: vi.fn(), onAnchor: vi.fn() }
    const { container, rerender } = render(<TooltipProvider><ArticleCollection articles={articles} preferences={preferencesFixture()} queryKey="all||newest" selectedID="" {...actions} /></TooltipProvider>)
    await waitFor(() => { expect(container.querySelectorAll('li').length).toBeGreaterThan(0) })
    expect(container.querySelectorAll('li').length).toBeLessThan(100)
    await act(async () => { rerender(<TooltipProvider><ArticleCollection articles={articles} preferences={preferencesFixture({ article_view_mode: 'masonry' })} queryKey="all||newest" selectedID="a-0" {...actions} /></TooltipProvider>) })
    await waitFor(() => { expect(container.querySelector('.masonry-scroll')).toBeInTheDocument() })
    await act(async () => { geometry.resize() })
    await waitFor(() => { expect(Array.from(container.querySelectorAll('li')).some(item => !item.style.transform.startsWith('translate(0px,'))).toBe(true) })
    const items = Array.from(container.querySelectorAll('li'))
    expect(items.length).toBeLessThan(100)
    const indexes = items.map(item => Number(item.dataset.index))
    expect(indexes).toEqual([...indexes].sort((a,b) => a-b))
    expect(items.some(item => !item.style.transform.startsWith('translate(0px,'))).toBe(true)
    expect(screen.getByRole('button', { name: '打开文章：测试文章 0' })).toHaveAttribute('aria-current', 'true')
  })

  it('快速切换前冲刷尚未到防抖时限的 ID 锚点，重挂载恢复位置', async () => {
    const articles = articleFixtures(1000)
    const actions = { onOpen: vi.fn(), onFavorite: vi.fn(), onRead: vi.fn(), onAnchor: vi.fn() }
    const { container, unmount } = render(<TooltipProvider><ArticleCollection articles={articles} preferences={preferencesFixture()} queryKey="all||newest" selectedID="" {...actions} /></TooltipProvider>)
    await waitFor(() => expect(container.querySelectorAll('li').length).toBeGreaterThan(0))
    await new Promise(resolve => setTimeout(resolve, 40))
    const scroller = container.querySelector<HTMLElement>('.article-scroll')!
    act(() => { scroller.scrollTop = 780; fireEvent.scroll(scroller) })
    unmount()
    expect(actions.onAnchor).toHaveBeenCalledWith('all||newest|list', { article_id: 'a-10', offset: 20 })
    const restored = render(<TooltipProvider><ArticleCollection articles={articles} preferences={preferencesFixture({ scroll_anchors: { 'all||newest|list': { article_id: 'a-10', offset: 20 } } })} queryKey="all||newest" selectedID="" {...actions} /></TooltipProvider>)
    await waitFor(() => expect(restored.container.querySelector('.article-scroll')?.scrollTop).toBe(780))
  })

  it('坏图回退为文字卡，标题和操作仍可用', () => {
    const article = articleFixtures(1)[0]
    const onOpen = vi.fn()
    render(<TooltipProvider><ArticleItem article={article} mode="masonry" selected={false} thumbnail="none" width={300} imageMode="original_ratio" onOpen={onOpen} onFavorite={vi.fn()} onRead={vi.fn()} /></TooltipProvider>)
    fireEvent.error(screen.getByRole('img'))
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '打开文章：测试文章 0' }))
    expect(onOpen).toHaveBeenCalledWith('a-0')
  })

  it('窄容器降为单列，极长图有上限，锚点绑定文章 ID', () => {
    expect(columnCount(180,'masonry')).toBe(1)
    expect(columnCount(820,'masonry')).toBe(3)
    const article = articleFixtures(1)[0]
    expect(coverHeight({ ...article, cover: { url: '/media/tall', width: 10, height: 10000 } }, 300, 'original_ratio')).toBe(320)
    expect(coverHeight({ ...article, cover: { url: 'https://external/image' } }, 300, 'cropped')).toBe(0)
    expect(findAnchor([{index:0,start:0,end:76},{index:1,start:76,end:152}],articleFixtures(2),91)).toEqual({article_id:'a-1',offset:15})
  })
})
