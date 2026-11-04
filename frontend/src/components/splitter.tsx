import { useRef, useState } from 'react'
import type { PointerEvent, KeyboardEvent } from 'react'
import { cn } from '@/lib/utils'

export function Splitter({ label, value, min, max, onResize, onCommit }: { label: string; value: number; min: number; max: number; onResize: (value: number) => void; onCommit: (value: number) => void }) {
  const drag = useRef<{ x: number; width: number; current: number } | null>(null)
  const [dragging, setDragging] = useState(false)
  const clamp = (next: number) => Math.max(min, Math.min(max, Math.round(next)))
  function finish(event: PointerEvent<HTMLDivElement>) {
    if (!drag.current) return
    const current = drag.current.current
    drag.current = null
    setDragging(false)
    if (event.currentTarget.hasPointerCapture?.(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId)
    onCommit(current)
  }
  function keyboard(event: KeyboardEvent<HTMLDivElement>) {
    let next: number
    switch (event.key) {
      case 'ArrowLeft': next = value - (event.shiftKey ? 20 : 8); break
      case 'ArrowRight': next = value + (event.shiftKey ? 20 : 8); break
      case 'Home': next = min; break
      case 'End': next = max; break
      default: return
    }
    event.preventDefault(); next = clamp(next); onResize(next); onCommit(next)
  }
  return <div role="separator" aria-label={label} aria-orientation="vertical" aria-valuemin={min} aria-valuemax={max} aria-valuenow={value} tabIndex={0} className={cn('splitter', dragging && 'is-dragging')} onKeyDown={keyboard}
    onPointerDown={event => { if (event.button !== 0) return; drag.current = { x: event.clientX, width: value, current: value }; setDragging(true); event.currentTarget.setPointerCapture(event.pointerId) }}
    onPointerMove={event => { if (!drag.current) return; const next = clamp(drag.current.width + event.clientX - drag.current.x); drag.current.current = next; onResize(next) }}
    onPointerUp={finish} onPointerCancel={finish} />
}
