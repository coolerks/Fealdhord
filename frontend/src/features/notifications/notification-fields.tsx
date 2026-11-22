import type { ReactNode } from 'react'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { Switch } from '@/components/ui/switch'

export const defaultZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
export const zones = [...new Set([defaultZone, 'UTC', ...(Intl.supportedValuesOf?.('timeZone') ?? [])])]
  .map(value => ({ value, label: value }))
export const time = (value: number) => value > 0 ? new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'short' }).format(value) : ''

export function toggle<T>(items: T[] | null | undefined, item: T) {
  return items?.includes(item) ? items.filter(value => value !== item) : [...(items ?? []), item]
}

export function TextField({ id, label, value, onChange, type = 'text', disabled }: {
  id: string; label: string; value: string; onChange(value: string): void; type?: string; disabled?: boolean
}) {
  return <Field>
    <FieldLabel htmlFor={id}>{label}</FieldLabel>
    <Input id={id} value={value} type={type} disabled={disabled} onChange={event => onChange(event.target.value)} />
  </Field>
}

export function FlagField({ id, label, value, onChange }: { id: string; label: string; value: boolean; onChange(value: boolean): void }) {
  return <Field orientation="horizontal"><FieldLabel htmlFor={id}>{label}</FieldLabel><Switch id={id} checked={value} onCheckedChange={onChange} /></Field>
}

export function Selection({ id, label, options, value, onChange, placeholder, disabled, labelAction }: {
  id: string; label: string; options: { value: string; label: string }[]; value: string | string[]
  onChange(value: string): void; placeholder?: string; disabled?: boolean; labelAction?: ReactNode
}) {
  return <Field>
    {labelAction ? (
      <div className="flex items-center justify-between gap-2">
        <FieldLabel htmlFor={id}>{label}</FieldLabel>
        {labelAction}
      </div>
    ) : <FieldLabel htmlFor={id}>{label}</FieldLabel>}
    <SearchableSelect id={id} label={label} options={options} value={value} onValueChange={onChange}
      className="w-full" placeholder={placeholder} disabled={disabled} />
  </Field>
}
