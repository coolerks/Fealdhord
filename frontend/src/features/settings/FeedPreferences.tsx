import { useState } from 'react'
import type { FormEvent } from 'react'
import { Button } from '@/components/ui/button'
import { FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { useWorkspace } from '@/stores/workspace'
import { SettingCard, SettingRow, SettingSection } from './settings-primitives'

const presets = [5, 15, 30, 60, 120, 360, 720, 1440]
const intervalOptions = [
  ...presets.map(minutes => ({
    value: String(minutes),
    label: minutes < 60 ? `${minutes} 分钟` : `${minutes / 60} 小时`,
  })),
  { value: 'custom', label: '自定义…' },
]

export function FeedPreferences() {
  const preferences = useWorkspace(state => state.preferences)!
  const patch = useWorkspace(state => state.patch)
  const minutes = preferences.refresh_interval_minutes
  const [custom, setCustom] = useState(!presets.includes(minutes))
  const [draft, setDraft] = useState(String(minutes))

  function saveInterval(event: FormEvent) {
    event.preventDefault()
    const value = Number(draft)
    if (Number.isInteger(value) && value >= 5 && value <= 1440) {
      patch({ refresh_interval_minutes: value })
    }
  }

  return (
    <SettingSection title="刷新策略">
      <SettingCard>
        <SettingRow>
          <FieldLabel htmlFor="refresh-interval" className="font-normal">自动刷新间隔</FieldLabel>
          <SearchableSelect
            id="refresh-interval"
            label="自动刷新间隔"
            className="min-w-36"
            options={intervalOptions}
            value={custom ? 'custom' : String(minutes)}
            onValueChange={value => {
              setCustom(value === 'custom')
              if (value === 'custom') setDraft(String(minutes))
              else patch({ refresh_interval_minutes: Number(value) })
            }}
          />
        </SettingRow>
        {custom && (
          <SettingRow>
            <form onSubmit={saveInterval} className="flex w-full flex-wrap items-center justify-between gap-2">
              <FieldLabel htmlFor="refresh-custom" className="font-normal">自定义间隔（分钟）</FieldLabel>
              <div className="flex items-center gap-2">
                <Input
                  id="refresh-custom"
                  type="number"
                  className="w-28"
                  min={5}
                  max={1440}
                  step={1}
                  required
                  value={draft}
                  onChange={event => setDraft(event.target.value)}
                />
                <Button size="sm" type="submit" disabled={Number(draft) === minutes}>应用</Button>
              </div>
            </form>
          </SettingRow>
        )}
      </SettingCard>
    </SettingSection>
  )
}
