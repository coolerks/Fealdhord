import { useId, useState } from 'react'
import { AnimatedGlyph } from '@/components/animated-glyph'
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

export function FeedIconField({ data, useDefault, onChange, disabled, siteAvailable }: {
  data?: string
  useDefault: boolean
  onChange: (value: boolean) => void
  disabled: boolean
  siteAvailable: boolean
}) {
  const label = useId()
  return <Field>
    <FieldLabel id={label}>图标</FieldLabel>
    <ToggleGroup type="single" variant="outline" size="sm" aria-labelledby={label} disabled={disabled}
      value={useDefault ? 'default' : 'site'} onValueChange={value => { if (value) onChange(value === 'default') }}>
      <ToggleGroupItem value="site" disabled={!siteAvailable}>
        {data && <FeedIcon data={data} />}网站图标
      </ToggleGroupItem>
      <ToggleGroupItem value="default" onClick={() => onChange(true)}><FeedIcon />默认图标</ToggleGroupItem>
    </ToggleGroup>
  </Field>
}
