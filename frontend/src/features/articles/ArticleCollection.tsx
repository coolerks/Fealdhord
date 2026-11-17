import { useVirtualizer } from '@tanstack/react-virtual'
import { useCallback, useEffect, useEffectEvent, useLayoutEffect, useRef, useState } from 'react'
import type { Article } from '@/lib/library'
import type { Preferences } from '@/lib/preferences'
import { ArticleItem } from './ArticleItem'
import { columnCount, coverHeight, findAnchor } from './layout'
import { cn } from '@/lib/utils'
import { ScrollArea } from '@/components/ui/scroll-area'

type Anchor = { article_id: string; offset: number }
type Props = {
  feedIcons?: ReadonlyMap<string, string | undefined>
  articles: Article[]; preferences: Preferences; queryKey: string; selectedID: string
  motionMode?: 'full' | 'subtle' | 'off'
  onOpen: (id: string) => void; onFavorite: (id: string) => void; onRead: (id: string) => void
  onAnchor: (key: string, anchor: Anchor) => void
  nextCursor?: string
  loadingMore?: boolean
  onLoadMore?: () => void
}
function visibleCardRects(viewport: HTMLDivElement) {
  const bounds = viewport.getBoundingClientRect()
  const cards = new Map<string, { x: number; y: number; width: number; element: HTMLLIElement }>()
  for (const card of viewport.querySelectorAll<HTMLLIElement>('.virtual-articles > li')) {
    const rect = card.getBoundingClientRect()
    if (rect.width && rect.bottom > bounds.top && rect.top < bounds.bottom) {
      cards.set(card.dataset.cardId!, { x: rect.left - bounds.left, y: rect.top - bounds.top, width: rect.width, element: card })
    }
  }
  return cards
}

