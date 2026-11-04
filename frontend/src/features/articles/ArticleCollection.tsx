import { useVirtualizer } from '@tanstack/react-virtual'
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { Article } from '@/lib/library'
import type { Preferences } from '@/lib/preferences'
import { ArticleItem } from './ArticleItem'
import { columnCount, coverHeight, findAnchor } from './layout'
import { cn } from '@/lib/utils'

type Anchor = { article_id: string; offset: number }
type Props = {
  articles: Article[]; preferences: Preferences; queryKey: string; selectedID: string
  onOpen: (id: string) => void; onFavorite: (id: string) => void; onRead: (id: string) => void
  onAnchor: (key: string, anchor: Anchor) => void
}
export function ArticleCollection({ articles, preferences, queryKey, selectedID, onOpen, onFavorite, onRead, onAnchor }: Props) {
  'use no memo' // Virtualizer 是可变实例，不能由 React Compiler 自动缓存其方法结果。
  const parent = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(360)
  const mode = preferences.article_view_mode
  const lanes = columnCount(width, mode)
  const gap = mode === 'masonry' ? 12 : 0
  const columnWidth = Math.max(1, (width - (lanes - 1) * gap) / lanes)
  const anchor = useRef<Anchor | null>(null)
  const restored = useRef(false)
  const previous = useRef({ query: queryKey, mode, lanes, width })
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pending = useRef<{ key: string; anchor: Anchor } | null>(null)
  const onAnchorRef = useRef(onAnchor)
  const storageKey = `${queryKey}|${mode}`
  const restoring = useRef(false)
  const virtualizer = useVirtualizer({
    count: articles.length, getScrollElement: () => parent.current,
    getItemKey: index => articles[index].id,
    estimateSize: index => mode === 'list' ? preferences.list_density === 'compact' ? 76 : 100 : 156 + coverHeight(articles[index], columnWidth, preferences.masonry_image_mode),
    overscan: 5, lanes, gap,
    initialRect: { width: 360, height: 600 },
  })
  useLayoutEffect(() => { onAnchorRef.current = onAnchor }, [onAnchor])
  const capture = useCallback(() => {
    if (!parent.current || !restored.current || restoring.current || previous.current.mode !== mode || previous.current.query !== queryKey) return
    const current = findAnchor(virtualizer.getVirtualItems(), articles, parent.current.scrollTop)
    if (current) anchor.current = current
  }, [articles, virtualizer, mode, queryKey])

  useLayoutEffect(() => {
    const element = parent.current
    if (!element) return
    const measure = () => {
      const style = getComputedStyle(element)
      const next = (element.clientWidth || element.getBoundingClientRect().width) - (parseFloat(style.paddingLeft) || 0) - (parseFloat(style.paddingRight) || 0)
      if (next > 0) { capture(); setWidth(next) }
    }
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(element)
    return () => observer.disconnect()
  }, [capture])

  useLayoutEffect(() => {
    const before = previous.current
    const changedQuery = before.query !== queryKey
    const changedLayout = before.mode !== mode || before.lanes !== lanes || before.width !== width
    if (restored.current && !changedQuery && !changedLayout && !restoring.current) return
    restored.current = true
    previous.current = { query: queryKey, mode, lanes, width }
    const target = changedQuery ? preferences.scroll_anchors?.[storageKey] ?? null : anchor.current ?? preferences.scroll_anchors?.[storageKey] ?? null
    restoring.current = true
    virtualizer.measure()
    const index = target ? articles.findIndex(article => article.id === target.article_id) : -1
    if (index >= 0) virtualizer.scrollToIndex(index, { align: 'start' })
    else virtualizer.scrollToOffset(0)
    const frame = requestAnimationFrame(() => {
      if (target && index >= 0) {
        const offset = virtualizer.getOffsetForIndex(index, 'start')?.[0]
        if (offset !== undefined) virtualizer.scrollToOffset(Math.max(0, offset + target.offset))
        anchor.current = target
      }
      restoring.current = false
    })
    return () => cancelAnimationFrame(frame)
    // 锚点写入本身不能重置浏览位置；只有查询或几何变化才恢复。
  }, [queryKey, mode, lanes, width, virtualizer, articles, preferences.scroll_anchors, storageKey])

  useEffect(() => {
    if (!selectedID) return
    const index = articles.findIndex(article => article.id === selectedID)
    if (index < 0) return
    const element = parent.current?.querySelector<HTMLElement>(`[data-article-trigger="${CSS.escape(selectedID)}"]`)
    if (!element) virtualizer.scrollToIndex(index, { align: 'auto' })
  }, [articles, selectedID, virtualizer])

  useEffect(() => () => {
    if (saveTimer.current) clearTimeout(saveTimer.current)
    const last = pending.current
    pending.current = null
    if (last) onAnchorRef.current(last.key, last.anchor)
  }, [storageKey])

  return <div ref={parent} className={cn('article-scroll', mode === 'masonry' && 'masonry-scroll')} data-density={preferences.list_density} aria-label={mode === 'masonry' ? '瀑布流文章' : '文章列表'} onScroll={() => {
    if (restoring.current) return
    capture()
    if (saveTimer.current) clearTimeout(saveTimer.current)
    const current = anchor.current
    if (current) {
      pending.current = { key: storageKey, anchor: current }
      saveTimer.current = setTimeout(() => { pending.current = null; onAnchorRef.current(storageKey, current) }, 180)
    }
  }}>
    <ol className="virtual-articles" aria-label="文章" style={{ height: virtualizer.getTotalSize() }}>
      {virtualizer.getVirtualItems().map(item => {
        const article = articles[item.index]
        return <li key={article.id} data-index={item.index} ref={virtualizer.measureElement} style={{ position: 'absolute', top: 0, left: 0, width: columnWidth, transform: `translate(${item.lane * (columnWidth + gap)}px, ${item.start}px)` }}>
          <ArticleItem article={article} mode={mode} selected={article.id === selectedID} thumbnail={preferences.list_thumbnail} width={columnWidth} imageMode={preferences.masonry_image_mode} onOpen={onOpen} onFavorite={onFavorite} onRead={onRead} />
        </li>
      })}
    </ol>
  </div>
}
