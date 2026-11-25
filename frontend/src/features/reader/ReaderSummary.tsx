import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { ChevronDown } from 'lucide-react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { native } from '@/lib/native'

type SummaryBlock =
  | { kind: 'paragraph'; text: string }
  | { kind: 'list'; items: string[] }

const bulletPattern = /^(?:[-*•·—–]\s+|\d+[.、)]\s+)/

function parseSummaryBlocks(text: string): SummaryBlock[] {
  const lines = text.split(/\r?\n/).map(line => line.trim()).filter(Boolean)
  const blocks: SummaryBlock[] = []
  let list: string[] = []
  const flushList = () => {
    if (!list.length) return
    blocks.push({ kind: 'list', items: list })
    list = []
  }
  for (const line of lines) {
    if (bulletPattern.test(line)) {
      const item = line.replace(bulletPattern, '').trim()
      if (item) list.push(item)
    } else {
      flushList()
      blocks.push({ kind: 'paragraph', text: line })
    }
  }
  flushList()
  return blocks
}

export type ReaderSummaryProps = {
  pending: boolean
  error: string
  needsModel: boolean
  text?: string
  collapsed: boolean
  onToggle: () => void
  onCancel: () => void
  onRetry: () => void
  onConfigureModel: () => void
}

export function ReaderSummary({
  pending, error, needsModel, text, collapsed, onToggle, onCancel, onRetry, onConfigureModel,
}: ReaderSummaryProps) {
  const areaID = useId()
  const [copied, setCopied] = useState(false)
  const [entering, setEntering] = useState(false)
  const previousPending = useRef(pending)
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const enterTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    const justFinished = previousPending.current && !pending && Boolean(text)
    previousPending.current = pending
    if (!justFinished) return
    let active = true
    queueMicrotask(() => {
      if (!active) return
      setEntering(true)
      if (enterTimer.current) clearTimeout(enterTimer.current)
      enterTimer.current = setTimeout(() => { setEntering(false) }, 360)
    })
    return () => { active = false }
  }, [pending, text])

  useEffect(() => () => {
    if (copyTimer.current) clearTimeout(copyTimer.current)
    if (enterTimer.current) clearTimeout(enterTimer.current)
  }, [])

  const blocks = useMemo(() => (text ? parseSummaryBlocks(text) : []), [text])

  function copySummary() {
    if (!text) return
    void native.copy(text).then(() => {
      setCopied(true)
      if (copyTimer.current) clearTimeout(copyTimer.current)
      copyTimer.current = setTimeout(() => { setCopied(false) }, 1600)
    }).catch(() => undefined)
  }

  return <>
    {error && <div className="reader-operation" data-tone="warning">
      <AnimatedGlyph meaning="badge-alert" className="reader-operation-icon" />
      <p role="alert" className="min-w-0 flex-1 m-0">{error}</p>
      {needsModel
        ? <Button variant="outline" size="sm" onClick={onConfigureModel}>配置模型</Button>
        : <Button variant="ghost" size="sm" onClick={onRetry}>重试摘要</Button>}
    </div>}

    {pending && (
      <section aria-label="文章摘要" className="reader-summary" data-state="pending">
        <div className="reader-summary-surface">
          <div className="reader-summary-header">
            <div className="reader-summary-heading">
              <AnimatedGlyph meaning="ai" className="reader-summary-icon" />
              <span className="reader-summary-status chat-thinking-label" role="status">正在生成文章摘要…</span>
            </div>
            <Button type="button" variant="ghost" size="sm" className="reader-summary-cancel" onClick={onCancel}>取消</Button>
          </div>
          <div className="reader-summary-skeleton" aria-hidden="true">
            <span className="reader-summary-skeleton-line" style={{ width: '100%' }} />
            <span className="reader-summary-skeleton-line" style={{ width: '91%' }} />
            <span className="reader-summary-skeleton-line" style={{ width: '62%' }} />
          </div>
        </div>
      </section>
    )}

    {!pending && text && (
      <section
        aria-label="文章摘要"
        className="reader-summary"
        data-state="ready"
        data-collapsed={collapsed ? 'true' : 'false'}
        data-entering={entering ? 'true' : undefined}
      >
        <div className="reader-summary-surface">
          <div className="reader-summary-header">
            <button
              type="button"
              className="reader-summary-toggle"
              aria-expanded={!collapsed}
              aria-controls={areaID}
              onClick={onToggle}
            >
              <AnimatedGlyph meaning="ai" className="reader-summary-icon" />
              <span className="reader-summary-title">摘要</span>
              <ChevronDown className="reader-summary-chevron" aria-hidden="true" />
            </button>
            {!collapsed && (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="reader-summary-copy"
                aria-label={copied ? '摘要已复制' : '复制摘要'}
                title={copied ? '已复制' : '复制摘要'}
                onClick={copySummary}
              >
                <AnimatedGlyph meaning={copied ? 'check' : 'copy'} />
              </Button>
            )}
            {copied && <span className="sr-only" role="status">摘要已复制</span>}
          </div>
          <div id={areaID} className="reader-summary-collapse" inert={collapsed || undefined} aria-hidden={collapsed || undefined}>
            <div className="reader-summary-collapse-inner">
              <div className="reader-summary-body" style={collapsed ? { visibility: 'hidden' } : undefined}>
                {blocks.map((block, index) => block.kind === 'list' ? (
                  <ul key={index} className="reader-summary-list">
                    {block.items.map((item, itemIndex) => <li key={itemIndex}>{item}</li>)}
                  </ul>
                ) : (
                  <p key={index}>{block.text}</p>
                ))}
              </div>
            </div>
          </div>
        </div>
      </section>
    )}
  </>
}
