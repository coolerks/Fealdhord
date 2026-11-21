import { useEffect } from 'react'
import { Events } from '@wailsio/runtime'
import { notify } from '@/lib/notifications'
import { FealdhordScheduledNotice } from '../../bindings/events'
import { AcknowledgeLocalNotice, ListLocalNotices } from '../../bindings/github.com/coolerks/Fealdhord/internal/notification/api'

export function useScheduledNotices() {
  useEffect(() => {
    let cancelled = false
    let loading = false
    let queued = false
    async function drain() {
      if (loading) {
        queued = true
        return
      }
      loading = true
      try {
        do {
          queued = false
          const pending = await ListLocalNotices()
          for (const item of pending ?? []) {
            if (cancelled) return
            await notify(item.title, item.body)
            await AcknowledgeLocalNotice(item.task_id, item.occurrence)
          }
          if (pending && pending.length === 30) queued = true
        } while (queued && !cancelled)
      } catch {
        // 未确认的通知可在重新打开应用时恢复。
      } finally {
        loading = false
      }
    }
    const off = Events.On(FealdhordScheduledNotice, () => { void drain() })
    void drain()
    return () => { cancelled = true; off() }
  }, [])
}
