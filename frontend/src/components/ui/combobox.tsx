import { useRef } from 'react'
import { Combobox } from '@base-ui/react/combobox'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'

type Option = { value: string; label: string }

// Chip presentation of the radix-nova Combobox, using its Base UI primitive.
export function SearchableSelectChips({ id, options, value, onValueChange, open, onOpenChange, disabled, placeholder }: {
  id: string
  options: Option[]
  value: string[]
  onValueChange: (value: string[]) => void
  open: boolean
  onOpenChange: (open: boolean) => void
  disabled?: boolean
  placeholder: string
}) {
  const anchor = useRef<HTMLDivElement>(null)
  const container = useRef<HTMLDivElement>(null)
  const selected = options.filter(option => value.includes(option.value))

  return <div ref={container}>
    <Combobox.Root items={options} multiple autoHighlight disabled={disabled} value={selected}
      open={open && !disabled} onOpenChange={onOpenChange}
      isItemEqualToValue={(item, selection) => item.value === selection.value}
      onValueChange={items => onValueChange(items.map(item => item.value))}>
      <Combobox.Chips ref={anchor} data-slot="combobox-chips"
        className="flex max-h-32 min-h-8 flex-wrap items-center gap-1 overflow-y-auto rounded-lg border border-input bg-transparent bg-clip-padding px-2.5 py-1 text-sm transition-colors focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50 has-data-[slot=combobox-chip]:px-1 dark:bg-input/30">
        <Combobox.Value>
          {selected.map(item => (
            <Combobox.Chip key={item.value} data-slot="combobox-chip"
              className="flex h-6 max-w-full items-center gap-0.5 rounded-sm bg-muted pl-1.5 text-xs font-medium text-foreground has-disabled:opacity-50">
              <span className="truncate" title={item.label}>{item.label}</span>
              <Combobox.ChipRemove render={<Button type="button" variant="ghost" size="icon-xs" />}
                aria-label={`移除${item.label}`} className="text-muted-foreground [&_.animated-glyph]:size-3! [&_svg]:size-3!">
                <AnimatedGlyph meaning="x" />
              </Combobox.ChipRemove>
            </Combobox.Chip>
          ))}
        </Combobox.Value>
        <Combobox.Input id={id} data-slot="combobox-chip-input" placeholder={selected.length ? undefined : placeholder}
          className="min-w-16 flex-1 outline-none placeholder:text-muted-foreground disabled:opacity-50" />
        <Combobox.Trigger render={<Button type="button" variant="ghost" size="icon-xs" />}
          aria-label="展开选项" className="[&_.animated-glyph]:size-4! [&_svg]:size-4!">
          <AnimatedGlyph meaning="chevron-right" className="rotate-90 text-muted-foreground" />
        </Combobox.Trigger>
      </Combobox.Chips>
      <Combobox.Portal container={container}>
        <Combobox.Positioner anchor={anchor} sideOffset={4} className="isolate z-50">
          <Combobox.Popup data-slot="combobox-content"
            className="group/combobox-content w-(--anchor-width) max-w-(--available-width) overflow-hidden rounded-lg bg-popover text-popover-foreground shadow-md ring-1 ring-foreground/10">
            <Combobox.Empty className="hidden py-2 text-center text-sm text-muted-foreground group-data-empty/combobox-content:block">
              没有匹配项
            </Combobox.Empty>
            <Combobox.List className="max-h-[min(16rem,var(--available-height))] overflow-y-auto overscroll-contain p-1 data-empty:p-0">
              {(item: Option) => (
                <Combobox.Item key={item.value} value={item}
                  className="relative flex cursor-default items-center rounded-md py-1 pr-8 pl-2 text-sm outline-hidden select-none data-highlighted:bg-accent data-highlighted:text-accent-foreground">
                  <span className="truncate" title={item.label}>{item.label}</span>
                  <Combobox.ItemIndicator className="pointer-events-none absolute right-2 [&_.animated-glyph]:size-4! [&_svg]:size-4!">
                    <AnimatedGlyph meaning="check" />
                  </Combobox.ItemIndicator>
                </Combobox.Item>
              )}
            </Combobox.List>
          </Combobox.Popup>
        </Combobox.Positioner>
      </Combobox.Portal>
    </Combobox.Root>
  </div>
}