export function ArticleCollection({ feedIcons, articles, preferences, queryKey, selectedID, motionMode = 'off', onOpen, onFavorite, onRead, onAnchor, nextCursor, loadingMore, onLoadMore }: Props) {
  'use no memo' // Virtualizer 是可变实例，不能由 React Compiler 自动缓存其方法结果。
  const parent = useRef<HTMLDivElement>(null)
  const end = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(360)
  const [resizeRevision, setResizeRevision] = useState(0)
  const measuredWidth = useRef(360)
  const previousCards = useRef<ReturnType<typeof visibleCardRects> | null>(null)
  const movingCards = useRef<Animation[]>([])
  const imagesAllowed = preferences.remote_images !== 'never'
  const mode = preferences.article_view_mode
  const rowHeight = preferences.list_density === 'compact' ? 96 : 120
  const lanes = columnCount(width, mode)
  const gap = mode === 'masonry' ? 12 : 0
  const columnWidth = Math.max(1, (width - (lanes - 1) * gap) / lanes)
  const anchor = useRef<Anchor | null>(null)
  const restored = useRef(false)
  const previous = useRef({ query: queryKey, mode, lanes, rowHeight, imageMode: preferences.masonry_image_mode, imagesAllowed, resizeRevision })
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pending = useRef<{ key: string; anchor: Anchor } | null>(null)
  const onAnchorRef = useRef(onAnchor)
  const storageKey = `${queryKey}|${mode}`
  const restoring = useRef(false)
  const getItemKey = useCallback((index: number) => articles[index].id, [articles])
  const virtualizer = useVirtualizer<HTMLDivElement, HTMLLIElement>({
    count: articles.length, getScrollElement: () => parent.current,
    getItemKey,
    estimateSize: index => mode === 'list' ? rowHeight : 156 + (imagesAllowed ? coverHeight(articles[index], columnWidth, preferences.masonry_image_mode) : 0),
    overscan: 5, lanes, gap,
    useAnimationFrameWithResizeObserver: mode === 'masonry',
    initialRect: { width: 360, height: 600 },
  })
  useLayoutEffect(() => { onAnchorRef.current = onAnchor }, [onAnchor])
  const capture = useCallback(() => {
    if (!parent.current || !restored.current || restoring.current || previous.current.mode !== mode || previous.current.query !== queryKey) return
    const current = findAnchor(virtualizer.getVirtualItems(), articles, parent.current.scrollTop)
    if (current) anchor.current = current
  }, [articles, virtualizer, mode, queryKey])

  const measureWidth = useEffectEvent((element: HTMLDivElement, animate: boolean) => {
    const style = getComputedStyle(element)
    const next = (element.clientWidth || element.getBoundingClientRect().width) - (parseFloat(style.paddingLeft) || 0) - (parseFloat(style.paddingRight) || 0)
    if (next <= 0 || Math.abs(next - width) < 1) return
    const changedLanes = columnCount(next, mode) !== lanes
    if (changedLanes) capture()
    if (animate && motionMode === 'full' && (changedLanes || Math.abs(next - width) >= 24)) {
      previousCards.current = visibleCardRects(element)
    }
    setWidth(next)
  })
  useLayoutEffect(() => {
    const element = parent.current
    if (!element || mode !== 'masonry') return
    let frame = 0
    measureWidth(element, false)
    const observer = new ResizeObserver(() => {
      if (frame) return
      frame = requestAnimationFrame(() => { frame = 0; measureWidth(element, true) })
    })
    observer.observe(element)
    return () => { observer.disconnect(); cancelAnimationFrame(frame) }
  }, [mode])

  const settleWidth = useEffectEvent(() => {
    if (measuredWidth.current === width) return
    capture()
    const viewport = parent.current
    if (viewport && motionMode === 'full') previousCards.current = visibleCardRects(viewport)
    // 只丢弃未挂载卡片的旧宽度测量值。可见卡片沿用当前 DOM 尺寸。
    virtualizer.measure()
    for (const [id, card] of virtualizer.elementsCache) {
      if (card.isConnected && card.offsetHeight > 0) virtualizer.itemSizeCache.set(id, card.offsetHeight)
    }
    setResizeRevision(revision => revision + 1)
  })
  useEffect(() => {
    if (mode !== 'masonry') return
    const idle = setTimeout(settleWidth, 120)
    return () => clearTimeout(idle)
  }, [mode, width])

  useLayoutEffect(() => {
    const before = previous.current
    const changedQuery = before.query !== queryKey
    const changedMode = before.mode !== mode
    const changedLanes = before.lanes !== lanes
    const changedMeasurements = mode === 'list' ? before.rowHeight !== rowHeight
      : before.imageMode !== preferences.masonry_image_mode || before.imagesAllowed !== imagesAllowed
    const finishedResize = before.resizeRevision !== resizeRevision
    if (restored.current && !changedQuery && !changedMode && !changedLanes && !changedMeasurements && !finishedResize) return
    restored.current = true
    previous.current = { query: queryKey, mode, lanes, rowHeight, imageMode: preferences.masonry_image_mode, imagesAllowed, resizeRevision }
    measuredWidth.current = width
    const saved = preferences.scroll_anchors?.[storageKey]
    const target = changedQuery ? saved ?? null : changedMode ? saved ?? anchor.current : anchor.current ?? saved
    restoring.current = true
    // 列数变化由 Virtualizer 自行重建。
    if (!changedLanes && (changedMode || changedMeasurements)) virtualizer.measure()
    const index = target ? articles.findIndex(article => article.id === target.article_id) : -1
    const offset = index >= 0 ? virtualizer.getOffsetForIndex(index, 'start')?.[0] : undefined
    virtualizer.scrollToOffset(offset === undefined ? 0 : Math.max(0, offset + (target?.offset ?? 0)))
    anchor.current = index >= 0 ? target ?? null : null
    restoring.current = false
    // 锚点写入本身不能重置浏览位置；只有查询或几何变化才恢复。
  }, [queryKey, mode, lanes, width, resizeRevision, rowHeight, imagesAllowed, virtualizer, articles, preferences.masonry_image_mode, preferences.scroll_anchors, storageKey])

  useLayoutEffect(() => {
    const before = previousCards.current
    const viewport = parent.current
    previousCards.current = null
    for (const animation of movingCards.current) animation.cancel()
    movingCards.current = []
    if (!before || !viewport || mode !== 'masonry' || motionMode !== 'full') return

    for (const [id, card] of visibleCardRects(viewport)) {
      const previous = before.get(id)
      if (!previous || !card.element.animate) continue
      const x = previous.x - card.x
      const y = previous.y - card.y
      if (Math.abs(x) < 1 && Math.abs(y) < 1 && Math.abs(previous.width - card.width) < 1) continue
      const transform = card.element.style.transform
      movingCards.current.push(card.element.animate([
        { transform: `translate(${x}px, ${y}px) ${transform} scaleX(${previous.width / card.width})` },
        { transform },
      ], { duration: 180, easing: 'cubic-bezier(.22,1,.36,1)' }))
    }
  }, [mode, motionMode, width, resizeRevision])
  useLayoutEffect(() => () => {
    for (const animation of movingCards.current) animation.cancel()
  }, [])

  const selectedIndex = selectedID ? articles.findIndex(article => article.id === selectedID) : -1
  useEffect(() => {
    if (selectedIndex < 0) return
    const element = parent.current?.querySelector<HTMLElement>(`[data-article-trigger="${CSS.escape(selectedID)}"]`)
    if (!element) virtualizer.scrollToIndex(selectedIndex, { align: 'auto' })
  }, [queryKey, selectedID, selectedIndex, virtualizer])

  useEffect(() => {
    const root = parent.current
    const target = end.current
    if (!root || !target || !nextCursor || !onLoadMore) return
    let active = true
    const observer = new IntersectionObserver(([entry]) => {
      if (!active || !entry?.isIntersecting) return
      active = false
      observer.disconnect()
      onLoadMore()
    }, { root, rootMargin: '0px 0px 400px' })
    observer.observe(target)
    return () => {
      active = false
      observer.disconnect()
    }
  }, [queryKey, nextCursor, onLoadMore])

  useEffect(() => () => {
    if (saveTimer.current) clearTimeout(saveTimer.current)
    const last = pending.current
    pending.current = null
    if (last) onAnchorRef.current(last.key, last.anchor)
  }, [storageKey])

  return <ScrollArea viewportProps={{
    ref: parent,
    className: cn('article-scroll', mode === 'masonry' && 'masonry-scroll'),
    'data-density': preferences.list_density,
    'aria-label': mode === 'masonry' ? '瀑布流文章' : '文章列表',
    onScroll: () => {
      if (restoring.current) return
      capture()
      if (saveTimer.current) clearTimeout(saveTimer.current)
      const current = anchor.current
      if (current) {
        pending.current = { key: storageKey, anchor: current }
        saveTimer.current = setTimeout(() => { pending.current = null; onAnchorRef.current(storageKey, current) }, 180)
      }
    },
  }}>
    <ol className="virtual-articles" aria-label="文章" style={{ height: virtualizer.getTotalSize() }}>
      {virtualizer.getVirtualItems().map(item => {
        const article = articles[item.index]
        return <li key={article.id} data-card-id={article.id} data-index={item.index} ref={virtualizer.measureElement} style={{ position: 'absolute', top: 0, left: 0, width: mode === 'list' ? '100%' : columnWidth, height: mode === 'list' ? rowHeight : undefined, transform: `translate(${item.lane * (columnWidth + gap)}px, ${item.start}px)` }}>
          <ArticleItem imagesAllowed={imagesAllowed} feedIcon={imagesAllowed && article.feedID ? feedIcons?.get(article.feedID) : undefined} article={article} mode={mode} selected={article.id === selectedID} width={columnWidth} imageMode={preferences.masonry_image_mode} onOpen={onOpen} onFavorite={onFavorite} onRead={onRead} />
        </li>
      })}
    </ol>
    {nextCursor && <div ref={end} aria-hidden="true" className="h-px" />}
    {loadingMore && <p className="library-limit" role="status">正在载入更多文章…</p>}
  </ScrollArea>
}
