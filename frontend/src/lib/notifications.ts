import { Send } from '../../bindings/github.com/coolerks/Fealdhord/internal/desktop/notifier'
import { useWorkspace } from '@/stores/workspace'
import { toast } from '@/components/ui/toast-manager'

let sequence = 0

export async function notify(title: string, description?: string, error = false, kind?: 'subscription-refresh') {
  const preferences = useWorkspace.getState().preferences
  if (preferences?.notification_mode === 'native') {
    try {
      await Send(`fealdhord-${Date.now()}-${++sequence}`, title, description ?? '')
      return
    } catch {
      description = [description, '系统通知无法送达'].filter(Boolean).join(' · ')
    }
  }
  toast.add({
    title, description, type: error ? 'error' : 'info',
    id: kind,
    timeout: kind === 'subscription-refresh' && !preferences?.refresh_toast_auto_dismiss ? 0 : undefined,
  })
}
