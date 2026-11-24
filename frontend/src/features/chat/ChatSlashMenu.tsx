import { useLayoutEffect, useRef } from 'react'
import type { RefObject } from 'react'
import type { LucideIcon } from 'lucide-react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandGroup, CommandItem, CommandList } from '@/components/ui/command'
import { Kbd } from '@/components/ui/kbd'

export type SlashPage = 'actions' | 'skills' | 'models' | 'effort'
export type SlashItem = {
  key: string
  label: string
  detail?: string
  aliases?: string[]
  icon: LucideIcon
} & (
  { page: SlashPage } |
  { action: 'new' | 'attach' | 'context' | 'settings' | 'tasks' } |
  { action: 'skill' | 'model' | 'effort'; value: string }
)

export function ChatSlashMenu({ composer, label, active, items, onHighlight, onChoose, onBack }: {
  composer: RefObject<HTMLTextAreaElement | null>; label: string; active?: SlashItem; items: SlashItem[]
  onHighlight: (key: string) => void; onChoose: (item: SlashItem) => void; onBack?: () => void
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
  return <div className="chat-slash-menu bg-popover" onMouseDown={event => event.preventDefault()}>
    {onBack && <div className="flex items-center gap-1 border-b border-border/70 px-2 py-1 text-xs text-muted-foreground">
      <Button type="button" variant="ghost" size="icon-xs" aria-label="返回斜杠命令" onClick={onBack}>
        <ChevronLeft />
      </Button>
      <span className="font-medium text-foreground">{label}</span>
      <Kbd className="ml-auto">Esc</Kbd>
    </div>}
    <Command label={label} value={active?.key ?? ''} shouldFilter={false} onValueChange={onHighlight}>
      <CommandList ref={list} aria-label={label} showScrollbar className="max-h-[min(40dvh,19rem)]">
        <CommandEmpty>没有匹配命令</CommandEmpty>
        <CommandGroup>
          {items.map(item => <CommandItem key={item.key} data-slash-active={activeKey === item.key ? true : undefined} value={item.key}
            aria-label={item.label} aria-description={item.detail} onSelect={() => onChoose(item)}>
            <AnimatedGlyph icon={item.icon} />
            <span className="shrink-0">{item.label}</span>
            {item.detail && <span className="ml-2 min-w-0 truncate text-xs text-muted-foreground">{item.detail}</span>}
            {'page' in item && <ChevronRight data-slot="command-shortcut" className="ml-auto shrink-0 text-muted-foreground" aria-hidden="true" />}
          </CommandItem>)}
        </CommandGroup>
      </CommandList>
    </Command>
  </div>
}
