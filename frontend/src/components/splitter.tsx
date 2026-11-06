import { useRef, useState } from 'react'
import type { PointerEvent, KeyboardEvent } from 'react'
import { cn } from '@/lib/utils'

export function Splitter({ label, value, min, max, onResize, onCommit, onCollapse }: { label: string; value: number; min: number; max: number; onResize: (value: number | null) => void; onCommit: (value: number) => void; onCollapse?: () => void }) {
  const drag = useRef<{ x: number; width: number; pointerId: number } | null>(null)
  const [dragging, setDragging] = useState(false)
  const clamp = (next: number) => Math.max(min, Math.min(max, Math.round(next)))
  function finish(event: PointerEvent<HTMLDivElement>, commit: boolean) {
    const active = drag.current
    if (!active || active.pointerId !== event.pointerId) return
    drag.current = null
    setDragging(false)
    if (event.currentTarget.hasPointerCapture?.(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId)
    if (!commit) {
      onResize(null)
      return
    }
    const next = active.width + event.clientX - active.x
    if (onCollapse && next <= min - 32) {
      onResize(null)
      onCollapse()
    } else {
      onCommit(clamp(next))
    }
  }
  function keyboard(event: KeyboardEvent<HTMLDivElement>) {
    let next: number
    switch (event.key) {
      case 'ArrowLeft':
        if (value === min && onCollapse) {
          event.preventDefault()
          onCollapse()
          return
        }
        next = value - (event.shiftKey ? 20 : 8)
        break
      case 'ArrowRight': next = value + (event.shiftKey ? 20 : 8); break
      case 'Home': next = min; break
      case 'End': next = max; break
      default: return
    }
    event.preventDefault(); next = clamp(next); onResize(next); onCommit(next)
  }
  return <div role="separator" aria-label={label} aria-orientation="vertical" aria-valuemin={min} aria-valuemax={max} aria-valuenow={value} tabIndex={0} className={cn('splitter', dragging && 'is-dragging')} onKeyDown={keyboard}
    onPointerDown={event => {
      if (event.button !== 0 || drag.current) return
      drag.current = { x: event.clientX, width: value, pointerId: event.pointerId }
      setDragging(true)
      event.currentTarget.setPointerCapture?.(event.pointerId)
    }}
    onPointerMove={event => {
      const active = drag.current
      if (active && active.pointerId === event.pointerId) onResize(clamp(active.width + event.clientX - active.x))
    }}
    onPointerUp={event => finish(event, true)}
    onPointerCancel={event => finish(event, false)}
    onLostPointerCapture={event => finish(event, false)} />
}
