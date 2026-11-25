import { memo, useEffect, useId, useState } from 'react'
import { Check, Code2, Copy, Download, Eye } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { native } from '@/lib/native'
import { notify } from '@/lib/notifications'
import { SaveMermaidImage } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/api'

function isDarkTheme(): boolean {
  if (typeof document === 'undefined') return false
  const root = document.documentElement
  if (root.classList.contains('dark') || root.dataset.theme === 'dark') return true
  if (root.classList.contains('light') || root.dataset.theme === 'light') return false
  return typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: dark)').matches
}

function parseSvgSize(svg: string): { width: number; height: number } {
  const parser = new DOMParser()
  const doc = parser.parseFromString(svg, 'image/svg+xml')
  const el = doc.querySelector('svg')
  const viewBox = el?.getAttribute('viewBox')?.trim().split(/[\s,]+/).map(Number)
  if (viewBox && viewBox.length === 4 && viewBox[2] > 0 && viewBox[3] > 0) {
    return { width: Math.ceil(viewBox[2]), height: Math.ceil(viewBox[3]) }
  }
  const w = Number.parseFloat(el?.getAttribute('width') ?? '')
  const h = Number.parseFloat(el?.getAttribute('height') ?? '')
  return {
    width: Number.isFinite(w) && w > 0 ? Math.ceil(w) : 800,
    height: Number.isFinite(h) && h > 0 ? Math.ceil(h) : 600,
  }
}

