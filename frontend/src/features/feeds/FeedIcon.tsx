import { useId, useState } from 'react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Input } from '@/components/ui/input'
import { Field, FieldLabel } from '@/components/ui/field'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

export function FeedIcon({ data }: { data?: string }) {
  const [failed, setFailed] = useState<string>()
  if (!data || data === failed || !/^data:image\/(x-icon|png|jpeg|gif|webp);base64,/.test(data)) {
    return <AnimatedGlyph meaning="radio-tower" />
  }
  return <img src={data} alt="" draggable={false} width={18} height={18}
    className="size-[18px] shrink-0 rounded-sm object-contain" onError={() => setFailed(data)} />
}

export function FeedIconField({
  data, useDefault, onChange, disabled, siteAvailable, customAllowed, customURL, onCustomURLChange, customData,
}: {
  data?: string
  useDefault: boolean
  onChange: (value: boolean) => void
  disabled: boolean
  siteAvailable: boolean
  customAllowed: boolean
  customURL?: string
  onCustomURLChange: (url: string) => void
  customData?: string
}) {
  const label = useId()
  const inputID = useId()
  const mode = customURL !== undefined ? 'custom' : useDefault ? 'default' : 'site'
  return <Field>
    <FieldLabel id={label}>图标</FieldLabel>
    <ToggleGroup className="flex-wrap" type="single" variant="outline" size="sm" aria-labelledby={label}
      disabled={disabled} value={mode} onValueChange={value => {
        if (value === 'custom') onCustomURLChange('')
        else if (value) onChange(value === 'default')
      }}>
      <ToggleGroupItem value="site" disabled={!siteAvailable}>
        {data && <FeedIcon data={data} />}网站图标
      </ToggleGroupItem>
      <ToggleGroupItem value="default" onClick={() => onChange(true)}><FeedIcon />默认图标</ToggleGroupItem>
      <ToggleGroupItem value="custom" disabled={!customAllowed}>
        {customData && <FeedIcon data={customData} />}自定义图标
      </ToggleGroupItem>
    </ToggleGroup>
    {customURL !== undefined && <Field>
      <FieldLabel htmlFor={inputID}>图标网址</FieldLabel>
      <Input id={inputID} type="url" required maxLength={4096} disabled={disabled || !customAllowed}
        value={customURL} onChange={event => onCustomURLChange(event.target.value)} />
    </Field>}
  </Field>
}
