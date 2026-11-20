import { useRef, useState } from 'react'
import { Popover } from 'radix-ui'
import type { LucideIcon } from 'lucide-react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import type { IconMeaning } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
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
              {options.map(option => <CommandItem className="h-8 min-w-0 rounded-lg px-2 py-0" key={option.value} value={option.value}
                keywords={[option.label, ...(option.keywords ?? [])]} disabled={option.disabled}
                data-checked={checked(option.value)} aria-checked={multiple || announceSelection ? checked(option.value) : undefined}
                aria-label={option.accessibleLabel}
                onSelect={() => {
                  onValueChange(option.value)
                  if (!multiple) changeOpen(false)
                }}>
                {option.icon && <AnimatedGlyph icon={option.icon} meaning={option.meaning} />}
                <span className="min-w-0 flex-1 truncate">{option.label}</span>
              </CommandItem>)}
            </CommandGroup>
          </CommandList>
        </Command>
      </Popover.Content>
    </Popover.Portal>
  </Popover.Root>
}
