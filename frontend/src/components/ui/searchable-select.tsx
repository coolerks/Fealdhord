import { useRef, useState } from 'react'
import type { ReactNode, RefObject } from 'react'
import { Popover } from 'radix-ui'
import type { LucideIcon } from 'lucide-react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import type { IconMeaning } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

type Option = {
  value: string
  label: string
  accessibleLabel?: string
  keywords?: readonly string[]
  icon?: LucideIcon
  graphic?: ReactNode
  meaning?: IconMeaning
  disabled?: boolean
}

type Submenu = { label: string; options: Option[]; value: string; summary?: string }

function SelectOption({ option, checked, announceSelection, onSelect, submenu, expanded, anchor }: {
  option: Option
  checked: boolean
  announceSelection: boolean
  onSelect: () => void
  submenu?: Submenu
  expanded?: boolean
  anchor?: (element: HTMLDivElement | null) => void
}) {
  const text = useRef<HTMLSpanElement>(null)
  const [open, setOpen] = useState(false)

  return <Tooltip open={open} disableHoverableContent onOpenChange={next => {
    const element = text.current
    setOpen(next && Boolean(element && element.scrollWidth > element.clientWidth))
  }}>
    <TooltipTrigger asChild>
      <div ref={anchor}>
        <CommandItem className="h-8 min-w-0 rounded-lg px-2 py-0 data-[expanded=true]:bg-muted" value={option.value}
          keywords={[option.label, ...(option.keywords ?? [])]} disabled={option.disabled}
          data-checked={checked} aria-checked={announceSelection ? checked : undefined}
          data-expanded={expanded} aria-haspopup={submenu ? 'dialog' : undefined}
          aria-label={option.accessibleLabel} aria-description={submenu?.summary} onSelect={onSelect}>
          {option.graphic ?? (option.icon && <AnimatedGlyph icon={option.icon} meaning={option.meaning} />)}
          <span className="flex min-w-0 flex-1 items-center gap-2">
            <span ref={text} className="truncate">{option.label}</span>
            {submenu?.summary && <span className="max-w-24 shrink-0 truncate text-muted-foreground">{submenu.summary}</span>}
          </span>
          {submenu && !checked && <span data-slot="command-shortcut"><AnimatedGlyph meaning="chevron-right" /></span>}
        </CommandItem>
      </div>
    </TooltipTrigger>
    <TooltipContent sideOffset={4} collisionPadding={8}
      className="pointer-events-none max-w-[min(24rem,var(--radix-tooltip-content-available-width))] whitespace-normal wrap-anywhere">
      {option.label}
    </TooltipContent>
  </Tooltip>
}

const panel = 'z-50 w-max rounded-xl border bg-popover p-1 shadow-md'

function SelectList({ label, options, multiple, input, onExpand, onBack, children }: {
  label: string
  options: Option[]
  multiple?: boolean
  input: RefObject<HTMLInputElement | null>
  onExpand?: (value: string) => void
  onBack?: () => void
  children: ReactNode
}) {
  const [search, setSearch] = useState('')
  const showSearch = options.length >= 10 || search.length > 0
  return <Command label={`搜索${label}`} className={cn('gap-1 p-0',
    showSearch ? '[&_[data-slot=command-input-wrapper]]:p-0' : '[&_[data-slot=command-input-wrapper]]:sr-only')}
    onKeyDown={event => {
      if (onBack && event.key === 'ArrowLeft' && !search && !event.nativeEvent.isComposing) onBack()
      if (!onExpand || event.key !== 'ArrowRight' || search || event.nativeEvent.isComposing) return
      const active = event.currentTarget.querySelector('[cmdk-item][data-selected=true]')?.getAttribute('data-value')
      if (active) onExpand(active)
    }}>
    <CommandInput ref={input} value={search} onValueChange={setSearch} placeholder="输入文字搜索…" />
    <CommandList aria-multiselectable={multiple || undefined} className={cn('min-w-0',
      showSearch ? 'max-h-[min(18rem,calc(var(--radix-popover-content-available-height)-3rem))]'
        : 'max-h-[min(18rem,calc(var(--radix-popover-content-available-height)-0.625rem))]')}>
      <CommandEmpty>没有匹配项</CommandEmpty>
      <CommandGroup className="p-0">{children}</CommandGroup>
    </CommandList>
  </Command>
}

