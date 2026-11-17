import { useCallback, useEffect, useEffectEvent, useLayoutEffect, useRef, useState } from 'react'
import type { PointerEvent as ReactPointerEvent } from 'react'
import { ChevronLeft, ChevronRight, Copy, Minus, Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ContextMenu, ContextMenuContent, ContextMenuGroup, ContextMenuItem, ContextMenuSeparator, ContextMenuTrigger } from '@/components/ui/context-menu'
import { Slider } from '@/components/ui/slider'
import { copyReaderImage } from '@/lib/reader-image'

export type ReaderImage = { id: string; src: string; alt: string; element: HTMLImageElement }

const minZoom = 25
const maxZoom = 800
const clampZoom = (zoom: number) => Math.max(minZoom, Math.min(maxZoom, zoom))
const imageEasing = 'cubic-bezier(.22,1,.36,1)'

function hasImageOverflow(scroll: Element) {
  return scroll.getAttribute('data-fitted') === 'false' &&
    (scroll.scrollWidth > scroll.clientWidth || scroll.scrollHeight > scroll.clientHeight)
}

type ReaderInputWindow = Window & {
  webkit?: { messageHandlers?: { fealdhordReaderTrackpad?: { postMessage: (enabled: boolean) => void } } }
  fealdhordReaderScrollSource?: (time: number, trackpad: boolean) => void
}

function imageTransform(destination: DOMRectReadOnly, layout: DOMRectReadOnly) {
  return `translate(${destination.left - layout.left}px, ${destination.top - layout.top}px) scale(${destination.width / layout.width}, ${destination.height / layout.height})`
}

