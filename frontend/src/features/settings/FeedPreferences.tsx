import { useState } from 'react'
import type { FormEvent } from 'react'
import { Authorize } from '../../../bindings/github.com/coolerks/Fealdhord/internal/desktop/notifier'
import { Button } from '@/components/ui/button'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { useWorkspace } from '@/stores/workspace'
import { toast } from '@/components/ui/toast-manager'

const presets = [5, 15, 30, 60, 120, 360, 720, 1440]
const intervalOptions = [
  ...presets.map(minutes => ({
    value: String(minutes),
    label: minutes < 60 ? `${minutes} 分钟` : `${minutes / 60} 小时`,
  })),
  { value: 'custom', label: '自定义…' },
]
const notificationOptions = [
  { value: 'toast', label: '应用内 Toast' },
  { value: 'native', label: '系统通知' },
]

export function FeedPreferences() {
  const preferences = useWorkspace(state => state.preferences)!
  const patch = useWorkspace(state => state.patch)
  const minutes = preferences.refresh_interval_minutes
  const [custom, setCustom] = useState(!presets.includes(minutes))
  const [draft, setDraft] = useState(String(minutes))
  const [authorizing, setAuthorizing] = useState(false)

  function saveInterval(event: FormEvent) {
    event.preventDefault()
    const value = Number(draft)
    if (Number.isInteger(value) && value >= 5 && value <= 1440) {
      patch({ refresh_interval_minutes: value })
    }
  }

  async function selectNotification(mode: string) {
    if (mode === 'toast') {
      patch({ notification_mode: mode })
    } else if (mode === 'native') {
      setAuthorizing(true)
      try {
        if (await Authorize()) patch({ notification_mode: mode })
        else toast.add({ title: '系统通知权限未开启', type: 'error' })
      } catch {
        toast.add({ title: '系统通知不可用', type: 'error' })
      } finally {
        setAuthorizing(false)
      }
    }
  }

  return (
    <>
      <Field orientation="horizontal">
        <FieldLabel htmlFor="refresh-interval">自动刷新间隔</FieldLabel>
        <SearchableSelect id="refresh-interval" label="自动刷新间隔" options={intervalOptions}
          value={custom ? 'custom' : String(minutes)}
          onValueChange={value => {
            setCustom(value === 'custom')
            if (value === 'custom') setDraft(String(minutes))
            else patch({ refresh_interval_minutes: Number(value) })
          }} />
      </Field>
      {custom && <form onSubmit={saveInterval} className="flex items-end gap-2">
        <Field>
          <FieldLabel htmlFor="refresh-custom">自定义间隔（分钟）</FieldLabel>
          <Input id="refresh-custom" type="number" min={5} max={1440} step={1} required value={draft}
            onChange={event => setDraft(event.target.value)} />
        </Field>
        <Button type="submit" disabled={Number(draft) === minutes}>应用</Button>
      </form>}
      <Field orientation="horizontal">
        <FieldLabel htmlFor="notification-mode">通知方式</FieldLabel>
        <SearchableSelect id="notification-mode" label="通知方式" options={notificationOptions}
          value={preferences.notification_mode} disabled={authorizing}
          onValueChange={mode => { void selectNotification(mode) }} />
      </Field>
    </>
  )
}
