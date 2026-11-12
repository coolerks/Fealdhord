import { useCallback, useEffect, useEffectEvent, useLayoutEffect, useRef, useState } from 'react'
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
  const [index, setIndex] = useState(initialIndex)
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

    const picture = node.querySelector<HTMLImageElement>('.reader-image-canvas img')
    const source = image.element
    const destination = source.getBoundingClientRect()
    const bounds = node.getBoundingClientRect()
    const visible = destination.width > 0 && destination.height > 0 &&
      destination.left < bounds.right && destination.right > bounds.left &&
      destination.top < bounds.bottom && destination.bottom > bounds.top
    let returning: Animation | undefined
    if (motion !== 'subtle' && entered && picture?.animate && visible) {
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
    if (cleanupExit.current) return
    const next = index + direction
    if (next < 0 || next >= images.length) return
    setEntry(undefined)
    setIndex(next)
    setZoomImmediately(100)
  }

  const wheel = useEffectEvent((delta: number) => {
    if (cleanupExit.current) return
    revealControls()
    if (wheelMode === 'navigate') stepImage(Math.sign(delta))
    else scaleZoom(Math.exp(-delta * 0.0018))
  })

  useEffect(() => {
    const node = viewer.current
    if (!node) return
    let lastNavigation = -Infinity
    const handleWheel = (event: WheelEvent) => {
      if (!event.deltaY) return
      event.preventDefault()
      const delta = event.deltaY * (event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? node.clientHeight : 1)
      if (wheelMode === 'navigate') {
        if (event.timeStamp - lastNavigation < 200) return
        lastNavigation = event.timeStamp
      }
      wheel(delta)
    }
    node.addEventListener('wheel', handleWheel, { passive: false })
    return () => node.removeEventListener('wheel', handleWheel)
  }, [wheelMode])

  useLayoutEffect(() => {
    viewer.current?.focus({ preventScroll: true })
    clearTimeout(timer.current)
    timer.current = setTimeout(() => setControlsVisible(false), 2800)
    return () => {
      clearTimeout(timer.current)
      stopZoom()
      cleanupExit.current?.()
    }
  }, [stopZoom])

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
        <ZoomedImage key={index} image={image} zoom={zoom} entry={entry} onReady={setEntered}
          onBackdropClick={backdropClose ? close : undefined} />
        <div className="reader-image-controls" data-visible={controlsVisible} inert={leaving}>
          <Button variant="ghost" size="icon-sm" aria-label="上一张" disabled={index === 0} onClick={() => stepImage(-1)}><ChevronLeft /></Button>
          <span className="reader-image-count">{index + 1} / {images.length}</span>
          <Button variant="ghost" size="icon-sm" aria-label="下一张" disabled={index === images.length - 1} onClick={() => stepImage(1)}><ChevronRight /></Button>
          <div className="reader-image-scale">
            <Slider value={[zoom]} min={minZoom} max={maxZoom} step={0.1} thumbProps={{ 'aria-label': '图片缩放', 'aria-valuetext': `${Math.round(zoom)}%` }}
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
        <ContextMenuItem disabled={zoom >= maxZoom} onSelect={() => scaleZoom(1.25)}><Plus />放大</ContextMenuItem>
        <ContextMenuItem disabled={zoom <= minZoom} onSelect={() => scaleZoom(1 / 1.25)}><Minus />缩小</ContextMenuItem>
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

function ZoomedImage({ image, zoom, entry, onReady, onBackdropClick }: {
  image: ReaderImage
  zoom: number
  entry?: DOMRectReadOnly
  onReady: (ready: boolean) => void
  onBackdropClick?: () => void
}) {
  const scroll = useRef<HTMLDivElement>(null)
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
    element.scrollLeft = (element.scrollWidth - element.clientWidth) / 2
    element.scrollTop = (element.scrollHeight - element.clientHeight) / 2
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

  return <div className="reader-image-scroll" ref={scroll} data-fitted={zoom <= 100}>
    <div className="reader-image-canvas" onClick={event => {
      if (event.target === event.currentTarget) onBackdropClick?.()
    }}>
      {failed ? <p role="alert">图片无法显示</p> : <img ref={picture} src={image.src} alt={image.alt} draggable={false}
        data-entry-pending={Boolean(entry && (!fit || !loaded))}
        style={fit ? { width: natural.width * fit, height: natural.height * fit } : { maxWidth: '100%', maxHeight: '100%' }}
        onError={() => setFailed(true)}
        onLoad={event => {
          setNatural({ width: event.currentTarget.naturalWidth, height: event.currentTarget.naturalHeight })
          setLoaded(true)
        }} />}
    </div>
  </div>
}
