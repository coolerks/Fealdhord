import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { RefObject } from 'react'
import { ChevronDown, ChevronUp, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const HIGHLIGHT_ALL = 'reader-find'
const HIGHLIGHT_ACTIVE = 'reader-find-active'

type HighlightRegistryLike = {
  set: (name: string, highlight: unknown) => void
  delete: (name: string) => void
}

function getHighlightRegistry(): HighlightRegistryLike | null {
  if (typeof CSS === 'undefined' || !('highlights' in CSS)) return null
  return (CSS as unknown as { highlights: HighlightRegistryLike }).highlights
}

function clearHighlights() {
  const registry = getHighlightRegistry()
  if (!registry) return
  registry.delete(HIGHLIGHT_ALL)
  registry.delete(HIGHLIGHT_ACTIVE)
}

function findTextRanges(root: HTMLElement, query: string): Range[] {
  const needle = query.trim().toLocaleLowerCase()
  if (!needle) return []
  const ranges: Range[] = []
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      const parent = node.parentElement
      if (!parent) return NodeFilter.FILTER_REJECT
      if (parent.closest('[aria-hidden="true"], [hidden], script, style, .reader-find-bar')) {
        return NodeFilter.FILTER_REJECT
      }
      return node.nodeValue ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT
    },
  })
  let current: Node | null
  while ((current = walker.nextNode())) {
    const text = current.nodeValue
    if (!text) continue
    const lower = text.toLocaleLowerCase()
    let offset = 0
    while (offset <= lower.length - needle.length) {
      const idx = lower.indexOf(needle, offset)
      if (idx === -1) break
      const range = document.createRange()
      range.setStart(current, idx)
      range.setEnd(current, idx + needle.length)
      ranges.push(range)
      offset = idx + needle.length
      if (ranges.length >= 2000) return ranges
    }
  }
  return ranges
}

function scrollRangeIntoView(range: Range, viewport: HTMLElement | null) {
  const element = range.startContainer.parentElement
  if (!element) return
  if (viewport) {
    const rangeRect = range.getBoundingClientRect()
    const viewRect = viewport.getBoundingClientRect()
    if (rangeRect.height > 0 && viewRect.height > 0) {
      const topOffset = rangeRect.top - viewRect.top + viewport.scrollTop
      const targetTop = Math.max(0, topOffset - viewRect.height * 0.35)
      if (typeof viewport.scrollTo === 'function') {
        viewport.scrollTo({ top: targetTop, behavior: 'smooth' })
        return
      }
    }
  }
  if (typeof element.scrollIntoView === 'function') {
    element.scrollIntoView({ block: 'center', behavior: 'smooth' })
  }
}

export function ReaderFindBar({
  open,
  focusTrigger,
  containerRef,
  viewportRef,
  contentKey,
  onClose,
}: {
  open: boolean
  focusTrigger: number
  containerRef: RefObject<HTMLElement | null>
  viewportRef: RefObject<HTMLElement | null>
  contentKey: string
  onClose: () => void
}) {
  const inputRef = useRef<HTMLInputElement>(null)
  const rangesRef = useRef<Range[]>([])
  const [query, setQuery] = useState('')
  const [matchCount, setMatchCount] = useState(0)
  const [activeIndex, setActiveIndex] = useState(0)

  const paintHighlights = useCallback((ranges: Range[], index: number) => {
    const registry = getHighlightRegistry()
    const HighlightCtor = (globalThis as unknown as { Highlight?: new (...ranges: Range[]) => unknown }).Highlight
    if (!registry || !HighlightCtor) return
    if (ranges.length === 0) {
      registry.delete(HIGHLIGHT_ALL)
      registry.delete(HIGHLIGHT_ACTIVE)
      return
    }
    const clamped = Math.max(0, Math.min(index, ranges.length - 1))
    registry.set(HIGHLIGHT_ALL, new HighlightCtor(...ranges))
    registry.set(HIGHLIGHT_ACTIVE, new HighlightCtor(ranges[clamped]))
  }, [])

  useLayoutEffect(() => {
    if (!open) {
      rangesRef.current = []
      clearHighlights()
      return
    }
    const input = inputRef.current
    if (input) {
      input.focus({ preventScroll: true })
      input.select()
    }
  }, [open, focusTrigger])

  useEffect(() => {
    if (!open) return
    let active = true
    queueMicrotask(() => {
      if (!active) return
      const root = containerRef.current
      const trimmed = query.trim()
      if (!root || !trimmed) {
        rangesRef.current = []
        setMatchCount(0)
        setActiveIndex(0)
        clearHighlights()
        return
      }
      const ranges = findTextRanges(root, trimmed)
      rangesRef.current = ranges
      setMatchCount(ranges.length)
      setActiveIndex(0)
      paintHighlights(ranges, 0)
      if (ranges.length > 0) scrollRangeIntoView(ranges[0], viewportRef.current)
    })
    return () => { active = false }
  }, [open, query, contentKey, containerRef, viewportRef, paintHighlights])

  useEffect(() => () => clearHighlights(), [])

  const step = useCallback((delta: 1 | -1) => {
    const ranges = rangesRef.current
    if (ranges.length === 0) return
    setActiveIndex(current => {
      const next = (current + delta + ranges.length) % ranges.length
      paintHighlights(ranges, next)
      scrollRangeIntoView(ranges[next], viewportRef.current)
      return next
    })
  }, [paintHighlights, viewportRef])

  useEffect(() => {
    if (!open) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        onClose()
      }
    }
    window.addEventListener('keydown', onKeyDown, true)
    return () => window.removeEventListener('keydown', onKeyDown, true)
  }, [open, onClose])

  if (!open) return null

  const trimmed = query.trim()

  return (
    <div
      role="search"
      aria-label="在文章内查找"
      className="reader-find-bar absolute top-2 right-4 z-20 flex items-center gap-1 rounded-xl border border-border bg-popover/95 p-1.5 shadow-md backdrop-blur-sm"
    >
      <Input
        ref={inputRef}
        id="reader-find-input"
        type="search"
        aria-label="查找文章内容"
        placeholder="在文章内查找…"
        value={query}
        onChange={event => setQuery(event.target.value)}
        onKeyDown={event => {
          if (event.nativeEvent.isComposing) return
          if (event.key === 'Enter') {
            event.preventDefault()
            step(event.shiftKey ? -1 : 1)
          } else if (event.key === 'Escape') {
            event.preventDefault()
            event.stopPropagation()
            onClose()
          }
        }}
        className="h-7 w-48 border-0 bg-transparent px-2 text-xs shadow-none focus-visible:ring-0"
      />
      {trimmed && (
        <span
          role="status"
          aria-live="polite"
          className="min-w-12 shrink-0 px-1 text-center text-xs text-muted-foreground tabular-nums"
        >
          {matchCount > 0 ? `${activeIndex + 1} / ${matchCount}` : '无匹配'}
        </span>
      )}
      <Button
        type="button"
        variant="ghost"
        size="icon-xs"
        aria-label="上一个匹配"
        disabled={matchCount === 0}
        onClick={() => step(-1)}
      >
        <ChevronUp className="size-3.5" />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon-xs"
        aria-label="下一个匹配"
        disabled={matchCount === 0}
        onClick={() => step(1)}
      >
        <ChevronDown className="size-3.5" />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon-xs"
        aria-label="关闭查找"
        onClick={onClose}
      >
        <X className="size-3.5" />
      </Button>
    </div>
  )
}
