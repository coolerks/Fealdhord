import { useLayoutEffect, useRef } from 'react'
import { createPortal } from 'react-dom'

type Bounds = Pick<DOMRectReadOnly, 'top' | 'right' | 'bottom' | 'left'>

export function selectedTextRange() {
  const selection = window.getSelection()
  return selection?.rangeCount && !selection.isCollapsed ? selection.getRangeAt(0) : undefined
}

function containingElement(node: Node) {
  return node instanceof Element ? node : node.parentElement
}

function chatRange() {
  const range = selectedTextRange()
  if (!range) return
  const timeline = containingElement(range.startContainer)?.closest('.chat-timeline')
  if (timeline && timeline.contains(range.endContainer)) return range
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
  function clipFor(element: Element | null): Bounds {
    if (!element || element === document.body) return viewport
    const cached = clips.get(element)
    if (cached) return cached
    const parent = clipFor(element.parentElement)
    const { overflowX, overflowY } = getComputedStyle(element)
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
  const rects: Bounds[] = []
  for (let node = root instanceof Text ? root : walker.nextNode(); node; node = walker.nextNode()) {
    if (!(node instanceof Text) || /^[\n\r]*$/.test(node.data)) continue
    const clip = clipFor(node.parentElement)
    if (clip.right <= clip.left || clip.bottom <= clip.top) continue
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
  return rects
}

export function TextSelection({ menuRange }: { menuRange?: Range }) {
  const overlay = useRef<HTMLDivElement>(null)

  useLayoutEffect(() => {
    let frame = 0
    function paint() {
      frame = 0
      const range = menuRange ?? chatRange()
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
