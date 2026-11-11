import { useRef, useState } from 'react'
import { Popover } from 'radix-ui'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { cn } from '@/lib/utils'

type Option = {
  value: string
  label: string
  keywords?: string[]
  disabled?: boolean
}

export function SearchableSelect({
  id, label, options, value, onValueChange, disabled, className, matchTriggerWidth = false, placeholder = '请选择',
}: {
  id?: string
  label: string
  options: Option[]
  value: string | string[]
  onValueChange: (value: string) => void
  disabled?: boolean
  className?: string
  matchTriggerWidth?: boolean
  placeholder?: string
}) {
  const [open, setOpen] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  const multiple = Array.isArray(value)
  const selected = options.find(option => option.value === value)
  const checked = (id: string) => Array.isArray(value) ? value.includes(id) : value === id
  return <Popover.Root open={open && !disabled} onOpenChange={setOpen} modal>
    <Popover.Trigger asChild>
      <Button type="button" id={id} aria-label={label} variant="outline" disabled={disabled}
        className={cn('min-w-0 justify-between font-normal', className)}>
        <span className={cn('truncate', !selected && !multiple && 'text-muted-foreground')}>{selected?.label ?? placeholder}</span>
        <AnimatedGlyph meaning="chevron-right" className="rotate-90 text-muted-foreground" />
      </Button>
    </Popover.Trigger>
    <Popover.Portal>
      <Popover.Content align="start" sideOffset={4} aria-label={label}
        className={cn('z-50 rounded-xl border bg-popover shadow-md', matchTriggerWidth
          ? 'w-[min(var(--radix-popover-trigger-width),var(--radix-popover-content-available-width))]'
          : 'w-[min(var(--radix-popover-content-available-width),24rem)]')}
        onOpenAutoFocus={event => {
          if (input.current) {
            event.preventDefault()
            input.current.focus()
          }
        }}>
        <Command label={`搜索${label}`}>
          <CommandInput ref={input} placeholder="输入文字搜索…" />
          <CommandList aria-multiselectable={multiple || undefined} showScrollbar className="max-h-[min(18rem,calc(var(--radix-popover-content-available-height)-3rem))] overflow-x-auto">
            <CommandEmpty>没有匹配项</CommandEmpty>
            <CommandGroup>
              {options.map(option => <CommandItem className="w-max min-w-full" key={option.value} value={option.value}
                keywords={[option.label, ...(option.keywords ?? [])]} disabled={option.disabled}
                data-checked={checked(option.value)} aria-checked={multiple ? checked(option.value) : undefined}
                onSelect={() => {
                  onValueChange(option.value)
                  if (!multiple) setOpen(false)
                }}>
                <span className="shrink-0 whitespace-nowrap">{option.label}</span>
              </CommandItem>)}
            </CommandGroup>
          </CommandList>
        </Command>
      </Popover.Content>
    </Popover.Portal>
  </Popover.Root>
}
