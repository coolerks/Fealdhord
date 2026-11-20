import { useRef, useState } from 'react'
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
  meaning?: IconMeaning
  disabled?: boolean
}

function SelectOption({ option, checked, announceSelection, onSelect }: {
  option: Option
  checked: boolean
  announceSelection: boolean
  onSelect: () => void
}) {
  const text = useRef<HTMLSpanElement>(null)
  const [open, setOpen] = useState(false)

  return <Tooltip open={open} disableHoverableContent onOpenChange={next => {
    const element = text.current
    setOpen(next && Boolean(element && element.scrollWidth > element.clientWidth))
  }}>
    <TooltipTrigger asChild>
      <div>
        <CommandItem className="h-8 min-w-0 rounded-lg px-2 py-0" value={option.value}
          keywords={[option.label, ...(option.keywords ?? [])]} disabled={option.disabled}
          data-checked={checked} aria-checked={announceSelection ? checked : undefined}
          aria-label={option.accessibleLabel} onSelect={onSelect}>
          {option.icon && <AnimatedGlyph icon={option.icon} meaning={option.meaning} />}
          <span ref={text} className="min-w-0 flex-1 truncate">{option.label}</span>
        </CommandItem>
      </div>
    </TooltipTrigger>
    <TooltipContent sideOffset={4} collisionPadding={8}
      className="pointer-events-none max-w-[min(24rem,var(--radix-tooltip-content-available-width))] whitespace-normal wrap-anywhere">
      {option.label}
    </TooltipContent>
  </Tooltip>
}

export function SearchableSelect({
  id, label, options, value, onValueChange, disabled, className, variant = 'outline',
  announceSelection = false, placeholder = '请选择',
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
}) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const input = useRef<HTMLInputElement>(null)
  const showSearch = options.length >= 10 || search.length > 0
  const multiple = Array.isArray(value)
  const selected = options.find(option => option.value === value)
  const checked = (id: string) => Array.isArray(value) ? value.includes(id) : value === id
  function changeOpen(next: boolean) {
    setOpen(next)
    if (!next) setSearch('')
  }
  return <Popover.Root open={open && !disabled} onOpenChange={changeOpen} modal>
    <Popover.Trigger asChild>
      <Button type="button" id={id} aria-label={label} variant={variant} disabled={disabled}
        aria-description={announceSelection ? selected?.accessibleLabel ?? selected?.label : undefined}
        className={cn('min-w-0 justify-between font-normal', className)}>
        <span className={cn('truncate', !selected && !multiple && 'text-muted-foreground')}>{selected?.label ?? placeholder}</span>
        <AnimatedGlyph meaning="chevron-right" className="rotate-90 text-muted-foreground" />
      </Button>
    </Popover.Trigger>
    <Popover.Portal>
      <Popover.Content align="start" sideOffset={4} aria-label={label}
        className="z-50 w-max max-w-[min(var(--radix-popover-content-available-width),24rem)] rounded-xl border bg-popover p-1 shadow-md"
        onOpenAutoFocus={event => {
          if (input.current) {
            event.preventDefault()
            input.current.focus()
          }
        }}>
        <Command label={`搜索${label}`} className={cn('gap-1 p-0',
          showSearch ? '[&_[data-slot=command-input-wrapper]]:p-0' : '[&_[data-slot=command-input-wrapper]]:sr-only')}>
          <CommandInput ref={input} value={search} onValueChange={setSearch} placeholder="输入文字搜索…" />
          <CommandList aria-multiselectable={multiple || undefined} className={cn('min-w-0 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden',
            showSearch ? 'max-h-[min(18rem,calc(var(--radix-popover-content-available-height)-3rem))]'
              : 'max-h-[min(18rem,calc(var(--radix-popover-content-available-height)-0.625rem))]')}>
            <CommandEmpty>没有匹配项</CommandEmpty>
            <CommandGroup className="p-0">
              {options.map(option => <SelectOption key={option.value} option={option} checked={checked(option.value)}
                announceSelection={multiple || announceSelection}
                onSelect={() => {
                  onValueChange(option.value)
                  if (!multiple) changeOpen(false)
                }} />)}
            </CommandGroup>
          </CommandList>
        </Command>
      </Popover.Content>
    </Popover.Portal>
  </Popover.Root>
}
