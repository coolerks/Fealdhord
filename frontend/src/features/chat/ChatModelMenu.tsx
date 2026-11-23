import { useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { Popover } from 'radix-ui'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { cn } from '@/lib/utils'

export type ModelMenuOption = {
  value: string
  label: string
  accessibleLabel?: string
  keywords?: readonly string[]
  graphic?: ReactNode
}

type EffortOption = {
  value: string
  label: string
}

function SubPickerRow({
  title,
  ariaLabel,
  valueLabel,
  valueGraphic,
  options,
  selectedValue,
  disabled,
  onSelect,
}: {
  title: string
  ariaLabel: string
  valueLabel: string
  valueGraphic?: ReactNode
  options: readonly ModelMenuOption[]
  selectedValue: string
  disabled?: boolean
  onSelect: (value: string) => void
}) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const showSearch = options.length >= 10 || search.length > 0

  return (
    <Popover.Root open={open && !disabled} onOpenChange={next => { setOpen(next); if (!next) setSearch('') }}>
      <Popover.Trigger asChild>
        <button
          type="button"
          aria-label={ariaLabel}
          disabled={disabled}
          className={cn(
            'flex h-9 w-full items-center justify-between gap-4 rounded-lg px-2.5 text-sm outline-none transition-colors',
            'hover:bg-muted focus-visible:bg-muted data-[state=open]:bg-muted',
            'disabled:pointer-events-none disabled:opacity-50',
          )}
        >
          <span className="shrink-0 font-normal text-foreground">{title}</span>
          <span className="flex min-w-0 items-center gap-1.5 text-muted-foreground">
            {valueGraphic && <span className="flex shrink-0 items-center">{valueGraphic}</span>}
            <span className="truncate">{valueLabel}</span>
            <ChevronRight className="size-3.5 shrink-0 opacity-70" aria-hidden="true" />
          </span>
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          role="dialog"
          aria-label={ariaLabel}
          side="left"
          align="start"
          sideOffset={6}
          collisionPadding={8}
          className="z-50 w-max min-w-44 max-w-[min(22rem,var(--radix-popover-content-available-width))] rounded-xl border bg-popover p-1 shadow-md outline-none"
          onOpenAutoFocus={event => {
            event.preventDefault()
            inputRef.current?.focus()
          }}
        >
          <Command
            label={`搜索${ariaLabel}`}
            className={cn(
              'gap-1 p-0',
              showSearch ? '[&_[data-slot=command-input-wrapper]]:p-0' : '[&_[data-slot=command-input-wrapper]]:sr-only',
            )}
          >
            <CommandInput ref={inputRef} value={search} onValueChange={setSearch} placeholder="输入文字搜索…" />
            <CommandList
              showScrollbar
              className={cn(
                'min-w-0',
                showSearch
                  ? 'max-h-[min(18rem,calc(var(--radix-popover-content-available-height)-3rem))]'
                  : 'max-h-[min(18rem,calc(var(--radix-popover-content-available-height)-0.625rem))]',
              )}
            >
              <CommandEmpty>没有匹配项</CommandEmpty>
              <CommandGroup className="p-0">
                {options.map(option => {
                  const checked = option.value === selectedValue
                  return (
                    <CommandItem
                      key={option.value}
                      value={option.value}
                      keywords={[option.label, ...(option.keywords ?? [])]}
                      data-checked={checked}
                      aria-checked={checked}
                      aria-label={option.accessibleLabel}
                      className="h-8 min-w-0 rounded-lg px-2 py-0"
                      onSelect={() => {
                        onSelect(option.value)
                        setOpen(false)
                        setSearch('')
                      }}
                    >
                      {option.graphic}
                      <span className="min-w-0 flex-1 truncate">{option.label}</span>
                    </CommandItem>
                  )
                })}
              </CommandGroup>
            </CommandList>
          </Command>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}

export function ChatModelMenu({
  modelOptions,
  selectedModel,
  effortOptions,
  selectedEffort,
  selectedEffortLabel,
  defaultEffortValue,
  disabled,
  onSelectModel,
  onSelectEffort,
}: {
  modelOptions: readonly ModelMenuOption[]
  selectedModel?: ModelMenuOption
  effortOptions: readonly EffortOption[]
  selectedEffort?: string
  selectedEffortLabel?: string
  defaultEffortValue: string
  disabled?: boolean
  onSelectModel: (id: string) => void
  onSelectEffort: (effort: string) => void
}) {
  const hasEffort = effortOptions.length > 0
  const allEfforts: ModelMenuOption[] = hasEffort
    ? [{ value: defaultEffortValue, label: '使用服务默认' }, ...effortOptions]
    : []

  return (
    <Popover.Root>
      <Popover.Trigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          aria-label="聊天模型"
          className="chat-model chat-toolbar-select max-w-full justify-between gap-1.5 font-normal"
          disabled={disabled}
        >
          {selectedModel?.graphic}
          <span className="truncate">{selectedModel?.label ?? '选择模型'}</span>
          {selectedEffortLabel && (
            <span className="max-w-24 shrink-0 truncate text-muted-foreground">{selectedEffortLabel}</span>
          )}
          <ChevronDown className="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
        </Button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          role="dialog"
          aria-label="模型与推理等级"
          side="top"
          align="end"
          sideOffset={6}
          collisionPadding={8}
          className="z-50 min-w-64 max-w-[min(22rem,var(--radix-popover-content-available-width))] rounded-xl border bg-popover p-1 shadow-md outline-none"
        >
          <SubPickerRow
            title="模型"
            ariaLabel="选择模型"
            valueLabel={selectedModel?.label ?? '选择模型'}
            valueGraphic={selectedModel?.graphic}
            options={modelOptions}
            selectedValue={selectedModel?.value ?? ''}
            disabled={disabled}
            onSelect={onSelectModel}
          />
          <SubPickerRow
            title="推理等级"
            ariaLabel="推理等级"
            valueLabel={hasEffort ? (selectedEffortLabel ?? '使用服务默认') : '未配置'}
            options={allEfforts}
            selectedValue={hasEffort ? (selectedEffort || defaultEffortValue) : ''}
            disabled={disabled || !hasEffort}
            onSelect={onSelectEffort}
          />
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}
