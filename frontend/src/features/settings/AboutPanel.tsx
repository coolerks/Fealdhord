import { useQuery } from '@tanstack/react-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Field, FieldLabel } from '@/components/ui/field'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { desktopAPI } from '@/lib/api'
import { native } from '@/lib/native'
import { notify } from '@/lib/notifications'
import appIcon from '../../../../build/appicon.png'

const sourceURL = 'https://github.com/coolerks/Fealdhord'

export function AboutPanel({ developerMode, onDeveloperModeChange }: {
  developerMode: boolean
  onDeveloperModeChange: (enabled: boolean) => void
}) {
  const info = useQuery({ queryKey: ['desktop-info'], queryFn: desktopAPI.info, staleTime: Infinity, retry: false })

  return <div className="flex flex-col gap-6">
    <div className="flex flex-col items-center gap-1.5 pt-4 text-center">
      <img src={appIcon} alt="" className="size-24" />
      <h2 className="text-xl font-semibold">{info.data?.name ?? '折栈 Fealdhord'}</h2>
      {info.data && <p className="text-sm text-muted-foreground">{info.data.version}</p>}
      {info.isError && <Button variant="ghost" size="sm" onClick={() => { void info.refetch() }}>重试读取版本</Button>}
      <div className="mt-3 flex flex-wrap justify-center gap-2">
        <Button variant="outline" size="sm" disabled><AnimatedGlyph meaning="refresh" />检查更新</Button>
        <Button variant="outline" size="sm" disabled><AnimatedGlyph meaning="external-link" />访问官网</Button>
        <Button variant="outline" size="sm" onClick={() => {
          void native.openURL(sourceURL).catch(() => { void notify('无法打开源码仓库', undefined, true) })
        }}><AnimatedGlyph meaning="external-link" />查看源码</Button>
      </div>
    </div>
    <Separator />
    <Field orientation="horizontal">
      <FieldLabel htmlFor="developer-mode">开发者模式</FieldLabel>
      <Switch id="developer-mode" checked={developerMode} onCheckedChange={onDeveloperModeChange} />
    </Field>
  </div>
}
