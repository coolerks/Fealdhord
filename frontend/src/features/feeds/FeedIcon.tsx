import { useId, useState } from 'react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { CustomIcon, hexColorRegex, parseFeedBuiltinIcon } from '@/components/custom-icon'
import { ColorPicker } from '@/components/ui/color-picker'
import { IconPicker } from '@/components/ui/icon-picker'
import { Input } from '@/components/ui/input'
import { Field, FieldLabel } from '@/components/ui/field'
import { SegmentedControl } from '@/components/ui/segmented-control'
import { ToggleGroupItem } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'

export function FeedIcon({ data }: { data?: string }) {
  const [failed, setFailed] = useState<string>()
  const builtin = parseFeedBuiltinIcon(data)
  if (builtin) {
    return (
      <span className={cn('inline-flex shrink-0', builtin.color && 'category-glyph')} style={{ color: builtin.color }}>
        <CustomIcon value={builtin.icon} fallback="radio-tower" />
      </span>
    )
  }
  if (!data || data === failed || !/^data:image\/(x-icon|png|jpeg|gif|webp);base64,/.test(data)) {
    return <AnimatedGlyph meaning="radio-tower" />
  }
  return <img src={data} alt="" draggable={false} width={16} height={16}
    className="feed-icon size-4 shrink-0 rounded-sm object-contain" onError={() => setFailed(data)} />
}

export function FeedIconField({
  data, useDefault, onChange, disabled, siteAvailable, customAllowed, customURL, onCustomURLChange, customData, icon, onIconChange, iconColor, onIconColorChange,
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
  icon?: string
  onIconChange?: (icon: string) => void
  iconColor?: string
  onIconColorChange?: (color: string) => void
}) {
  const label = useId()
  const inputID = useId()
  const pickerID = useId()
  const colorID = useId()
  const mode = customURL !== undefined ? 'custom' : useDefault ? 'default' : 'site'
  const validColor = iconColor && hexColorRegex.test(iconColor) ? iconColor : undefined
  return <Field>
    <FieldLabel id={label}>图标</FieldLabel>
    <SegmentedControl size="sm" aria-labelledby={label}
      disabled={disabled} value={mode} onValueChange={value => {
        if (value === 'custom') onCustomURLChange('')
        else if (value) onChange(value === 'default')
      }}>
      <ToggleGroupItem value="site" disabled={!siteAvailable}>
        {data ? <FeedIcon data={data} /> : <AnimatedGlyph meaning="external-link" />}网站图标
      </ToggleGroupItem>
      <ToggleGroupItem value="default" onClick={() => onChange(true)}>
        <span className={cn('inline-flex shrink-0', validColor && 'category-glyph')} style={{ color: validColor }}>
          <CustomIcon value={icon || 'radio-tower'} fallback="radio-tower" />
        </span>
        内置图标
      </ToggleGroupItem>
      <ToggleGroupItem value="custom" disabled={!customAllowed}>
        {customData ? <FeedIcon data={customData} /> : <AnimatedGlyph meaning="settings" />}自定义图标
      </ToggleGroupItem>
    </SegmentedControl>
    {mode === 'default' && onIconChange && (
      <div className="space-y-2.5 pt-1">
        <IconPicker
          id={pickerID}
          label="订阅图标"
          className="w-full"
          disabled={disabled}
          value={icon || 'radio-tower'}
          fallback="radio-tower"
          onChange={onIconChange}
        />
        {onIconColorChange && (
          <Field>
            <FieldLabel htmlFor={colorID}>图标颜色</FieldLabel>
            <ColorPicker
              id={colorID}
              label="图标颜色"
              value={iconColor ?? ''}
              disabled={disabled}
              onChange={onIconColorChange}
            />
          </Field>
        )}
      </div>
    )}
    {customURL !== undefined && <Field>
      <FieldLabel htmlFor={inputID}>图标网址</FieldLabel>
      <Input id={inputID} type="url" required maxLength={4096} disabled={disabled || !customAllowed}
        value={customURL} onChange={event => onCustomURLChange(event.target.value)} />
    </Field>}
  </Field>
}
