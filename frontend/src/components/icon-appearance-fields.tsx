import { useId } from 'react'
import type { IconMeaning } from '@/components/animated-glyph'
import { ColorPicker } from '@/components/ui/color-picker'
import { Field, FieldLabel } from '@/components/ui/field'
import { IconPicker } from '@/components/ui/icon-picker'

export function IconAppearanceFields({ icon, color, fallback, disabled, onIconChange, onColorChange }: {
  icon: string
  color: string
  fallback: IconMeaning
  disabled: boolean
  onIconChange: (value: string) => void
  onColorChange: (value: string) => void
}) {
  const iconID = useId()
  const colorID = useId()
  return <>
    <Field>
      <FieldLabel htmlFor={iconID}>图标</FieldLabel>
      <IconPicker id={iconID} label="图标" className="w-full" value={icon || fallback} fallback={fallback}
        disabled={disabled} onChange={onIconChange} />
    </Field>
    <Field>
      <FieldLabel htmlFor={colorID}>图标颜色</FieldLabel>
      <ColorPicker id={colorID} label="图标颜色" value={color} disabled={disabled} onChange={onColorChange} />
    </Field>
  </>
}