export function ReaderImageViewer({ images, initialIndex, origin, wheelMode, backdropClose, onClose }: {
  images: ReaderImage[]
  initialIndex: number
  origin: DOMRectReadOnly
  wheelMode: string
  backdropClose: boolean
  onClose: () => void
}) {
  const viewer = useRef<HTMLElement>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const cleanupExit = useRef<(() => void) | null>(null)
  const cleanupSlide = useRef<(() => void) | null>(null)
  const [index, setIndex] = useState(initialIndex)
  const [slide, setSlide] = useState<{ index: number; direction: number }>()
  const [entry, setEntry] = useState(origin.width > 0 && origin.height > 0 ? origin : undefined)
  const [entered, setEntered] = useState(false)
  const [zoom, setZoom] = useState(100)
  const zoomMotion = useRef({ value: 100, target: 100, frame: 0 })
  const [controlsVisible, setControlsVisible] = useState(true)
  const [leaving, setLeaving] = useState(false)
  const image = images[index]

  const stopZoom = useCallback(() => {
    const motion = zoomMotion.current
    cancelAnimationFrame(motion.frame)
    motion.frame = 0
    motion.target = motion.value
  }, [])

  function setZoomImmediately(value: number) {
    stopZoom()
    zoomMotion.current.value = value
    zoomMotion.current.target = value
    setZoom(value)
  }

  function scaleZoom(factor: number) {
    const motion = zoomMotion.current
    motion.target = clampZoom(motion.target * factor)
    const preference = document.documentElement.dataset.motion
    if (preference === 'off') {
      setZoomImmediately(motion.target)
      return
    }
    if (motion.frame) return

    const smoothing = preference === 'subtle' ? 15 : 30
    let lastFrame: number | undefined
    function advance(time: number) {
      const elapsed = lastFrame === undefined ? 16 : time - lastFrame
      lastFrame = time
      const remaining = motion.target - motion.value
      motion.value += remaining * (1 - Math.exp(-elapsed / smoothing))
      const settled = Math.abs(motion.target - motion.value) < 0.05
      if (settled) motion.value = motion.target
      setZoom(motion.value)
      motion.frame = settled ? 0 : requestAnimationFrame(advance)
    }
    motion.frame = requestAnimationFrame(advance)
  }

  function close() {
    if (cleanupExit.current) return
    stopZoom()
    const node = viewer.current
    const motion = document.documentElement.dataset.motion
    if (!node?.animate || motion === 'off') {
      onClose()
      return
    }

    const picture = node.querySelector<HTMLImageElement>(`[data-image-index="${index}"] .reader-image-canvas img`)
    const source = image.element
    const destination = source.getBoundingClientRect()
    const bounds = node.getBoundingClientRect()
    const visible = destination.width > 0 && destination.height > 0 &&
      destination.left < bounds.right && destination.right > bounds.left &&
      destination.top < bounds.bottom && destination.bottom > bounds.top
    let returning: Animation | undefined
    if (motion !== 'subtle' && !slide && entered && picture?.animate && visible) {
      const current = picture.getBoundingClientRect()
      const radius = getComputedStyle(picture).borderRadius
      for (const animation of picture.getAnimations?.() ?? []) animation.cancel()
      const layout = picture.getBoundingClientRect()
      if (current.width && current.height && layout.width && layout.height) {
        source.classList.add('reader-image-source-returning')
        returning = picture.animate([
          { transform: imageTransform(current, layout), borderRadius: radius },
          { transform: imageTransform(destination, layout), borderRadius: '6px' },
        ], { duration: 240, easing: imageEasing, fill: 'forwards' })
      }
    }
    const exit = node.animate(returning
      ? [{ backgroundColor: getComputedStyle(node).backgroundColor }, { backgroundColor: 'transparent' }]
      : [{ opacity: 1 }, { opacity: 0 }],
    { duration: motion === 'subtle' ? 100 : 240, easing: imageEasing, fill: 'forwards' })
    cleanupExit.current = () => {
      exit.onfinish = null
      exit.cancel()
      returning?.cancel()
      if (returning) source.classList.remove('reader-image-source-returning')
    }
    setLeaving(true)
    exit.onfinish = onClose
  }

  function revealControls() {
    if (cleanupExit.current) return
    clearTimeout(timer.current)
    setControlsVisible(true)
    timer.current = setTimeout(() => setControlsVisible(false), 2800)
  }

  function stepImage(direction: number) {
    if (cleanupExit.current || slide) return
    const next = index + direction
    if (next < 0 || next >= images.length) return
    setEntry(undefined)
    const motion = document.documentElement.dataset.motion
    if (viewer.current?.animate && motion !== 'off' && motion !== 'subtle') {
      stopZoom()
      setSlide({ index: next, direction })
      return
    }
    setIndex(next)
    setZoomImmediately(100)
  }

  function playSlide() {
    if (!slide || cleanupSlide.current || cleanupExit.current) return
    const outgoing = viewer.current?.querySelector<HTMLElement>(`[data-image-index="${index}"]`)
    const incoming = viewer.current?.querySelector<HTMLElement>(`[data-image-index="${slide.index}"]`)
    if (!outgoing || !incoming) return
    incoming.style.visibility = 'visible'
    const options: KeyframeAnimationOptions = { duration: 220, easing: imageEasing, fill: 'both' }
    const out = outgoing.animate([
      { transform: 'translateX(0)' },
      { transform: `translateX(${-100 * slide.direction}%)` },
    ], options)
    const into = incoming.animate([
      { transform: `translateX(${100 * slide.direction}%)` },
      { transform: 'translateX(0)' },
    ], options)
    cleanupSlide.current = () => {
      into.onfinish = null
      out.cancel()
      into.cancel()
      incoming.style.removeProperty('visibility')
    }
    into.onfinish = () => {
      if (cleanupExit.current) return
      setIndex(slide.index)
      setZoomImmediately(100)
      setSlide(undefined)
    }
  }

  const wheel = useEffectEvent((delta: number, pinch: boolean) => {
    if (cleanupExit.current || slide) return
    revealControls()
    if (pinch) {
      scaleZoom(Math.exp(-delta / 100))
      return
    }
    if (wheelMode === 'navigate') stepImage(Math.sign(delta))
    else scaleZoom(Math.exp(-delta * 0.0018))
  })

  const magnify = useEffectEvent((value: number) => {
    if (cleanupExit.current || slide) return
    revealControls()
    setZoomImmediately(clampZoom(value))
  })

  useEffect(() => {
    const node = viewer.current
    if (!node) return
    const host: ReaderInputWindow = window
    const source = host.webkit?.messageHandlers?.fealdhordReaderTrackpad
    // 配对 AppKit 和 DOM 中同一事件的时间戳，余下事件沿用鼠标滚轮行为。
    const input: { time: number; trackpad: boolean }[] = []
    if (source) {
      host.fealdhordReaderScrollSource = (time, trackpad) => {
        input.push({ time, trackpad })
        if (input.length > 16) input.shift()
      }
      source.postMessage(true)
    }
    let lastNavigation = -Infinity
    let gestureZoom: number | undefined
    const handleWheel = (event: WheelEvent) => {
      const time = event.timeStamp > 1e12 ? event.timeStamp : performance.timeOrigin + event.timeStamp
      let closest = 4
      let match = -1
      for (const [index, sample] of input.entries()) {
        const distance = Math.abs(sample.time - time)
        if (distance <= closest) {
          closest = distance
          match = index
        }
      }
      const trackpad = match >= 0 && input[match].trackpad
      if (match >= 0) input.splice(0, match + 1)
      const scroll = event.target instanceof Element && event.target.closest('.reader-image-scroll')
      if (!event.ctrlKey && !source && event.deltaX && scroll && hasImageOverflow(scroll)) return
      if (!event.ctrlKey && trackpad && scroll) {
        if (!hasImageOverflow(scroll)) event.preventDefault()
        return
      }
      if (!event.deltaY) return
      event.preventDefault()
      if (gestureZoom !== undefined) return
      const delta = event.deltaY * (event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? node.clientHeight : 1)
      if (!event.ctrlKey && wheelMode === 'navigate') {
        if (event.timeStamp - lastNavigation < 200) return
        lastNavigation = event.timeStamp
      }
      wheel(delta, event.ctrlKey)
    }
    const gestureStart = (event: Event) => {
      event.preventDefault()
      stopZoom()
      gestureZoom = zoomMotion.current.value
    }
    const gestureChange = (event: Event) => {
      event.preventDefault()
      // WKWebView 的 scale 是相对本次手势起点的累计比例。
      const scale = (event as Event & { scale?: number }).scale
      if (gestureZoom !== undefined && typeof scale === 'number' && Number.isFinite(scale) && scale > 0) {
        magnify(gestureZoom * scale)
      }
    }
    const gestureEnd = (event: Event) => {
      event.preventDefault()
      gestureZoom = undefined
    }
    node.addEventListener('wheel', handleWheel, { passive: false })
    node.addEventListener('gesturestart', gestureStart)
    node.addEventListener('gesturechange', gestureChange)
    node.addEventListener('gestureend', gestureEnd)
    return () => {
      if (source) {
        source.postMessage(false)
        delete host.fealdhordReaderScrollSource
      }
      node.removeEventListener('wheel', handleWheel)
      node.removeEventListener('gesturestart', gestureStart)
      node.removeEventListener('gesturechange', gestureChange)
      node.removeEventListener('gestureend', gestureEnd)
    }
  }, [wheelMode, stopZoom])

  useLayoutEffect(() => {
    viewer.current?.focus({ preventScroll: true })
    clearTimeout(timer.current)
    timer.current = setTimeout(() => setControlsVisible(false), 2800)
    return () => {
      clearTimeout(timer.current)
      stopZoom()
      cleanupExit.current?.()
      cleanupSlide.current?.()
    }
  }, [stopZoom])

  useLayoutEffect(() => {
    if (slide) return
    cleanupSlide.current?.()
    cleanupSlide.current = null
  }, [slide])

  return <ContextMenu>
    <ContextMenuTrigger asChild>
      <section ref={viewer} className="reader-image-viewer" data-reader-image-viewer="" data-entered={entered || !entry} data-leaving={leaving}
        aria-label="看图模式" tabIndex={-1}
        onPointerMove={revealControls} onFocus={revealControls}
        onKeyDown={event => {
          revealControls()
          if (event.key === 'Escape') {
            event.stopPropagation()
            close()
          } else if (event.target === event.currentTarget && ['ArrowLeft', 'ArrowRight'].includes(event.key)) {
            event.preventDefault()
            event.stopPropagation()
            stepImage(event.key === 'ArrowLeft' ? -1 : 1)
          }
        }}>
        <div className="reader-image-stage">
          {(slide ? [index, slide.index] : [index]).map(page => {
            const pending = page === slide?.index
            return <div className="reader-image-slide" key={page} data-image-index={page}
              data-pending={pending} inert={pending} aria-hidden={pending || undefined}>
              <ZoomedImage image={images[page]} zoom={pending ? 100 : zoom} entry={pending ? undefined : entry}
                onReady={setEntered} onAvailable={pending ? playSlide : undefined}
                onBackdropClick={backdropClose ? close : undefined} />
            </div>
          })}
        </div>
        <div className="reader-image-controls" data-visible={controlsVisible} inert={leaving}>
          <Button variant="ghost" size="icon-sm" aria-label="上一张" disabled={index === 0} onClick={() => stepImage(-1)}><ChevronLeft /></Button>
          <span className="reader-image-count">{index + 1} / {images.length}</span>
          <Button variant="ghost" size="icon-sm" aria-label="下一张" disabled={index === images.length - 1} onClick={() => stepImage(1)}><ChevronRight /></Button>
          <div className="reader-image-scale">
            <Slider value={[zoom]} min={minZoom} max={maxZoom} step={0.1} disabled={Boolean(slide)}
              thumbProps={{ 'aria-label': '图片缩放', 'aria-valuetext': `${Math.round(zoom)}%` }}
              onValueChange={([value]) => setZoomImmediately(value)} />
            <span>{Math.round(zoom)}%</span>
          </div>
          <Button variant="ghost" size="icon-sm" aria-label="退出看图" onClick={close}><X /></Button>
        </div>
      </section>
    </ContextMenuTrigger>
    <ContextMenuContent aria-label="看图操作"
      onCloseAutoFocus={event => {
        event.preventDefault()
        viewer.current?.focus({ preventScroll: true })
      }}
      onEscapeKeyDown={event => {
        event.preventDefault()
        close()
      }}>
      <ContextMenuGroup>
        <ContextMenuItem onSelect={() => { void copyReaderImage(image.id) }}><Copy />复制图片</ContextMenuItem>
        <ContextMenuItem disabled={Boolean(slide) || zoom >= maxZoom} onSelect={() => scaleZoom(1.25)}><Plus />放大</ContextMenuItem>
        <ContextMenuItem disabled={Boolean(slide) || zoom <= minZoom} onSelect={() => scaleZoom(1 / 1.25)}><Minus />缩小</ContextMenuItem>
      </ContextMenuGroup>
      <ContextMenuSeparator />
      <ContextMenuGroup>
        <ContextMenuItem disabled={index === 0} onSelect={() => stepImage(-1)}><ChevronLeft />上一张</ContextMenuItem>
        <ContextMenuItem disabled={index === images.length - 1} onSelect={() => stepImage(1)}><ChevronRight />下一张</ContextMenuItem>
      </ContextMenuGroup>
      <ContextMenuSeparator />
      <ContextMenuItem onSelect={close}><X />退出看图</ContextMenuItem>
    </ContextMenuContent>
  </ContextMenu>
}

