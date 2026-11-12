import { useCallback, useEffect, useId, useRef, useState } from 'react'
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
  const scrollFrame = useRef<number | null>(null)
  const cancelScroll = useCallback(() => {
    if (scrollFrame.current !== null) cancelAnimationFrame(scrollFrame.current)
    scrollFrame.current = null
  }, [])

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
        if (scrollFrame.current !== null) return
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
    const interrupts = ['wheel', 'touchstart', 'pointerdown', 'keydown']
    for (const type of interrupts) scroll.addEventListener(type, cancelScroll, { passive: true })
    const observer = new ResizeObserver(update)
    observer.observe(scroll)
    if (prose.current) observer.observe(prose.current)
    return () => {
      active = false
      cancelAnimationFrame(frame)
      cancelScroll()
      scroll.removeEventListener('scroll', update)
      for (const type of interrupts) scroll.removeEventListener(type, cancelScroll)
      observer.disconnect()
    }
  }, [cancelScroll, html, mode, prose, viewport])

  useEffect(() => {
    if (inert) cancelScroll()
  }, [cancelScroll, inert])

  if (!snapshot?.headings.length || snapshot.html !== html || mode === 'hidden') return null
  const { headings } = snapshot

  function jump(heading: HTMLHeadingElement) {
    const scroll = viewport.current
    if (!scroll || !heading.isConnected) return
    cancelScroll()
    const start = scroll.scrollTop
    const offset = heading.getBoundingClientRect().top - scroll.getBoundingClientRect().top - 20
    const target = Math.min(Math.max(0, scroll.scrollHeight - scroll.clientHeight), Math.max(0, start + offset))
    heading.tabIndex = -1
    heading.focus({ preventScroll: true })
    setCurrent(heading)
    setOpen(false)

    const motion = document.documentElement.dataset.motion
    const distance = target - start
    if (motion === 'off' || Math.abs(distance) < 2) {
      scroll.scrollTo({ top: target, behavior: 'instant' })
      return
    }

    // 远处先定位到目标附近，只绘制最后一段滚动。
    const finalTravel = Math.max(160, scroll.clientHeight * .9)
    const far = Math.abs(distance) > finalTravel * 1.5
    const animationStart = far ? target - Math.sign(distance) * finalTravel : start
    const duration = motion === 'subtle' ? 100 : far ? 340 : 220
    if (far) scroll.scrollTo({ top: animationStart, behavior: 'instant' })
    let startedAt: number | undefined
    const step = (now: number) => {
      startedAt ??= now
      const progress = Math.min(1, (now - startedAt) / duration)
      const eased = 1 - (1 - progress) ** 3
      scroll.scrollTo({ top: animationStart + (target - animationStart) * eased, behavior: 'instant' })
      scrollFrame.current = progress < 1 ? requestAnimationFrame(step) : null
    }
    scrollFrame.current = requestAnimationFrame(step)
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
