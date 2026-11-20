import { useLayoutEffect, useRef } from 'react'
import { createPortal } from 'react-dom'

type Bounds = { top: number; right: number; bottom: number; left: number }

export function selectedTextRange() {
  const selection = window.getSelection()
  return selection?.rangeCount && !selection.isCollapsed ? selection.getRangeAt(0) : undefined
}

function containingElement(node: Node) {
  return node instanceof Element ? node : node.parentElement
}

function proseRange() {
  const range = selectedTextRange()
  if (!range) return
  const surface = containingElement(range.startContainer)?.closest('.chat-timeline, .reader-content')
  if (surface?.contains(range.endContainer)) return range
}

function joinLines(rects: Bounds[], lineHeight: number) {
  const lines: Bounds[] = []
  for (const rect of rects.sort((a, b) => (a.top + a.bottom) - (b.top + b.bottom))) {
    const line = lines.at(-1)
    const overlap = line ? Math.min(line.bottom, rect.bottom) - Math.max(line.top, rect.top) : 0
    if (!line || overlap < Math.min(line.bottom - line.top, rect.bottom - rect.top) / 2) {
      lines.push(rect)
      continue
    }
    line.top = Math.min(line.top, rect.top)
    line.bottom = Math.max(line.bottom, rect.bottom)
    line.left = Math.min(line.left, rect.left)
    line.right = Math.max(line.right, rect.right)
  }

  for (let i = 1; i < lines.length; i++) {
    const previous = lines[i - 1]
    const next = lines[i]
    const gap = next.top - previous.bottom
    const leading = Math.max(lineHeight, Math.min(previous.bottom - previous.top, next.bottom - next.top)) / 2
    if (gap > 0 && gap <= leading) previous.bottom = next.top = Math.round((previous.bottom + next.top) / 2)
  }
  return lines
}

function selectionRects(range: Range) {
  const root = range.commonAncestorContainer
  if (!root.isConnected) return []
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT, node => {
    if (!range.intersectsNode(node)) return NodeFilter.FILTER_REJECT
    return node instanceof Text ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_SKIP
  })
  const viewport = { top: 0, left: 0, right: window.innerWidth, bottom: window.innerHeight }
  const clips = new Map<Element, Bounds>()
  const styles = new Map<Element, CSSStyleDeclaration>()
  function styleFor(element: Element) {
    const cached = styles.get(element)
    if (cached) return cached
    const style = getComputedStyle(element)
    styles.set(element, style)
    return style
  }

  function lineBlock(element: Element): Element {
    const display = styleFor(element).display
    return element.parentElement && (display.startsWith('inline') || display === 'contents')
      ? lineBlock(element.parentElement) : element
  }

  function clipFor(element: Element | null): Bounds {
    if (!element || element === document.body) return viewport
    const cached = clips.get(element)
    if (cached) return cached
    const parent = clipFor(element.parentElement)
    const { overflowX, overflowY } = styleFor(element)
    const clipsX = overflowX && overflowX !== 'visible'
    const clipsY = overflowY && overflowY !== 'visible'
    const bounds = { ...parent }
    if (clipsX || clipsY) {
      const box = element.getBoundingClientRect()
      if (clipsX) {
        bounds.left = Math.max(parent.left, box.left + element.clientLeft)
        bounds.right = Math.min(parent.right, box.left + element.clientLeft + element.clientWidth)
      }
      if (clipsY) {
        bounds.top = Math.max(parent.top, box.top + element.clientTop)
        bounds.bottom = Math.min(parent.bottom, box.top + element.clientTop + element.clientHeight)
      }
    }
    clips.set(element, bounds)
    return bounds
  }

  const textRange = document.createRange()
  const blocks = new Map<Element, Bounds[]>()
  for (let node = root instanceof Text ? root : walker.nextNode(); node; node = walker.nextNode()) {
    if (!(node instanceof Text) || !node.parentElement || /^[\n\r]*$/.test(node.data)) continue
    const clip = clipFor(node.parentElement)
    if (clip.right <= clip.left || clip.bottom <= clip.top) continue
    const block = lineBlock(node.parentElement)
    let rects = blocks.get(block)
    if (!rects) blocks.set(block, rects = [])
    textRange.setStart(node, node === range.startContainer ? range.startOffset : 0)
    textRange.setEnd(node, node === range.endContainer ? range.endOffset : node.length)
    for (const rect of textRange.getClientRects()) {
      const top = Math.max(clip.top, rect.top)
      const bottom = Math.min(clip.bottom, rect.bottom)
      const left = Math.max(clip.left, rect.left)
      const right = Math.min(clip.right, rect.right)
      if (right > left && bottom > top) rects.push({ left, top, right, bottom })
    }
  }
  return Array.from(blocks).flatMap(([block, rects]) => {
    const style = styleFor(block)
    return joinLines(rects, parseFloat(style.lineHeight) || 1.2 * parseFloat(style.fontSize) || 0)
  })
}

export function TextSelection({ menuRange }: { menuRange?: Range }) {
  const overlay = useRef<HTMLDivElement>(null)

  useLayoutEffect(() => {
    let frame = 0
    function paint() {
      frame = 0
      const range = menuRange ?? proseRange()
      const rects = range && !range.collapsed && typeof range.getClientRects === 'function' ? selectionRects(range) : []
      const fragments = rects.map(({ left, top, right, bottom }) => {
        const fragment = document.createElement('span')
        Object.assign(fragment.style, {
          left: `${left}px`, top: `${top}px`, width: `${right - left}px`, height: `${bottom - top}px`,
        })
        return fragment
      })
      overlay.current?.replaceChildren(...fragments)
      document.documentElement.toggleAttribute('data-text-selection-painted', rects.length > 0)
    }
    function queuePaint() {
      if (!frame) frame = requestAnimationFrame(paint)
    }

    paint()
    document.addEventListener('selectionchange', queuePaint)
    window.addEventListener('scroll', queuePaint, true)
    window.addEventListener('resize', queuePaint)
    return () => {
      cancelAnimationFrame(frame)
      document.removeEventListener('selectionchange', queuePaint)
      window.removeEventListener('scroll', queuePaint, true)
      window.removeEventListener('resize', queuePaint)
      document.documentElement.removeAttribute('data-text-selection-painted')
    }
  }, [menuRange])

  return createPortal(<div ref={overlay} className="text-selection" aria-hidden="true" />, document.body)
}
