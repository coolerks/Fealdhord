import { AnimatedGlyph } from '@/components/animated-glyph'
import type { IconMeaning } from '@/components/animated-glyph'
import { useState } from 'react'
import { Folder,Info, Palette, Rss, SlidersHorizontal, Sparkles } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Switch } from '@/components/ui/switch'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { restoreSettingsFocus, useWorkspace } from '@/stores/workspace'
import type { SettingsTab } from '@/stores/workspace'
import { preferencesAPI } from '@/lib/preferences'
import type { Preferences } from '@/lib/preferences'
import { RefreshStatus } from '@/features/feeds/RefreshStatus'
import { CategoryManager } from '@/features/categories/CategoryManager'
import { FeedSettings } from '@/features/feeds/FeedSettings'
import DiagnosticPanel from './DiagnosticPanel'

function Choice({ name, label, value, choices, onChange }: { name: string; label: string; value: string; choices: [string, string][]; onChange: (value: string) => void }) {
  return <Field><FieldLabel id={`${name}-label`}>{label}</FieldLabel><ToggleGroup type="single" value={value} onValueChange={next => { if (next) onChange(next) }} variant="outline" aria-labelledby={`${name}-label`}>{choices.map(([key, title]) => <ToggleGroupItem key={key} value={key}>{title}</ToggleGroupItem>)}</ToggleGroup></Field>
}
function Range({ name, label, value, min, max, step = 1, unit = '', onChange }: { name: string; label: string; value: number; min: number; max: number; step?: number; unit?: string; onChange: (value: number) => void }) {
  return <Field><FieldLabel htmlFor={name}>{label}<span className="range-value">{value}{unit}</span></FieldLabel><Input type="range" id={name} min={min} max={max} step={step} value={value} onChange={event => { const next = event.target.valueAsNumber; if (Number.isFinite(next)) onChange(next) }} /></Field>
}

const sections: { id: SettingsTab; title: string; icon: typeof Palette; meaning:IconMeaning }[] = [
  { id: 'appearance', title: '外观与浏览', icon: Palette, meaning:"palette" }, { id: 'reader', title: '阅读排版', icon: SlidersHorizontal, meaning:"sliders-horizontal" },
  {id:'categories',title:'分类',icon:Folder,meaning:'folder'},
  { id: 'feeds', title: '订阅', icon: Rss, meaning:"radio-tower" }, { id: 'ai', title: 'AI 与扩展', icon: Sparkles, meaning:"ai" }, { id: 'about', title: '关于与诊断', icon: Info, meaning:"circle-help" },
]

