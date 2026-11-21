import { FeedPreferences } from './FeedPreferences'
import { Authorize } from '../../../bindings/github.com/coolerks/Fealdhord/internal/desktop/notifier'
import { toast } from '@/components/ui/toast-manager'
import { AnimatedGlyph } from '@/components/animated-glyph'
import type { IconMeaning } from '@/components/animated-glyph'
import { useCallback, useEffectEvent, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { SemanticButton } from '@/components/semantic-button'
import { SidebarAction } from '@/components/sidebar-action'
import { moveSidebarFocus } from '@/lib/sidebar-navigation'
import type { effectiveMotion } from '@/lib/environment'
import { animateWorkspaceEntry } from '@/lib/workspace-motion'
import { BadgeAlert,Folder,Info, Maximize, Palette, Rss, SlidersHorizontal, Sparkles } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { SegmentedControl } from '@/components/ui/segmented-control'
import { ToggleGroupItem } from '@/components/ui/toggle-group'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { readerToolbarOptions } from '@/features/reader/toolbar-actions'
import { availableCodeTheme, codeThemeOptions } from '@/features/reader/code-themes'
import { Switch } from '@/components/ui/switch'
import { Slider } from '@/components/ui/slider'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { AISettings } from './AISettings'
import { useWorkspace } from '@/stores/workspace'
import type { SettingsTab } from '@/stores/workspace'
import { preferencesAPI } from '@/lib/preferences'
import type { Preferences } from '@/lib/preferences'
import { RefreshStatus } from '@/features/feeds/RefreshStatus'
import { CategoryManager } from '@/features/categories/CategoryManager'
import { FeedSettings } from '@/features/feeds/FeedSettings'
import { TransferPanel } from '@/features/transfer/TransferPanel'
import { AboutPanel } from './AboutPanel'

function Choice({ name, label, value, choices, onChange, disabled }: { name: string; label: string; value: string; choices: [string, string, IconMeaning][]; onChange: (value: string) => void; disabled?: boolean }) {
  return <Field><FieldLabel id={`${name}-label`}>{label}</FieldLabel><SegmentedControl disabled={disabled} value={value} onValueChange={next => { if (next) onChange(next) }} aria-labelledby={`${name}-label`}>{choices.map(([key, title, icon]) => <ToggleGroupItem key={key} value={key}><AnimatedGlyph meaning={icon}/>{title}</ToggleGroupItem>)}</SegmentedControl></Field>
}
function Range({ name, label, value, min, max, step = 1, unit = '', onChange }: { name: string; label: string; value: number; min: number; max: number; step?: number; unit?: string; onChange: (value: number) => void }) {
  return <Field>
    <FieldLabel><span id={name}>{label}</span><span className="range-value">{value}{unit}</span></FieldLabel>
    <Slider min={min} max={max} step={step} value={[value]}
      thumbProps={{ 'aria-labelledby': name, 'aria-valuetext': unit ? `${value} ${unit}` : undefined }}
      onValueChange={([next]) => { if (Number.isFinite(next)) onChange(next) }} />
  </Field>
}

const sections: { id: SettingsTab; title: string; icon: typeof Palette; meaning:IconMeaning }[] = [
  { id: 'appearance', title: '外观与浏览', icon: Palette, meaning:"palette" }, { id: 'reader', title: '阅读排版', icon: SlidersHorizontal, meaning:"sliders-horizontal" },
  {id:'data',title:'导入与导出',icon:Folder,meaning:'folder'},
  {id:'categories',title:'分类',icon:Folder,meaning:'folder'},
  { id: 'feeds', title: '订阅', icon: Rss, meaning:"radio-tower" },
  { id: 'notifications', title: '通知', icon: BadgeAlert, meaning: 'badge-alert' },
  { id: 'ai', title: 'AI 与扩展', icon: Sparkles, meaning:"ai" }, { id: 'about', title: '关于', icon: Info, meaning:"circle-help" },
]

export function SettingsDialog({ sidebar, motionMode }: {
  sidebar: HTMLElement | null
  motionMode: ReturnType<typeof effectiveMotion>
}) {
  const preferences = useWorkspace(state => state.preferences)
  const fullscreen = useWorkspace(state => state.settingsPresentation === 'full')
  const close = useWorkspace(state => state.closeSettings)
  const tab = useWorkspace(state => state.settingsTab)
  const patch = useWorkspace(state => state.patch)
  const saveStatus = useWorkspace(state => state.saveStatus)
  const saveError = useWorkspace(state => state.saveError)
  const retrySave = useWorkspace(state => state.retrySave)
  const initialize = useWorkspace(state => state.initialize)
  const [reloadError, setReloadError] = useState('')
  const workspaceSidebar = useRef<HTMLElement>(null)
  const workspaceMain = useRef<HTMLElement>(null)
  const [contentHost] = useState(() => document.createElement('div'))
  const attachContent = useCallback((section: HTMLElement | null) => {
    section?.append(contentHost)
  }, [contentHost])
  const animateEntry = useEffectEvent(() =>
    animateWorkspaceEntry([workspaceSidebar.current, workspaceMain.current], motionMode, 6),
  )
  useLayoutEffect(() => {
    if (!fullscreen || !sidebar) return
    sidebar.querySelector<HTMLElement>('#settings-back')?.focus({ preventScroll: true })
    return animateEntry()
  }, [fullscreen, sidebar])
  if (!preferences) return null
  const set = <K extends keyof Preferences>(key: K, value: Preferences[K]) => patch({ [key]: value })
  const title = sections.find(section => section.id === tab)?.title
  const navigation = <nav aria-label="设置分区" className={fullscreen ? 'system-nav settings-workspace-nav' : 'settings-nav'}
    onKeyDown={fullscreen ? moveSidebarFocus : undefined}>
    {sections.map(({ id, title, icon: Icon, meaning }) => <Button key={id}
      variant={tab === id ? 'secondary' : 'ghost'} className={fullscreen ? 'nav-item' : undefined}
      data-navigation={fullscreen ? id : undefined} aria-current={fullscreen && tab === id ? 'page' : undefined}
      aria-pressed={fullscreen ? undefined : tab === id}
      onClick={() => useWorkspace.setState({ settingsTab: id })}><AnimatedGlyph icon={Icon} meaning={meaning} />{title}</Button>)}
  </nav>
  const body = <section ref={attachContent} className="settings-body" aria-label={title} />
  const content = createPortal(<>
        {tab === 'appearance' && <FieldGroup>
          <Choice name="theme" label="主题" value={preferences.theme} choices={[["light", "浅色","sun"], ["dark", "深色","moon"], ["system", "跟随系统","monitor-cog"]]} onChange={value => set('theme', value)} />
          <Choice name="settings-open-mode" label="设置打开方式" value={preferences.settings_open_mode ?? 'full'}
            choices={[[ 'full', '全屏', 'maximize' ], [ 'dialog', '对话框', 'layers' ]]}
            onChange={value => set('settings_open_mode', value)} />
          <Choice name="window-close" label="关闭窗口时" value={preferences.quit_on_window_close ? 'quit' : 'background'}
            choices={[[ 'background', '在后台运行', 'minimize' ], [ 'quit', '退出应用', 'x' ]]}
            onChange={value => set('quit_on_window_close', value === 'quit')} />
          <Choice name="view" label="默认文章视图" value={preferences.article_view_mode} choices={[["list", "列表","list"], ["masonry", "瀑布流","grid"]]} onChange={value => set('article_view_mode', value)} />
          <Choice name="density" label="列表密度" value={preferences.list_density} choices={[["compact", "紧凑","minimize"], ["comfortable", "舒适","maximize"]]} onChange={value => set('list_density', value)} />
          <Choice name="ratio" label="瀑布流图片" value={preferences.masonry_image_mode} choices={[["original_ratio", "原始比例","maximize"], ["cropped", "统一裁剪","scan-text"]]} onChange={value => set('masonry_image_mode', value)} />
          <Separator />
          <Choice name="animation" label="动画" value={preferences.animation} choices={[["full", "完整","sparkles"], ["subtle", "轻微","circle-dashed"], ["off", "关闭","x"]]} onChange={value => set('animation', value)} />
          <Field orientation="horizontal"><FieldLabel htmlFor="system-motion">遵从系统减少动态效果</FieldLabel><Switch id="system-motion" checked={preferences.follow_system_reduce_motion} onCheckedChange={value => set('follow_system_reduce_motion', value)} /></Field>
        </FieldGroup>}
        {tab === 'reader' && <FieldGroup>
          <Choice name="reader-open-mode" label="点击文章时" value={preferences.reader_open_mode} choices={[["feed","订阅预览","book-text"],["full","阅读模式","scan-text"]]} onChange={value=>set('reader_open_mode',value)}/>
          <Field>
            <FieldLabel htmlFor="reader-code-theme">代码块主题</FieldLabel>
            <SearchableSelect id="reader-code-theme" label="代码块主题" options={codeThemeOptions}
              value={availableCodeTheme(preferences.reader_code_theme)} onValueChange={value => set('reader_code_theme', value)} />
          </Field>
          <Choice name="reader-image-wheel" label="看图时滚动鼠标" value={preferences.reader_image_wheel}
            choices={[['zoom', '缩放', 'scan-text'], ['navigate', '切换图片', 'layers']]} onChange={value => set('reader_image_wheel', value)} />
          <Field orientation="horizontal">
            <FieldLabel htmlFor="reader-image-backdrop-close">点击空白处退出看图</FieldLabel>
            <Switch id="reader-image-backdrop-close" checked={preferences.reader_image_backdrop_close}
              onCheckedChange={value => set('reader_image_backdrop_close', value)} />
          </Field>
          <Choice name="reader-outline" label="文章目录" value={preferences.reader_outline}
            choices={[['ticks', '刻度', 'list'], ['auto', '自适应', 'maximize'], ['fixed', '固定面板', 'panel-right-open'], ['hidden', '隐藏', 'x']]}
            onChange={value => set('reader_outline', value)} />
          <Field>
            <FieldLabel htmlFor="reader-toolbar-pinned">常驻工具栏按钮</FieldLabel>
            <SearchableSelect id="reader-toolbar-pinned" label="常驻工具栏按钮" options={[...readerToolbarOptions]}
              value={preferences.reader_toolbar_pinned}
              placeholder={preferences.reader_toolbar_pinned.length ? `已常驻 ${preferences.reader_toolbar_pinned.length} 项` : '全部放入更多'}
              onValueChange={id => {
                const pinned = preferences.reader_toolbar_pinned
                set('reader_toolbar_pinned', pinned.includes(id) ? pinned.filter(item => item !== id) : [...pinned, id])
              }} />
          </Field>
          <Field orientation="horizontal"><FieldLabel htmlFor="reader-auto-read">正文呈现后标为已读</FieldLabel><Switch id="reader-auto-read" checked={preferences.reader_auto_read} onCheckedChange={value=>set('reader_auto_read',value)}/></Field><Choice name="reader-font" label="阅读字体" value={preferences.reader_font_family} choices={[["sans","系统无衬线","book-text"],["serif","衬线","book"],["monospace","等宽","layers"]]} onChange={value=>set('reader_font_family',value)}/><Choice name="reader-layout" label="瀑布流阅读布局" value={preferences.reader_layout === 'fill' ? 'fill' : 'drawer'} choices={[["fill","铺满","maximize"],["drawer","抽屉","panel-right-open"]]} onChange={value=>set('reader_layout',value)}/><Range name="font-size" label="正文字号" value={preferences.reader_font_size} min={14} max={24} unit="px" onChange={value => set('reader_font_size', value)} /><Range name="line-height" label="行高" value={preferences.reader_line_height} min={1.4} max={2} step={.1} onChange={value => set('reader_line_height', value)} /><Range name="reader-width" label="正文宽度" value={preferences.reader_width} min={680} max={760} step={20} unit="px" onChange={value => set('reader_width', value)} /></FieldGroup>}
        {tab === 'feeds' && <FieldGroup><FeedPreferences /><Separator /><FeedSettings /></FieldGroup>}
        {tab === 'notifications' && <NotificationPreferences mode={preferences.notification_mode}
          autoDismiss={preferences.refresh_toast_auto_dismiss} patch={patch} />}
        {tab==='data' && <TransferPanel/>}
        {tab==='categories' && <CategoryManager/>}
        {tab === 'ai' && <FieldGroup>
          <Field>
            <FieldLabel htmlFor="chat-code-theme">聊天代码块主题</FieldLabel>
            <SearchableSelect id="chat-code-theme" label="聊天代码块主题" options={codeThemeOptions}
              value={availableCodeTheme(preferences.chat_code_theme)} onValueChange={value => set('chat_code_theme', value)} />
          </Field>
          <Separator />
          <AISettings />
        </FieldGroup>}
        {tab === 'about' && <AboutPanel developerMode={preferences.developer_mode}
          onDeveloperModeChange={enabled => set('developer_mode', enabled)} />}
  </>, contentHost)
  const status = (saveStatus !== 'saved' || saveError || reloadError) && <div className="settings-save" aria-live="polite">
      {saveStatus === 'saving' && <span>正在保存…</span>}
      {saveStatus === 'error' && <span>有未保存的更改</span>}
      {saveError && <>
        <p role="alert">{saveError}</p>
        <Button size="sm" variant="outline" onClick={retrySave}>重试保存</Button>
        <Button size="sm" variant="ghost" onClick={() => {
          void preferencesAPI.get().then(snapshot => {
            initialize(snapshot)
            setReloadError('')
          }).catch(() => setReloadError('重新载入失败，请稍后重试。'))
        }}>重新载入</Button>
      </>}
      {reloadError && <p role="alert">{reloadError}</p>}
    </div>
  return <>
    {content}
    {fullscreen ? <>
    {sidebar && createPortal(<aside ref={workspaceSidebar} id="workspace-sidebar" className="glass-sidebar settings-workspace-sidebar" aria-label="设置侧栏">
      <div className="sidebar-titlebar" />
      <div className="system-nav">
        <SidebarAction id="settings-back" label="返回" meaning="chevron-left" shortcut="Escape" onClick={close} />
      </div>
      {navigation}
    </aside>, sidebar)}
    <main ref={workspaceMain} className="reading-workspace settings-workspace" aria-label="设置">
      <header className="workspace-toolbar settings-workspace-heading"><h1 className="truncate font-semibold">{title}</h1></header>
      <RefreshStatus />
      {body}
      {status}
    </main>
    </> : <Dialog open onOpenChange={value => { if (!value) close() }}>
    <DialogContent aria-describedby={undefined} className="settings-dialog" onCloseAutoFocus={event => event.preventDefault()}>
      <DialogHeader><DialogTitle>设置</DialogTitle></DialogHeader>
      <div className="settings-expand">
        <SemanticButton icon={Maximize} meaning="maximize" label="全屏设置"
          onClick={() => useWorkspace.setState({ settingsPresentation: 'full' })} />
      </div>
      <RefreshStatus />
      <div className="settings-layout">{navigation}{body}</div>
      {status}
    </DialogContent>
  </Dialog>}
  </>
}

function NotificationPreferences({ mode, autoDismiss, patch }: {
  mode: string
  autoDismiss: boolean
  patch: (delta: Partial<Preferences>) => void
}) {
  const [authorizing, setAuthorizing] = useState(false)

  async function selectMode(next: string) {
    if (next === 'toast') {
      patch({ notification_mode: next })
    } else if (next === 'native') {
      setAuthorizing(true)
      try {
        if (await Authorize()) patch({ notification_mode: next })
        else toast.add({ title: '系统通知权限未开启', type: 'error' })
      } catch {
        toast.add({ title: '系统通知不可用', type: 'error' })
      } finally {
        setAuthorizing(false)
      }
    }
  }

  return <FieldGroup>
    <Choice name="notification-mode" label="通知方式" value={mode} disabled={authorizing}
      choices={[[ 'toast', '应用内通知', 'panel-right-open' ], [ 'native', '系统通知', 'monitor-cog' ]]}
      onChange={next => { void selectMode(next) }} />
    <Field orientation="horizontal">
      <FieldLabel htmlFor="refresh-toast-auto-dismiss">自动收起应用内订阅更新通知</FieldLabel>
      <Switch id="refresh-toast-auto-dismiss" checked={autoDismiss}
        onCheckedChange={checked => patch({ refresh_toast_auto_dismiss: checked })} />
    </Field>
  </FieldGroup>
}
