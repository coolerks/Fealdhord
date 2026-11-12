import { useEffect, useId, useState } from 'react'
import type { RefObject } from 'react'
import { Button } from '@/components/ui/button'
import { Kbd } from '@/components/ui/kbd'

const headingSelector = 'h1,h2,h3,h4,h5,h6'

export function ReaderOutline({ mode, html, prose, viewport, inert }: {
  mode: string
  html: string
  prose: RefObject<HTMLDivElement | null>
  viewport: RefObject<HTMLDivElement | null>
  inert: boolean
}) {
  const panelID = useId()
  const [snapshot, setSnapshot] = useState<{ html: string; headings: HTMLHeadingElement[] }>()
  const [current, setCurrent] = useState<HTMLHeadingElement>()
  const [open, setOpen] = useState(false)

  useEffect(() => {
    const scroll = viewport.current
    if (!scroll || mode === 'hidden') return
    const nodes = Array.from(prose.current?.querySelectorAll<HTMLHeadingElement>(headingSelector) ?? [])
      .filter(heading => heading.textContent?.trim())
    if (!nodes.length) return
    let active = true
    let frame = 0
    const update = () => {
      if (frame) cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        const cutoff = scroll.getBoundingClientRect().top + 48
        let activeHeading = nodes[0]
        for (const node of nodes) {
          if (node.getBoundingClientRect().top > cutoff) break
          activeHeading = node
        }
        setCurrent(activeHeading)
      })
    }
    queueMicrotask(() => {
      if (!active) return
      setSnapshot({ html, headings: nodes })
      update()
    })
    scroll.addEventListener('scroll', update, { passive: true })
    const observer = new ResizeObserver(update)
    observer.observe(scroll)
    if (prose.current) observer.observe(prose.current)
    return () => {
      active = false
      cancelAnimationFrame(frame)
      scroll.removeEventListener('scroll', update)
      observer.disconnect()
    }
  }, [html, mode, prose, viewport])

  if (!snapshot?.headings.length || snapshot.html !== html || mode === 'hidden') return null
  const { headings } = snapshot

  function jump(heading: HTMLHeadingElement) {
    const scroll = viewport.current
    if (!scroll || !heading.isConnected) return
    scroll.scrollTo({ top: scroll.scrollTop + heading.getBoundingClientRect().top - scroll.getBoundingClientRect().top - 20 })
    heading.tabIndex = -1
    heading.focus({ preventScroll: true })
    setCurrent(heading)
    setOpen(false)
  }

  return <aside className="reader-outline" data-mode={mode} data-open={open} inert={inert} aria-hidden={inert || undefined}
    onPointerEnter={event => { if (event.pointerType !== 'touch') setOpen(true) }}
    onPointerLeave={event => {
      if (!event.currentTarget.contains(document.activeElement)) setOpen(false)
    }}
    onBlur={event => {
      if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false)
    }}
    onKeyDown={event => {
      if (event.key !== 'Escape' || !open) return
      event.stopPropagation()
      event.currentTarget.querySelector<HTMLButtonElement>('.reader-outline-ticks')?.focus()
      setOpen(false)
    }}>
    <Button className="reader-outline-ticks" variant="ghost" aria-label="文章目录" aria-expanded={open} aria-controls={panelID}
      onClick={() => setOpen(true)}>
      {headings.map((heading, index) => <span key={index} aria-hidden="true" data-current={heading === current} />)}
    </Button>
    <nav id={panelID} className="reader-outline-panel" aria-label="文章目录">
      <ul>
        {headings.map((heading, index) => <li key={index}>
          <Button type="button" variant="ghost" aria-current={heading === current ? 'location' : undefined}
            onClick={() => jump(heading)}>
            <span className="reader-outline-label">{heading.textContent?.trim()}</span>
            <Kbd>{heading.tagName}</Kbd>
          </Button>
        </li>)}
      </ul>
    </nav>
  </aside>
}