async function svgToPngDataUrl(svg: string, dark: boolean): Promise<string> {
  const { width, height } = parseSvgSize(svg)
  const scale = 2
  const padding = 24
  const canvas = document.createElement('canvas')
  canvas.width = (width + padding * 2) * scale
  canvas.height = (height + padding * 2) * scale
  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('CanvasUnavailable')
  ctx.fillStyle = dark ? '#18181b' : '#ffffff'
  ctx.fillRect(0, 0, canvas.width, canvas.height)

  // Ensure explicit width/height and xmlns on the SVG for clean Image decoding.
  const normalized = svg.includes('xmlns=')
    ? svg
    : svg.replace('<svg', '<svg xmlns="http://www.w3.org/2000/svg"')
  const svgUrl = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(normalized)}`
  const img = await new Promise<HTMLImageElement>((resolve, reject) => {
    const image = new Image()
    image.onload = () => resolve(image)
    image.onerror = () => reject(new Error('SVGDecodeFailed'))
    image.src = svgUrl
  })
  ctx.scale(scale, scale)
  ctx.drawImage(img, padding, padding, width, height)
  return canvas.toDataURL('image/png')
}

function triggerBrowserDownload(filename: string, url: string) {
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
}

export const ChatMermaid = memo(function ChatMermaid({ code, streaming }: { code: string; streaming: boolean }) {
  const rawId = useId()
  const renderId = `mermaid-${rawId.replace(/[^a-zA-Z0-9_-]/g, '')}`
  const [dark, setDark] = useState(() => isDarkTheme())
  const [svg, setSvg] = useState('')
  const [error, setError] = useState('')
  const [showSource, setShowSource] = useState(false)
  const [copied, setCopied] = useState(false)
  const [exporting, setExporting] = useState<'svg' | 'png' | null>(null)

  useEffect(() => {
    if (typeof document === 'undefined' || typeof MutationObserver === 'undefined') return
    const observer = new MutationObserver(() => setDark(isDarkTheme()))
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class', 'data-theme'] })
    return () => observer.disconnect()
  }, [])

  useEffect(() => {
    if (!copied) return
    const timer = window.setTimeout(() => setCopied(false), 1600)
    return () => window.clearTimeout(timer)
  }, [copied])

  useEffect(() => {
    if (streaming || !code.trim()) return
    let cancelled = false
    void import('mermaid').then(async ({ default: mermaid }) => {
      mermaid.initialize({
        startOnLoad: false,
        theme: dark ? 'dark' : 'default',
        securityLevel: 'strict',
      })
      const result = await mermaid.render(`${renderId}-${dark ? 'd' : 'l'}`, code)
      if (!cancelled) {
        setSvg(result.svg)
        setError('')
      }
    }).catch((err: unknown) => {
      if (!cancelled) {
        setSvg('')
        setError(err instanceof Error ? err.message : 'Mermaid 语法渲染失败')
      }
    })
    return () => { cancelled = true }
  }, [code, streaming, dark, renderId])

  async function copySource() {
    try {
      await native.copy(code)
      setCopied(true)
      void notify('已复制 Mermaid 源码')
    } catch {
      void notify('无法复制源码', undefined, true)
    }
  }

  async function download(format: 'svg' | 'png') {
    if (!svg || exporting) return
    setExporting(format)
    try {
      const filename = `mermaid-${Date.now()}.${format}`
      const payload = format === 'svg' ? svg : await svgToPngDataUrl(svg, dark)
      try {
        const saved = await SaveMermaidImage(filename, format, payload)
        if (saved) void notify(`已保存 ${format.toUpperCase()} 图片`)
      } catch (err) {
        const message = err instanceof Error ? err.message : ''
        if (message.includes('Unavailable') || message.includes('Wails')) {
          const url = format === 'svg'
            ? URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml;charset=utf-8' }))
            : payload
          triggerBrowserDownload(filename, url)
          if (format === 'svg') window.setTimeout(() => URL.revokeObjectURL(url), 1000)
          void notify(`已下载 ${format.toUpperCase()} 图片`)
        } else {
          throw err
        }
      }
    } catch {
      void notify(`导出 ${format.toUpperCase()} 失败`, undefined, true)
    } finally {
      setExporting(null)
    }
  }

  return <div className="chat-mermaid-block">
    <div className="chat-mermaid-toolbar">
      <span className="text-xs font-medium text-muted-foreground">Mermaid</span>
      <div className="ml-auto flex items-center gap-1">
        {svg && !streaming && <>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button type="button" variant="ghost" size="xs" className="h-6 gap-1 px-2 text-xs"
                aria-label={showSource ? '查看图表' : '查看源码'} onClick={() => setShowSource(value => !value)}>
                {showSource ? <Eye className="size-3.5" /> : <Code2 className="size-3.5" />}
                <span>{showSource ? '图表' : '源码'}</span>
              </Button>
            </TooltipTrigger>
            <TooltipContent>{showSource ? '切换为图表预览' : '查看 Mermaid 源码'}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button type="button" variant="ghost" size="xs" className="h-6 gap-1 px-2 text-xs"
                aria-label="下载 SVG" disabled={exporting !== null} onClick={() => void download('svg')}>
                <Download className="size-3.5" />
                <span>SVG</span>
              </Button>
            </TooltipTrigger>
            <TooltipContent>下载 SVG 矢量图</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button type="button" variant="ghost" size="xs" className="h-6 gap-1 px-2 text-xs"
                aria-label="下载 PNG" disabled={exporting !== null} onClick={() => void download('png')}>
                <Download className="size-3.5" />
                <span>PNG</span>
              </Button>
            </TooltipTrigger>
            <TooltipContent>下载 PNG 图片</TooltipContent>
          </Tooltip>
        </>}
        <Tooltip>
          <TooltipTrigger asChild>
            <Button type="button" variant="ghost" size="icon-xs" aria-label="复制 Mermaid 源码" onClick={() => void copySource()}>
              {copied ? <Check /> : <Copy />}
            </Button>
          </TooltipTrigger>
          <TooltipContent>{copied ? '已复制' : '复制源码'}</TooltipContent>
        </Tooltip>
      </div>
    </div>
    {streaming ? (
      <div className="chat-mermaid-source">
        <p className="mb-1.5 text-xs text-muted-foreground">正在生成图表…</p>
        <pre><code>{code}</code></pre>
      </div>
    ) : error || showSource || !svg ? (
      <div className="chat-mermaid-source">
        {error && <p role="alert" className="mb-1.5 text-xs text-destructive">图表渲染失败：{error}</p>}
        <pre><code>{code}</code></pre>
      </div>
    ) : (
      <div className="chat-mermaid-canvas" dangerouslySetInnerHTML={{ __html: svg }} />
    )}
  </div>
})
