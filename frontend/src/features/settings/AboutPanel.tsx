import { Activity, Copy, ExternalLink } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { FieldLabel } from '@/components/ui/field'
import { Switch } from '@/components/ui/switch'
import { toast } from '@/components/ui/toast-manager'
import { desktopAPI } from '@/lib/api'
import { native } from '@/lib/native'
import { notify } from '@/lib/notifications'
import { useWorkspace } from '@/stores/workspace'
import { SettingCard, SettingRow, SettingSection } from './settings-primitives'
import appIcon from '../../../../build/appicon.png'

const sourceURL = 'https://github.com/coolerks/Fealdhord'

export function AboutPanel({ developerMode, onDeveloperModeChange }: {
  developerMode: boolean
  onDeveloperModeChange: (enabled: boolean) => void
}) {
  const info = useQuery({ queryKey: ['desktop-info'], queryFn: desktopAPI.info, staleTime: Infinity, retry: false })

  function copyVersion() {
    const label = [info.data?.name ?? '折栈 Fealdhord', info.data?.version].filter(Boolean).join(' ')
    void native.copy(label).then(() => {
      toast.add({ title: '已复制版本信息', description: label })
    }).catch(() => {
      toast.add({ title: '无法复制版本信息', type: 'error' })
    })
  }

  return <div className="space-y-6">
    <SettingCard divided={false} className="flex flex-col items-center gap-1.5 px-4 py-7 text-center">
      <img src={appIcon} alt="" className="size-20" />
      <h2 className="mt-1 text-lg font-semibold tracking-[-0.02em]">{info.data?.name ?? '折栈 Fealdhord'}</h2>
      {info.data && <p className="text-xs text-muted-foreground tabular-nums">{info.data.version}</p>}
      {info.isError && <Button variant="ghost" size="sm" onClick={() => { void info.refetch() }}>重试读取版本</Button>}
      <div className="mt-3 flex flex-wrap justify-center gap-2">
        <Button asChild variant="outline" size="sm">
          <a href={sourceURL} onClick={event => {
            event.preventDefault()
            void native.openURL(sourceURL).catch(() => { void notify('无法打开源码仓库', undefined, true) })
          }}>
            <ExternalLink />查看源码
          </a>
        </Button>
        {info.data && (
          <Button type="button" variant="outline" size="sm" onClick={copyVersion}>
            <Copy />复制版本
          </Button>
        )}
      </div>
    </SettingCard>

    <SettingSection title="支持与调试">
      <SettingCard>
        <SettingRow>
          <span className="text-[13px]">连接诊断</span>
          <Button type="button" variant="outline" size="sm" onClick={() => useWorkspace.setState({ diagnosticsOpen: true })}>
            <Activity />运行诊断
          </Button>
        </SettingRow>
        <SettingRow>
          <FieldLabel htmlFor="developer-mode" className="font-normal">开发者模式</FieldLabel>
          <Switch id="developer-mode" checked={developerMode} onCheckedChange={onDeveloperModeChange} />
        </SettingRow>
      </SettingCard>
    </SettingSection>
  </div>
}