function ZoomedImage({ image, zoom, entry, onReady, onAvailable, onBackdropClick }: {
  image: ReaderImage
  zoom: number
  entry?: DOMRectReadOnly
  onReady: (ready: boolean) => void
  onAvailable?: () => void
  onBackdropClick?: () => void
}) {
  const scroll = useRef<HTMLDivElement>(null)
  const scrollPosition = useRef({ x: 0.5, y: 0.5 })
  const drag = useRef<{
    id: number; x: number; y: number; left: number; top: number
    moved: boolean; released: boolean; menuPending: boolean
  }>(null)
  const picture = useRef<HTMLImageElement>(null)
  const didEnter = useRef(false)
  const [viewport, setViewport] = useState({ width: 0, height: 0 })
  const [natural, setNatural] = useState({ width: image.element.naturalWidth, height: image.element.naturalHeight })
  const [failed, setFailed] = useState(false)
  const [loaded, setLoaded] = useState(false)
  const fit = natural.width && natural.height && viewport.width && viewport.height
    ? Math.min(1, viewport.width / natural.width, viewport.height / natural.height) * zoom / 100
    : 0

  useLayoutEffect(() => {
    // 滚动条改变内部可滚动尺寸，适配比例只取外层视口尺寸。
    const frame = scroll.current?.parentElement
    if (!frame) return
    const observer = new ResizeObserver(() => setViewport({ width: frame.clientWidth, height: frame.clientHeight }))
    observer.observe(frame)
    return () => observer.disconnect()
  }, [])

  useLayoutEffect(() => {
    const element = scroll.current
    if (!element) return
    element.scrollLeft = (element.scrollWidth - element.clientWidth) * scrollPosition.current.x
    element.scrollTop = (element.scrollHeight - element.clientHeight) * scrollPosition.current.y
  }, [fit])

  useLayoutEffect(() => {
    if (!entry || didEnter.current || (!failed && (!loaded || !fit))) return
    didEnter.current = true
    onReady(true)
    const element = picture.current
    const motion = document.documentElement.dataset.motion
    if (!element || !element.animate || motion === 'off' || motion === 'subtle') return
    const target = element.getBoundingClientRect()
    if (!target.width || !target.height) return
    const source = image.element
    source.classList.add('reader-image-source-hidden')
    const transition = element.animate([
      {
        transform: imageTransform(entry, target),
        borderRadius: '6px',
      },
      { transform: 'translate(0, 0) scale(1)', borderRadius: '0px' },
    ], { duration: 280, easing: imageEasing })
    return () => {
      transition.cancel()
      source.classList.remove('reader-image-source-hidden')
    }
  }, [entry, failed, fit, image.element, loaded, onReady])

  function finishDrag(event: ReactPointerEvent<HTMLDivElement>, cancelled = false) {
    const current = drag.current
    if (!current || current.released || current.id !== event.pointerId) return
    const target = event.currentTarget
    delete target.dataset.dragging
    current.released = true
    if (target.hasPointerCapture?.(event.pointerId)) target.releasePointerCapture(event.pointerId)
    if (current.moved && !cancelled) return
    drag.current = null
    if (!cancelled && current.menuPending) {
      // 部分 WebView 在右键按下时派发菜单事件，松手确认是单击后再打开菜单。
      target.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true, cancelable: true, button: 2, clientX: event.clientX, clientY: event.clientY,
      }))
    }
  }

  return <div className="reader-image-scroll" ref={scroll} data-fitted={zoom <= 100}
    onScroll={({ currentTarget: target }) => {
      const width = target.scrollWidth - target.clientWidth
      const height = target.scrollHeight - target.clientHeight
      scrollPosition.current = { x: width > 0 ? target.scrollLeft / width : 0.5, y: height > 0 ? target.scrollTop / height : 0.5 }
    }}
    onPointerDown={event => {
      const target = event.currentTarget
      drag.current = null
      if (event.button !== 2 || event.target === target || !hasImageOverflow(target)) return
      drag.current = {
        id: event.pointerId, x: event.clientX, y: event.clientY, left: target.scrollLeft, top: target.scrollTop,
        moved: false, released: false, menuPending: false,
      }
      target.dataset.dragging = 'true'
      target.setPointerCapture?.(event.pointerId)
    }}
    onPointerMove={event => {
      const current = drag.current
      if (!current || current.released || current.id !== event.pointerId) return
      const dx = event.clientX - current.x
      const dy = event.clientY - current.y
      if (!current.moved && Math.hypot(dx, dy) < 4) return
      current.moved = true
      event.currentTarget.scrollLeft = current.left - dx
      event.currentTarget.scrollTop = current.top - dy
    }}
    onPointerUp={event => finishDrag(event)}
    onPointerCancel={event => finishDrag(event, true)}
    onLostPointerCapture={event => finishDrag(event, true)}
    onContextMenu={event => {
      const current = drag.current
      if (!current || event.button !== 2) return
      if (!current.released || current.moved) event.preventDefault()
      if (current.released) drag.current = null
      else current.menuPending = true
    }}>
    <div className="reader-image-canvas" onClick={event => {
      if (event.target === event.currentTarget) onBackdropClick?.()
    }}>
      {failed ? <p role="alert">图片无法显示</p> : <img ref={picture} src={image.src} alt={image.alt} draggable={false}
        data-entry-pending={Boolean(entry && (!fit || !loaded))}
        style={fit ? { width: natural.width * fit, height: natural.height * fit } : { maxWidth: '100%', maxHeight: '100%' }}
        onError={() => {
          setFailed(true)
          onAvailable?.()
        }}
        onLoad={event => {
          setNatural({ width: event.currentTarget.naturalWidth, height: event.currentTarget.naturalHeight })
          setLoaded(true)
          onAvailable?.()
        }} />}
    </div>
  </div>
}
