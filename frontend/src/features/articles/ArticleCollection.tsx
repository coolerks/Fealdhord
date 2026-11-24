import { defaultRangeExtractor, useVirtualizer } from '@tanstack/react-virtual'
import type { Range } from '@tanstack/react-virtual'
import { useCallback, useEffect, useEffectEvent, useLayoutEffect, useMemo, useRef, useState } from 'react'
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
  onOpen: (id: string) => void; onFavorite: (id: string) => void; onRead: (id: string) => void
  onShare?: (article: Article, trigger: HTMLButtonElement) => void
  onAnchor: (key: string, anchor: Anchor) => void
  nextCursor?: string
  loadingMore?: boolean
  onLoadMore?: () => void
}
export function ArticleCollection({ feedIcons, articles, preferences, queryKey, selectedID, onOpen, onFavorite, onRead, onShare, onAnchor, nextCursor, loadingMore, onLoadMore }: Props) {
  'use no memo' // Virtualizer 是可变实例，不能由 React Compiler 自动缓存其方法结果。
  const parent = useRef<HTMLDivElement>(null)
  const end = useRef<HTMLDivElement>(null)
  const [layout, setLayout] = useState<{ width: number; retained?: { queryKey: string; indexes: number[] } }>({ width: 360 })
  const { width, retained } = layout
  const imagesAllowed = preferences.remote_images !== 'never'
  const mode = preferences.article_view_mode
  const rowHeight = preferences.list_density === 'compact' ? 96 : 120
  const lanes = columnCount(width, mode)
  const gap = mode === 'masonry' ? 12 : 0
  const columnWidth = Math.max(1, (width - (lanes - 1) * gap) / lanes)
  const anchor = useRef<Anchor | null>(null)
  const restored = useRef(false)
  const previous = useRef({ query: queryKey, mode, width, rowHeight, imageMode: preferences.masonry_image_mode, imagesAllowed })
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pending = useRef<{ key: string; anchor: Anchor } | null>(null)
  const onAnchorRef = useRef(onAnchor)
  const storageKey = `${queryKey}|${mode}`
  const restoring = useRef(false)
  const indexesByID = useMemo(() => new Map(articles.map((article, index) => [article.id, index])), [articles])
  const getItemKey = useCallback((index: number) => articles[index].id, [articles])
  const rangeExtractor = useCallback((range: Range) => mode === 'masonry' && retained?.queryKey === queryKey
    ? retained.indexes.filter(index => index < range.count)
    : defaultRangeExtractor(range), [mode, retained, queryKey])
  const virtualizer = useVirtualizer<HTMLDivElement, HTMLLIElement>({
    count: articles.length, getScrollElement: () => parent.current,
    getItemKey,
    estimateSize: index => mode === 'list' ? rowHeight : 156 + (imagesAllowed ? coverHeight(articles[index], columnWidth, preferences.masonry_image_mode) : 0),
    overscan: 5, lanes, gap, rangeExtractor,
    useAnimationFrameWithResizeObserver: mode === 'masonry',
    initialRect: { width: 360, height: 600 },
  })
  useLayoutEffect(() => { onAnchorRef.current = onAnchor }, [onAnchor])
  useLayoutEffect(() => {
    if (mode !== 'masonry') return
    virtualizer.shouldAdjustScrollPositionOnItemSizeChange = (item, _delta, instance) => {
      if (restoring.current || retained) return false
      const index = anchor.current ? indexesByID.get(anchor.current.article_id) : undefined
      const current = index === undefined ? undefined : instance.measurementsCache[index]
      // 瀑布流只有锚点所在列的前置卡片会改变该锚点的纵坐标。
      return Boolean(current && item.lane === current.lane && item.index < current.index)
    }
    return () => { virtualizer.shouldAdjustScrollPositionOnItemSizeChange = undefined }
  }, [mode, retained, indexesByID, virtualizer])
  const capture = useCallback(() => {
    if (!parent.current || !restored.current || restoring.current || previous.current.mode !== mode || previous.current.query !== queryKey) return
    const current = findAnchor(virtualizer.getVirtualItems(), articles, parent.current.scrollTop)
    if (current) anchor.current = current
  }, [articles, virtualizer, mode, queryKey])

  const commitWidth = useEffectEvent((next: number, preserveRange: boolean) => {
    if (next <= 0 || next === width) return
    capture()
    // 新列分配尚未写入滚动位置时，旧像素偏移会对应到另一组文章。
    // 先在现有节点上计算新尺寸，锚点就位后再放行虚拟范围。
    const indexes = preserveRange ? virtualizer.getVirtualIndexes() : []
    restoring.current = true
    setLayout({ width: next, retained: indexes.length > 0 ? { queryKey, indexes } : undefined })
  })
  useLayoutEffect(() => {
    const element = parent.current
    if (!element || mode !== 'masonry') return
    const readWidth = () => {
      const style = getComputedStyle(element)
      return Math.round((element.clientWidth || element.getBoundingClientRect().width) -
        (parseFloat(style.paddingLeft) || 0) - (parseFloat(style.paddingRight) || 0))
    }
    let observed = readWidth()
    let idle: ReturnType<typeof setTimeout> | undefined
    commitWidth(observed, false)
    const observer = new ResizeObserver(() => {
      const next = readWidth()
      if (next === observed) return
      observed = next
      // 保持当前网格直到拖动稳定，避免断点附近反复换列。
      clearTimeout(idle)
      idle = setTimeout(() => commitWidth(observed, true), 100)
    })
    observer.observe(element)
    return () => { observer.disconnect(); clearTimeout(idle) }
  }, [mode])

  useLayoutEffect(() => {
    const before = previous.current
    const changedQuery = before.query !== queryKey
    const changedMode = before.mode !== mode
    const changedWidth = mode === 'masonry' && before.width !== width
    const changedMeasurements = mode === 'list' ? before.rowHeight !== rowHeight
      : before.imageMode !== preferences.masonry_image_mode || before.imagesAllowed !== imagesAllowed
    if (restored.current && !changedQuery && !changedMode && !changedWidth && !changedMeasurements) return
    restored.current = true
    previous.current = { query: queryKey, mode, width, rowHeight, imageMode: preferences.masonry_image_mode, imagesAllowed }
    const saved = preferences.scroll_anchors?.[storageKey]
    const target = changedQuery ? saved ?? null : changedMode ? saved ?? anchor.current : anchor.current ?? saved
    restoring.current = true
    if (changedMode || changedWidth || changedMeasurements) {
      virtualizer.measure()
      if (mode === 'masonry') {
        // 在绘制前把当前尺寸一次性写回，避免逐卡校正引起重排和滚动跳动。
        for (const [id, card] of virtualizer.elementsCache) {
          if (!card.isConnected) continue
          const height = card.offsetHeight
          if (height > 0) virtualizer.itemSizeCache.set(id, height)
        }
      }
    }
    const index = target ? indexesByID.get(target.article_id) ?? -1 : -1
    const offset = index >= 0 ? virtualizer.getOffsetForIndex(index, 'start')?.[0] : undefined
    virtualizer.scrollToOffset(offset === undefined ? 0 : Math.max(0, offset + (target?.offset ?? 0)))
    anchor.current = index >= 0 ? target ?? null : null
    restoring.current = false
    // 锚点写入本身不能重置浏览位置；只有查询或几何变化才恢复。
  }, [queryKey, mode, width, rowHeight, imagesAllowed, virtualizer, indexesByID, preferences.masonry_image_mode, preferences.scroll_anchors, storageKey])

  useLayoutEffect(() => {
    if (!retained) return
    const frame = requestAnimationFrame(() => {
      setLayout(current => current.retained === retained ? { width: current.width } : current)
    })
    return () => cancelAnimationFrame(frame)
  }, [retained])

  const selectedIndex = indexesByID.get(selectedID) ?? -1
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
          <ArticleItem imagesAllowed={imagesAllowed} showTranslated={preferences.list_show_translated} feedIcon={imagesAllowed && article.feedID ? feedIcons?.get(article.feedID) : undefined} article={article} mode={mode} selected={article.id === selectedID} width={columnWidth} imageMode={preferences.masonry_image_mode} onOpen={onOpen} onFavorite={onFavorite} onRead={onRead} onShare={onShare} />
        </li>
      })}
    </ol>
    {nextCursor && <div ref={end} aria-hidden="true" className="h-px" />}
    {loadingMore && <p className="library-limit" role="status">正在载入更多文章…</p>}
  </ScrollArea>
}
