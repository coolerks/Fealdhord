import { useEffect, useEffectEvent, useLayoutEffect, useRef, useState } from 'react'
import { ChevronLeft, ChevronRight, Copy, Minus, Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ContextMenu, ContextMenuContent, ContextMenuGroup, ContextMenuItem, ContextMenuSeparator, ContextMenuTrigger } from '@/components/ui/context-menu'
import { Slider } from '@/components/ui/slider'
import { copyReaderImage } from '@/lib/reader-image'

export type ReaderImage = { id: string; src: string; alt: string; element: HTMLImageElement }

const minZoom = 25
const maxZoom = 800
const clampZoom = (zoom: number) => Math.max(minZoom, Math.min(maxZoom, zoom))

export function ReaderImageViewer({ images, initialIndex, wheelMode, onClose }: {
  images: ReaderImage[]
  initialIndex: number
  wheelMode: string
  onClose: () => void
}) {
  const viewer = useRef<HTMLElement>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const [index, setIndex] = useState(initialIndex)
  const [zoom, setZoom] = useState(100)
  const [controlsVisible, setControlsVisible] = useState(true)
  const image = images[index]

  function revealControls() {
    clearTimeout(timer.current)
    setControlsVisible(true)
    timer.current = setTimeout(() => setControlsVisible(false), 2800)
  }

  function stepImage(direction: number) {
    const next = index + direction
    if (next < 0 || next >= images.length) return
    setIndex(next)
    setZoom(100)
  }

  const wheel = useEffectEvent((delta: number) => {
    revealControls()
    if (wheelMode === 'navigate') stepImage(Math.sign(delta))
    else setZoom(value => clampZoom(value * Math.exp(-delta * 0.0018)))
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
    return () => clearTimeout(timer.current)
  }, [])

  return <ContextMenu>
    <ContextMenuTrigger asChild>
      <section ref={viewer} className="reader-image-viewer" data-reader-image-viewer="" aria-label="看图模式" tabIndex={-1}
        onPointerMove={revealControls} onFocus={revealControls}
        onKeyDown={event => {
          revealControls()
          if (event.key === 'Escape') {
            event.stopPropagation()
            onClose()
          } else if (event.target === event.currentTarget && ['ArrowLeft', 'ArrowRight'].includes(event.key)) {
            event.preventDefault()
            event.stopPropagation()
            stepImage(event.key === 'ArrowLeft' ? -1 : 1)
          }
        }}>
        <ZoomedImage key={index} image={image} zoom={zoom} />
        <div className="reader-image-controls" data-visible={controlsVisible}>
          <Button variant="ghost" size="icon-sm" aria-label="上一张" disabled={index === 0} onClick={() => stepImage(-1)}><ChevronLeft /></Button>
          <span className="reader-image-count">{index + 1} / {images.length}</span>
          <Button variant="ghost" size="icon-sm" aria-label="下一张" disabled={index === images.length - 1} onClick={() => stepImage(1)}><ChevronRight /></Button>
          <div className="reader-image-scale">
            <Slider value={[zoom]} min={minZoom} max={maxZoom} step={0.1} thumbProps={{ 'aria-label': '图片缩放', 'aria-valuetext': `${Math.round(zoom)}%` }}
              onValueChange={([value]) => setZoom(value)} />
            <span>{Math.round(zoom)}%</span>
          </div>
          <Button variant="ghost" size="icon-sm" aria-label="退出看图" onClick={onClose}><X /></Button>
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
        onClose()
      }}>
      <ContextMenuGroup>
        <ContextMenuItem onSelect={() => { void copyReaderImage(image.id) }}><Copy />复制图片</ContextMenuItem>
        <ContextMenuItem disabled={zoom >= maxZoom} onSelect={() => setZoom(value => clampZoom(value * 1.25))}><Plus />放大</ContextMenuItem>
        <ContextMenuItem disabled={zoom <= minZoom} onSelect={() => setZoom(value => clampZoom(value / 1.25))}><Minus />缩小</ContextMenuItem>
      </ContextMenuGroup>
      <ContextMenuSeparator />
      <ContextMenuGroup>
        <ContextMenuItem disabled={index === 0} onSelect={() => stepImage(-1)}><ChevronLeft />上一张</ContextMenuItem>
        <ContextMenuItem disabled={index === images.length - 1} onSelect={() => stepImage(1)}><ChevronRight />下一张</ContextMenuItem>
      </ContextMenuGroup>
      <ContextMenuSeparator />
      <ContextMenuItem onSelect={onClose}><X />退出看图</ContextMenuItem>
    </ContextMenuContent>
  </ContextMenu>
}

function ZoomedImage({ image, zoom }: { image: ReaderImage; zoom: number }) {
  const scroll = useRef<HTMLDivElement>(null)
  const [viewport, setViewport] = useState({ width: 0, height: 0 })
  const [natural, setNatural] = useState({ width: image.element.naturalWidth, height: image.element.naturalHeight })
  const [failed, setFailed] = useState(false)
  const fit = natural.width && natural.height && viewport.width && viewport.height
    ? Math.min(1, viewport.width / natural.width, viewport.height / natural.height) * zoom / 100
    : 0

  useLayoutEffect(() => {
    const element = scroll.current
    if (!element) return
    const observer = new ResizeObserver(() => setViewport({ width: element.clientWidth, height: element.clientHeight }))
    observer.observe(element)
    return () => observer.disconnect()
  }, [])

  useLayoutEffect(() => {
    const element = scroll.current
    if (!element) return
    element.scrollLeft = (element.scrollWidth - element.clientWidth) / 2
    element.scrollTop = (element.scrollHeight - element.clientHeight) / 2
  }, [fit])

  return <div className="reader-image-scroll" ref={scroll}>
    <div className="reader-image-canvas">
      {failed ? <p role="alert">图片无法显示</p> : <img src={image.src} alt={image.alt} draggable={false}
        style={fit ? { width: natural.width * fit, height: natural.height * fit } : { maxWidth: '100%', maxHeight: '100%' }}
        onError={() => setFailed(true)}
        onLoad={event => setNatural({ width: event.currentTarget.naturalWidth, height: event.currentTarget.naturalHeight })} />}
    </div>
  </div>
}
