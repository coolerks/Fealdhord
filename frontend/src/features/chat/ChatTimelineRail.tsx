import { useEffect, useMemo, useRef, useState } from 'react'
import type { RefObject } from 'react'
import { HoverCard, HoverCardContent, HoverCardTrigger } from '@/components/ui/hover-card'
import { cn } from '@/lib/utils'
import type { Message } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'

export type ChatTurn = {
  id: string
  question: string
  answerPreview: string
  running: boolean
}

function cleanPreview(markdown: string): string {
  return markdown
    .replace(/```[\s\S]*?```/g, ' [代码] ')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/^#{1,6}\s+/gm, '')
    .replace(/^\s*[-*+]\s+/gm, '• ')
    .replace(/[*_~>]/g, '')
    .split('\n')
    .map(line => line.trim())
    .filter(Boolean)
    .slice(0, 4)
    .join('\n')
}

export function buildChatTurns(messages: readonly Message[] | null | undefined): ChatTurn[] {
  if (!messages?.length) return []
  const turns: ChatTurn[] = []
  for (let i = 0; i < messages.length; i++) {
    const current = messages[i]
    if (current.role !== 'user') continue
    const next = messages[i + 1]?.role === 'assistant' ? messages[i + 1] : undefined
    const question = current.text.replace(/\s+/g, ' ').trim() || '（空消息）'
    const answerPreview = next?.text.trim()
      ? cleanPreview(next.text)
      : next?.status === 'running'
        ? '正在回复…'
        : ''
    turns.push({
      id: current.id,
      question,
      answerPreview,
      running: next?.status === 'running',
    })
  }
  return turns
}

function tickWidth(index: number, hoveredIndex: number | null, active: boolean): number {
  if (hoveredIndex !== null) {
    const distance = Math.abs(index - hoveredIndex)
    if (distance === 0) return 26
    if (distance === 1) return 18
    if (distance === 2) return 13
    if (distance === 3) return 10
    return 8
  }
  return active ? 20 : 8
}

export function ChatTimelineRail({
  messages,
  viewport,
}: {
  messages: readonly Message[] | null | undefined
  viewport: RefObject<HTMLDivElement | null>
}) {
  const turns = useMemo(() => buildChatTurns(messages), [messages])
  const railRef = useRef<HTMLDivElement>(null)
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null)
  const [activeID, setActiveID] = useState<string>('')

  const resolvedActiveID = turns.some(turn => turn.id === activeID)
    ? activeID
    : (turns.at(-1)?.id ?? '')

  useEffect(() => {
    const scroll = viewport.current
    if (!scroll || turns.length === 0) return
    let frame = 0
    const update = () => {
      if (frame) cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        const scrollRect = scroll.getBoundingClientRect()
        const cutoff = scrollRect.top + 80
        const nearBottom = scroll.scrollHeight - scroll.scrollTop - scroll.clientHeight < 24
        if (nearBottom) {
          setActiveID(turns[turns.length - 1].id)
          return
        }
        let current = turns[0].id
        for (const turn of turns) {
          const element = scroll.querySelector<HTMLElement>(`[data-message-id="${CSS.escape(turn.id)}"]`)
          if (!element) continue
          if (element.getBoundingClientRect().top > cutoff) break
          current = turn.id
        }
        setActiveID(current)
      })
    }
    update()
    scroll.addEventListener('scroll', update, { passive: true })
    const observer = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(update) : null
    observer?.observe(scroll)
    return () => {
      cancelAnimationFrame(frame)
      scroll.removeEventListener('scroll', update)
      observer?.disconnect()
    }
  }, [turns, viewport])

  useEffect(() => {
    if (hoveredIndex !== null || !resolvedActiveID || !railRef.current) return
    const target = railRef.current.querySelector<HTMLElement>(`[data-rail-turn="${CSS.escape(resolvedActiveID)}"]`)
    target?.scrollIntoView?.({ block: 'nearest' })
  }, [hoveredIndex, resolvedActiveID])

  if (turns.length === 0) return null

  function jumpToTurn(id: string) {
    const scroll = viewport.current
    if (!scroll) return
    const target = scroll.querySelector<HTMLElement>(`[data-message-id="${CSS.escape(id)}"]`)
    if (!target) return
    setActiveID(id)
    const offset = target.getBoundingClientRect().top - scroll.getBoundingClientRect().top - 16
    const nextTop = Math.max(0, Math.min(scroll.scrollHeight - scroll.clientHeight, scroll.scrollTop + offset))
    const motion = document.documentElement.dataset.motion
    scroll.scrollTo({ top: nextTop, behavior: motion === 'off' ? 'instant' : 'smooth' })
  }

  return (
    <nav
      ref={railRef}
      aria-label="对话轮次导航"
      className="chat-timeline-rail"
      onPointerLeave={() => setHoveredIndex(null)}
    >
      <div className="chat-timeline-rail-list">
        {turns.map((turn, index) => {
          const isActive = turn.id === resolvedActiveID
          const isHovered = hoveredIndex === index
          const width = tickWidth(index, hoveredIndex, isActive)
          return (
            <HoverCard key={turn.id} openDelay={60} closeDelay={60}>
              <HoverCardTrigger asChild>
                <button
                  type="button"
                  data-rail-turn={turn.id}
                  aria-label={`跳转到第 ${index + 1} 轮：${turn.question}`}
                  aria-current={isActive ? 'location' : undefined}
                  className="chat-rail-tick-button"
                  onPointerEnter={() => setHoveredIndex(index)}
                  onFocus={() => setHoveredIndex(index)}
                  onBlur={() => setHoveredIndex(current => (current === index ? null : current))}
                  onClick={() => jumpToTurn(turn.id)}
                >
                  <span
                    aria-hidden="true"
                    className={cn(
                      'chat-rail-tick',
                      (isHovered || isActive) ? 'bg-foreground' : 'bg-muted-foreground/40',
                    )}
                    style={{ width }}
                  />
                </button>
              </HoverCardTrigger>
              <HoverCardContent
                side="right"
                align="center"
                sideOffset={10}
                collisionPadding={12}
                className="w-80 rounded-2xl border bg-popover/95 p-3.5 shadow-lg backdrop-blur-xs"
              >
                <p className="truncate text-sm font-semibold text-foreground">{turn.question}</p>
                {turn.answerPreview && (
                  <p className="mt-1.5 line-clamp-3 whitespace-pre-line text-xs leading-relaxed text-muted-foreground">
                    {turn.answerPreview}
                  </p>
                )}
              </HoverCardContent>
            </HoverCard>
          )
        })}
      </div>
    </nav>
  )
}
