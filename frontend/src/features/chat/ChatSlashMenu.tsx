import { useLayoutEffect, useRef } from 'react'
import type { RefObject } from 'react'
import type { LucideIcon } from 'lucide-react'
import { ChevronRight } from 'lucide-react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Command, CommandEmpty, CommandGroup, CommandItem, CommandList } from '@/components/ui/command'

export type SlashPage = 'actions' | 'skills' | 'models' | 'effort'
export type SlashItem = {
  key: string
  label: string
  detail?: string
  aliases?: string[]
  icon: LucideIcon
} & (
  { page: SlashPage } |
  { action: 'new' | 'attach' | 'context' | 'settings' } |
  { action: 'skill' | 'model' | 'effort'; value: string }
)

export function ChatSlashMenu({ composer, label, active, items, onHighlight, onChoose }: {
  composer: RefObject<HTMLTextAreaElement | null>; label: string; active?: SlashItem; items: SlashItem[]
  onHighlight: (key: string) => void; onChoose: (item: SlashItem) => void
}) {
  const list = useRef<HTMLDivElement>(null)
  const activeKey = active?.key
  useLayoutEffect(() => {
    const input = composer.current
    const options = list.current
    if (!input || !options) return
    input.setAttribute('aria-controls', options.id)
    const item = options.querySelector('[data-slash-active]')
    if (item?.id) {
      input.setAttribute('aria-activedescendant', item.id)
      item.scrollIntoView?.({ block: 'nearest' })
    }
    return () => {
      input.removeAttribute('aria-controls')
      input.removeAttribute('aria-activedescendant')
    }
  }, [activeKey, composer])
  return <div className="chat-slash-menu" onMouseDown={event => event.preventDefault()}>
    <Command label={label} value={active?.key ?? ''} shouldFilter={false} onValueChange={onHighlight}>
      <CommandList ref={list} aria-label={label} showScrollbar className="max-h-[min(40dvh,19rem)]">
        <CommandEmpty>没有匹配命令</CommandEmpty>
        <CommandGroup>
          {items.map(item => <CommandItem key={item.key} data-slash-active={activeKey === item.key ? true : undefined} value={item.key}
            aria-label={item.label} aria-description={item.detail} onSelect={() => onChoose(item)}>
            <AnimatedGlyph icon={item.icon} />
            <span className="min-w-0 truncate">{item.label}</span>
            {item.detail && <span className="truncate text-muted-foreground">{item.detail}</span>}
            {'page' in item && <ChevronRight className="ml-auto" aria-hidden="true" />}
          </CommandItem>)}
        </CommandGroup>
      </CommandList>
    </Command>
  </div>
}
