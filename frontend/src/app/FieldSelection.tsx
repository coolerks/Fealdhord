import { useLayoutEffect, useRef } from 'react'
import type { TextTarget } from '@/lib/text-actions'
import { createPortal } from 'react-dom'

export function FieldSelection({ target }: { target: TextTarget }) {
  const mirror = useRef<HTMLDivElement>(null)
  const { field, start, end, value } = target

  useLayoutEffect(() => {
    const element = mirror.current
    if (!element || !field) return

    function align() {
      if (!element || !field) return
      const bounds = field.getBoundingClientRect()
      const style = getComputedStyle(field)
      Object.assign(element.style, {
        left: `${bounds.left}px`, top: `${bounds.top}px`,
        width: `${bounds.width}px`, height: `${bounds.height}px`,
        font: style.font, letterSpacing: style.letterSpacing, textAlign: style.textAlign,
        textIndent: style.textIndent, direction: style.direction, tabSize: style.tabSize,
        padding: style.padding, borderWidth: style.borderWidth, borderStyle: 'solid',
        borderRadius: style.borderRadius,
      })
      element.scrollLeft = field.scrollLeft
      element.scrollTop = field.scrollTop
    }

    align()
    field.addEventListener('scroll', align)
    window.addEventListener('scroll', align, true)
    window.addEventListener('resize', align)
    return () => {
      field.removeEventListener('scroll', align)
      window.removeEventListener('scroll', align, true)
      window.removeEventListener('resize', align)
    }
  }, [field])

  if (!field || start === end) return null
  return createPortal(
    <div ref={mirror} aria-hidden="true" className="field-selection" data-single-line={field instanceof HTMLInputElement}>
      <div>{value.slice(0, start)}<mark>{value.slice(start, end)}</mark>{value.slice(end)}{'\u200b'}</div>
    </div>,
    document.body,
  )
}