export function SearchableSelect({
  id, label, options, value, onValueChange, disabled, className, variant = 'outline',
  announceSelection = false, placeholder = '请选择', submenus, onSubValueChange,
}: {
  id?: string
  label: string
  options: Option[]
  value: string | string[]
  onValueChange: (value: string) => void
  disabled?: boolean
  className?: string
  variant?: 'outline' | 'ghost'
  announceSelection?: boolean
  placeholder?: string
  submenus?: Record<string, Submenu>
  onSubValueChange?: (value: string, subValue: string) => void
}) {
  const [open, setOpen] = useState(false)
  const [expanded, setExpanded] = useState<{ value: string; anchor: HTMLDivElement } | null>(null)
  const [content, setContent] = useState<HTMLDivElement | null>(null)
  const input = useRef<HTMLInputElement>(null)
  const subInput = useRef<HTMLInputElement>(null)
  const anchors = useRef(new Map<string, HTMLDivElement>())
  const multiple = Array.isArray(value)
  const selected = options.find(option => option.value === value)
  const summary = selected && submenus?.[selected.value]?.summary
  const submenu = expanded && submenus?.[expanded.value]
  const checked = (id: string) => Array.isArray(value) ? value.includes(id) : value === id
  function expand(id: string) {
    const anchor = anchors.current.get(id)
    if (anchor) setExpanded({ value: id, anchor })
  }
  function changeOpen(next: boolean) {
    setOpen(next)
    if (!next) setExpanded(null)
  }
  return <Popover.Root open={open && !disabled} onOpenChange={changeOpen} modal>
    <Popover.Trigger asChild>
      <Button type="button" id={id} aria-label={label} variant={variant} disabled={disabled}
        aria-description={announceSelection && selected ? [selected.accessibleLabel ?? selected.label, summary].filter(Boolean).join(' · ') : undefined}
        className={cn('min-w-0 justify-between font-normal', className)}>
        {selected?.graphic}
        <span className={cn('truncate', !selected && !multiple && 'text-muted-foreground')}>{selected?.label ?? placeholder}</span>
        {summary && <span className="max-w-24 shrink-0 truncate text-muted-foreground">{summary}</span>}
        <AnimatedGlyph meaning="chevron-right" className="rotate-90 text-muted-foreground" />
      </Button>
    </Popover.Trigger>
    <Popover.Portal>
      <Popover.Content ref={submenus ? setContent : undefined} align={submenus ? 'end' : 'start'} sideOffset={4} aria-label={label}
        className={cn(panel, 'max-w-[min(var(--radix-popover-content-available-width),24rem)]')}
        onOpenAutoFocus={event => {
          if (input.current) {
            event.preventDefault()
            input.current.focus()
          }
        }}>
        <SelectList label={label} options={options} multiple={multiple} input={input}
          onExpand={submenus && expand}>
          {options.map(option => <SelectOption key={option.value} option={option} checked={checked(option.value)}
            announceSelection={multiple || announceSelection} submenu={submenus?.[option.value]}
            expanded={expanded?.value === option.value || undefined}
            anchor={submenus?.[option.value] ? element => {
              if (element) anchors.current.set(option.value, element)
              else anchors.current.delete(option.value)
            } : undefined}
            onSelect={() => {
              if (submenus?.[option.value]) {
                expand(option.value)
                return
              }
              onValueChange(option.value)
              if (!multiple) changeOpen(false)
            }} />)}
        </SelectList>
        {expanded && submenu && content && <Popover.Root key={expanded.value} open onOpenChange={next => { if (!next) setExpanded(null) }}>
          <Popover.Anchor virtualRef={{ current: expanded.anchor }} />
          <Popover.Portal container={content}>
            <Popover.Content side="right" align="start" sideOffset={5} collisionPadding={8} aria-label={submenu.label}
              className={cn(panel, 'min-w-[min(9rem,var(--radix-popover-content-available-width))] max-w-[min(18rem,var(--radix-popover-content-available-width))]')}
              onOpenAutoFocus={event => {
                event.preventDefault()
                subInput.current?.focus({ preventScroll: true })
              }}
              onCloseAutoFocus={event => {
                event.preventDefault()
                if (!subInput.current) input.current?.focus({ preventScroll: true })
              }}>
              <SelectList label={submenu.label} options={submenu.options} input={subInput}
                onBack={() => setExpanded(null)}>
                {submenu.options.map(option => <SelectOption key={option.value} option={option}
                  checked={submenu.value === option.value} announceSelection onSelect={() => {
                    onSubValueChange?.(expanded.value, option.value)
                    changeOpen(false)
                  }} />)}
              </SelectList>
            </Popover.Content>
          </Popover.Portal>
        </Popover.Root>}
      </Popover.Content>
    </Popover.Portal>
  </Popover.Root>
}