export function SettingsDialog() {
  const preferences = useWorkspace(state => state.preferences)
  const open = useWorkspace(state => state.settingsOpen)
  const tab = useWorkspace(state => state.settingsTab)
  const patch = useWorkspace(state => state.patch)
  const saveStatus = useWorkspace(state => state.saveStatus)
  const saveError = useWorkspace(state => state.saveError)
  const retrySave = useWorkspace(state => state.retrySave)
  const initialize = useWorkspace(state => state.initialize)
  const [reloadError, setReloadError] = useState('')
  if (!preferences) return null
  const set = <K extends keyof Preferences>(key: K, value: Preferences[K]) => patch({ [key]: value })
  return <Dialog open={open} onOpenChange={value => useWorkspace.setState({ settingsOpen: value })}><DialogContent aria-describedby={undefined} className="settings-dialog" onCloseAutoFocus={event => { event.preventDefault(); restoreSettingsFocus() }}><DialogHeader><DialogTitle>设置</DialogTitle></DialogHeader><RefreshStatus />
    <div className="settings-layout"><nav aria-label="设置分区" className="settings-nav">{sections.map(({ id, title, icon: Icon, meaning }) => <Button key={id} variant={tab === id ? 'secondary' : 'ghost'} aria-pressed={tab === id} onClick={() => useWorkspace.setState({ settingsTab: id })}><AnimatedGlyph icon={Icon} meaning={meaning}/>{title}</Button>)}</nav>
      <section className="settings-body" aria-label={sections.find(section => section.id === tab)?.title}>
        {tab === 'appearance' && <FieldGroup>
          <Choice name="theme" label="主题" value={preferences.theme} choices={[["light", "浅色"], ["dark", "深色"], ["system", "跟随系统"]]} onChange={value => set('theme', value)} />
          <Choice name="view" label="默认文章视图" value={preferences.article_view_mode} choices={[["list", "列表"], ["masonry", "瀑布流"]]} onChange={value => set('article_view_mode', value)} />
          <Choice name="density" label="列表密度" value={preferences.list_density} choices={[["compact", "紧凑"], ["comfortable", "舒适"]]} onChange={value => set('list_density', value)} />
          <Choice name="thumbnail" label="列表缩略图" value={preferences.list_thumbnail} choices={[["none", "不显示"], ["small", "小缩略图"]]} onChange={value => set('list_thumbnail', value)} />
          <Choice name="ratio" label="瀑布流图片" value={preferences.masonry_image_mode} choices={[["original_ratio", "原始比例"], ["cropped", "统一裁剪"]]} onChange={value => set('masonry_image_mode', value)} />
          <Separator />
          <Choice name="animation" label="动画" value={preferences.animation} choices={[["full", "完整"], ["subtle", "轻微"], ["off", "关闭"]]} onChange={value => set('animation', value)} />
          <Field orientation="horizontal"><FieldLabel htmlFor="system-motion">遵从系统减少动态效果</FieldLabel><Switch id="system-motion" checked={preferences.follow_system_reduce_motion} onCheckedChange={value => set('follow_system_reduce_motion', value)} /></Field>
        </FieldGroup>}
        {tab === 'reader' && <FieldGroup><Field orientation="horizontal"><FieldLabel htmlFor="reader-auto-read">正文呈现后标为已读</FieldLabel><Switch id="reader-auto-read" checked={preferences.reader_auto_read} onCheckedChange={value=>set('reader_auto_read',value)}/></Field><Choice name="reader-font" label="阅读字体" value={preferences.reader_font_family} choices={[["sans","系统无衬线"],["serif","衬线"],["monospace","等宽"]]} onChange={value=>set('reader_font_family',value)}/><Field orientation="horizontal"><FieldLabel htmlFor="pin-reader">并排阅读（宽窗口三栏）</FieldLabel><Switch id="pin-reader" checked={preferences.reader_pinned} onCheckedChange={value=>set('reader_pinned',value)}/></Field><Range name="font-size" label="正文字号" value={preferences.reader_font_size} min={14} max={24} unit="px" onChange={value => set('reader_font_size', value)} /><Range name="line-height" label="行高" value={preferences.reader_line_height} min={1.4} max={2} step={.1} onChange={value => set('reader_line_height', value)} /><Range name="reader-width" label="正文宽度" value={preferences.reader_width} min={680} max={760} step={20} unit="px" onChange={value => set('reader_width', value)} /></FieldGroup>}
        {tab === 'feeds' && <FeedSettings />}
        {tab==='categories' && <CategoryManager/>}
        {tab === 'ai' && <Empty><EmptyHeader><EmptyMedia variant="icon"><AnimatedGlyph icon={Sparkles} meaning="ai"/></EmptyMedia><EmptyTitle>AI 与扩展尚未开放</EmptyTitle></EmptyHeader></Empty>}
        {tab === 'about' && <DiagnosticPanel />}
      </section>
    </div>
    <div className="settings-save" aria-live="polite"><span>{saveStatus === 'saving' ? '正在保存…' : saveStatus === 'error' ? '有未保存的更改' : '已保存'}</span>{saveError && <><p role="alert">{saveError}</p><Button size="sm" variant="outline" onClick={retrySave}>重试保存</Button><Button size="sm" variant="ghost" onClick={() => { void preferencesAPI.get().then(snapshot => { initialize(snapshot); setReloadError('') }).catch(() => setReloadError('重新载入失败，请稍后重试。')) }}>重新载入</Button></>}{reloadError && <p role="alert">{reloadError}</p>}</div>
  </DialogContent></Dialog>
}
