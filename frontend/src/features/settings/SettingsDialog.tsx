import { FeedPreferences } from './FeedPreferences'
import { MediaPreferences } from './MediaPreferences'
import { SettingCard, SettingRow, SettingSection } from './settings-primitives'
import { Authorize } from '../../../bindings/github.com/coolerks/Fealdhord/internal/desktop/notifier'
import { toast } from '@/components/ui/toast-manager'
import { AnimatedGlyph } from '@/components/animated-glyph'
import type { IconMeaning } from '@/components/animated-glyph'
import { lazy, Suspense, useCallback, useEffectEvent, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { SemanticButton } from '@/components/semantic-button'
import { Splitter } from '@/components/splitter'
import type { CSSProperties } from 'react'
import { SidebarAction } from '@/components/sidebar-action'
import { moveSidebarFocus } from '@/lib/sidebar-navigation'
import type { effectiveMotion } from '@/lib/environment'
import { animateWorkspaceEntry } from '@/lib/workspace-motion'
import {
  Archive, Bell, Blocks, Bot, Database, FolderTree, Info, Maximize, Palette, PanelLeftClose, PanelLeftOpen, Plug, Rss, SlidersHorizontal, Sparkles,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { maxArticleShareCount, maxArticleShareSummaryLimit } from '@/features/notifications/article-share-preferences'
import { SegmentedControl } from '@/components/ui/segmented-control'
import { ToggleGroupItem } from '@/components/ui/toggle-group'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { readerToolbarOptions } from '@/features/reader/toolbar-actions'
import { availableCodeTheme, codeThemeOptions } from '@/features/reader/code-themes'
import { Switch } from '@/components/ui/switch'
import { Slider } from '@/components/ui/slider'
import { Button } from '@/components/ui/button'
import { AISettings } from './AISettings'
import { ArchivedConversations } from './ArchivedConversations'
import { approvalModes, approvalPresentations } from '@/features/chat/approval-options'
import { MCPSettings, SkillsSettings } from './AIPlatformSettings'
import { ArticleAISettings } from '@/features/article-ai/ArticleAISettings'
import { useWorkspace } from '@/stores/workspace'
import type { SettingsTab } from '@/stores/workspace'
import { preferencesAPI } from '@/lib/preferences'
import type { Preferences } from '@/lib/preferences'
import { RefreshStatus } from '@/features/feeds/RefreshStatus'
import { CategoryManager } from '@/features/categories/CategoryManager'
import { FeedSettings } from '@/features/feeds/FeedSettings'
import { TransferPanel } from '@/features/transfer/TransferPanel'
import { AboutPanel } from './AboutPanel'

const NotificationSettings = lazy(() => import('./NotificationSettings').then(module => ({ default: module.NotificationSettings })))

function ChoiceRow({ name, label, value, choices, onChange, disabled }: {
  name: string; label: string; value: string; choices: [string, string, IconMeaning][]; onChange: (value: string) => void; disabled?: boolean
}) {
  return (
    <SettingRow>
      <FieldLabel id={`${name}-label`} className="font-normal">{label}</FieldLabel>
      <SegmentedControl disabled={disabled} value={value} onValueChange={next => { if (next) onChange(next) }} aria-labelledby={`${name}-label`}>
        {choices.map(([key, title, icon]) => <ToggleGroupItem key={key} value={key}><AnimatedGlyph meaning={icon} />{title}</ToggleGroupItem>)}
      </SegmentedControl>
    </SettingRow>
  )
}

function RangeRow({ name, label, value, min, max, step = 1, unit = '', onChange }: {
  name: string; label: string; value: number; min: number; max: number; step?: number; unit?: string; onChange: (value: number) => void
}) {
  return (
    <div className="space-y-2.5 px-3.5 py-2.5">
      <div className="flex items-center justify-between gap-2">
        <FieldLabel id={name} className="font-normal">{label}</FieldLabel>
        <span className="range-value text-xs">{value}{unit}</span>
      </div>
      <Slider min={min} max={max} step={step} value={[value]}
        thumbProps={{ 'aria-labelledby': name, 'aria-valuetext': unit ? `${value} ${unit}` : undefined }}
        onValueChange={([next]) => { if (Number.isFinite(next)) onChange(next) }} />
    </div>
  )
}

type SectionEntry = { id: SettingsTab; title: string; icon: LucideIcon; meaning: IconMeaning }
type SectionGroup = { label: string; items: SectionEntry[] }

const sectionGroups: SectionGroup[] = [
  {
    label: '通用',
    items: [
      { id: 'appearance', title: '外观与浏览', icon: Palette, meaning: 'palette' },
      { id: 'reader', title: '阅读排版', icon: SlidersHorizontal, meaning: 'sliders-horizontal' },
      { id: 'notifications', title: '通知', icon: Bell, meaning: 'bell' },
    ],
  },
  {
    label: '内容与数据',
    items: [
      { id: 'feeds', title: '订阅', icon: Rss, meaning: 'radio-tower' },
      { id: 'categories', title: '分类', icon: FolderTree, meaning: 'folder-tree' },
      { id: 'data', title: '媒体与数据', icon: Database, meaning: 'database-backup' },
    ],
  },
  {
    label: 'AI 与扩展',
    items: [
      { id: 'ai', title: '服务与模型', icon: Sparkles, meaning: 'ai' },
      { id: 'archived-chat', title: '已归档会话', icon: Archive, meaning: 'box' },
      { id: 'article-ai', title: '文章 AI', icon: Bot, meaning: 'bot' },
      { id: 'mcp', title: 'MCP 与权限', icon: Plug, meaning: 'webhook' },
      { id: 'skills', title: 'Skills', icon: Blocks, meaning: 'blocks' },
    ],
  },
  {
    label: '系统',
    items: [
      { id: 'about', title: '关于', icon: Info, meaning: 'circle-help' },
    ],
  },
]

const allSections = sectionGroups.flatMap(group => group.items)
const chatRetryOptions = Array.from({ length: 11 }, (_, count) => ({
  value: String(count),
  label: count === 0 ? '不重试' : `${count} 次`,
}))

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
  const [sidebarDraft, setSidebarDraft] = useState<number | null>(null)
  const workspaceSidebar = useRef<HTMLElement>(null)
  const workspaceMain = useRef<HTMLElement>(null)
  const [contentHost] = useState(() => document.createElement('div'))
  const attachContent = useCallback((section: HTMLElement | null) => {
    section?.append(contentHost)
  }, [contentHost])
  const animateEntry = useEffectEvent(() =>
    animateWorkspaceEntry([workspaceSidebar.current?.hasAttribute('inert') ? null : workspaceSidebar.current, workspaceMain.current], motionMode, 6),
  )
  useLayoutEffect(() => {
    if (!fullscreen || !sidebar) return
    const back = sidebar.querySelector<HTMLElement>('#settings-back')
    const target = back?.closest('[inert]') ? document.getElementById('settings-sidebar-toggle') : back
    target?.focus({ preventScroll: true })
    return animateEntry()
  }, [fullscreen, sidebar])
  if (!preferences) return null
  const set = <K extends keyof Preferences>(key: K, value: Preferences[K]) => patch({ [key]: value })
  const title = allSections.find(section => section.id === tab)?.title
  const collapsed = Boolean(preferences.settings_sidebar_collapsed)
  const width = sidebarDraft ?? preferences.settings_sidebar_width ?? 220
  const toggle = <SemanticButton id="settings-sidebar-toggle" icon={collapsed ? PanelLeftOpen : PanelLeftClose}
    label={collapsed ? '展开设置侧栏' : '收起设置侧栏'} meaning={collapsed ? 'expand' : 'collapse'}
    aria-expanded={!collapsed} aria-controls={fullscreen ? 'workspace-sidebar' : 'settings-dialog-sidebar'}
    onClick={() => set('settings_sidebar_collapsed', !collapsed)} />
  const navigation = (
    <nav aria-label="设置分区" className={fullscreen ? 'settings-workspace-nav' : 'settings-nav'} onKeyDown={moveSidebarFocus}>
      {sectionGroups.map(group => (
        <div key={group.label} className="settings-nav-group">
          <div className="settings-nav-heading">{group.label}</div>
          <div className="settings-nav-items">
            {group.items.map(({ id, title: itemTitle, icon: Icon, meaning }) => (
              <Button
                key={id}
                variant={tab === id ? 'secondary' : 'ghost'}
                className={fullscreen ? 'nav-item' : 'w-full justify-start gap-2.5 text-[13px] font-normal'}
                data-navigation={id}
                aria-current={fullscreen && tab === id ? 'page' : undefined}
                aria-pressed={fullscreen ? undefined : tab === id}
                onClick={() => useWorkspace.setState({ settingsTab: id })}
              >
                <AnimatedGlyph icon={Icon} meaning={meaning} />
                <span className="truncate">{itemTitle}</span>
              </Button>
            ))}
          </div>
        </div>
      ))}
    </nav>
  )
  const body = <section ref={attachContent} className="settings-body" aria-label={title} />
  const content = createPortal(
    <div className="space-y-6">
      {tab === 'appearance' && <>
        <SettingSection title="外观与动效">
          <SettingCard>
            <ChoiceRow name="theme" label="主题" value={preferences.theme}
              choices={[['light', '浅色', 'sun'], ['dark', '深色', 'moon'], ['system', '跟随系统', 'monitor-cog']]}
              onChange={value => set('theme', value)} />
            <ChoiceRow name="animation" label="动画" value={preferences.animation}
              choices={[['full', '完整', 'sparkles'], ['subtle', '轻微', 'circle-dashed'], ['off', '关闭', 'x']]}
              onChange={value => set('animation', value)} />
            <SettingRow>
              <FieldLabel htmlFor="system-motion" className="font-normal">遵从系统减少动态效果</FieldLabel>
              <Switch id="system-motion" checked={preferences.follow_system_reduce_motion}
                onCheckedChange={value => set('follow_system_reduce_motion', value)} />
            </SettingRow>
          </SettingCard>
        </SettingSection>
        <SettingSection title="窗口与浏览">
          <SettingCard>
            <ChoiceRow name="settings-open-mode" label="设置打开方式" value={preferences.settings_open_mode ?? 'full'}
              choices={[['full', '全屏', 'maximize'], ['dialog', '对话框', 'layers']]}
              onChange={value => set('settings_open_mode', value)} />
            <ChoiceRow name="window-close" label="关闭窗口时" value={preferences.quit_on_window_close ? 'quit' : 'background'}
              choices={[['background', '在后台运行', 'minimize'], ['quit', '退出应用', 'x']]}
              onChange={value => set('quit_on_window_close', value === 'quit')} />
            <ChoiceRow name="view" label="默认文章视图" value={preferences.article_view_mode}
              choices={[['list', '列表', 'list'], ['masonry', '瀑布流', 'grid']]}
              onChange={value => set('article_view_mode', value)} />
            <ChoiceRow name="density" label="列表密度" value={preferences.list_density}
              choices={[['compact', '紧凑', 'minimize'], ['comfortable', '舒适', 'maximize']]}
              onChange={value => set('list_density', value)} />
            <ChoiceRow name="ratio" label="瀑布流图片" value={preferences.masonry_image_mode}
              choices={[['original_ratio', '原始比例', 'maximize'], ['cropped', '统一裁剪', 'scan-text']]}
              onChange={value => set('masonry_image_mode', value)} />
          </SettingCard>
        </SettingSection>
      </>}
      {tab === 'reader' && <>
        <SettingSection title="阅读方式与排版">
          <SettingCard>
            <ChoiceRow name="reader-open-mode" label="点击文章时" value={preferences.reader_open_mode}
              choices={[['feed', '订阅预览', 'book-text'], ['full', '阅读模式', 'scan-text']]}
              onChange={value => set('reader_open_mode', value)} />
            <ChoiceRow name="reader-layout" label="瀑布流阅读布局" value={preferences.reader_layout === 'fill' ? 'fill' : 'drawer'}
              choices={[['fill', '铺满', 'maximize'], ['drawer', '抽屉', 'panel-right-open']]}
              onChange={value => set('reader_layout', value)} />
            <ChoiceRow name="reader-font" label="阅读字体" value={preferences.reader_font_family}
              choices={[['sans', '系统无衬线', 'book-text'], ['serif', '衬线', 'book'], ['monospace', '等宽', 'layers']]}
              onChange={value => set('reader_font_family', value)} />
            <RangeRow name="font-size" label="正文字号" value={preferences.reader_font_size} min={14} max={24} unit="px"
              onChange={value => set('reader_font_size', value)} />
            <RangeRow name="line-height" label="行高" value={preferences.reader_line_height} min={1.4} max={2} step={.1}
              onChange={value => set('reader_line_height', value)} />
            <RangeRow name="reader-width" label="正文宽度" value={preferences.reader_width} min={680} max={760} step={20} unit="px"
              onChange={value => set('reader_width', value)} />
            <SettingRow>
              <FieldLabel htmlFor="reader-code-theme" className="font-normal">代码块主题</FieldLabel>
              <SearchableSelect id="reader-code-theme" label="代码块主题" className="min-w-44"
                options={codeThemeOptions}
                value={availableCodeTheme(preferences.reader_code_theme)}
                onValueChange={value => set('reader_code_theme', value)} />
            </SettingRow>
            <SettingRow>
              <FieldLabel htmlFor="reader-auto-read" className="font-normal">正文呈现后标为已读</FieldLabel>
              <Switch id="reader-auto-read" checked={preferences.reader_auto_read}
                onCheckedChange={value => set('reader_auto_read', value)} />
            </SettingRow>
          </SettingCard>
        </SettingSection>
        <SettingSection title="目录与工具栏">
          <SettingCard>
            <ChoiceRow name="reader-outline" label="文章目录" value={preferences.reader_outline}
              choices={[['ticks', '刻度', 'list'], ['auto', '自适应', 'maximize'], ['fixed', '固定面板', 'panel-right-open'], ['hidden', '隐藏', 'x']]}
              onChange={value => set('reader_outline', value)} />
            <SettingRow>
              <FieldLabel htmlFor="reader-toolbar-pinned" className="font-normal">常驻工具栏按钮</FieldLabel>
              <SearchableSelect id="reader-toolbar-pinned" label="常驻工具栏按钮" className="min-w-44"
                options={[...readerToolbarOptions]}
                value={preferences.reader_toolbar_pinned}
                placeholder={preferences.reader_toolbar_pinned.length ? `已常驻 ${preferences.reader_toolbar_pinned.length} 项` : '全部放入更多'}
                onValueChange={id => {
                  const pinned = preferences.reader_toolbar_pinned
                  set('reader_toolbar_pinned', pinned.includes(id) ? pinned.filter(item => item !== id) : [...pinned, id])
                }} />
            </SettingRow>
          </SettingCard>
        </SettingSection>
        <SettingSection title="图片查看">
          <SettingCard>
            <ChoiceRow name="reader-image-wheel" label="看图时滚动鼠标" value={preferences.reader_image_wheel}
              choices={[['zoom', '缩放', 'scan-text'], ['navigate', '切换图片', 'layers']]}
              onChange={value => set('reader_image_wheel', value)} />
            <SettingRow>
              <FieldLabel htmlFor="reader-image-backdrop-close" className="font-normal">点击空白处退出看图</FieldLabel>
              <Switch id="reader-image-backdrop-close" checked={preferences.reader_image_backdrop_close}
                onCheckedChange={value => set('reader_image_backdrop_close', value)} />
            </SettingRow>
          </SettingCard>
        </SettingSection>
      </>}
      {tab === 'feeds' && <>
        <FeedPreferences />
        <FeedSettings />
      </>}
      {tab === 'notifications' && <>
        <NotificationPreferences
          mode={preferences.notification_mode}
          refreshEnabled={preferences.refresh_notifications_enabled}
          autoDismiss={preferences.refresh_toast_auto_dismiss}
          patch={patch}
        />
        <SettingSection title="文章发送">
          <SettingCard>
            <ArticleSharePreference id="article-share-default-count" label="默认发送条数"
              value={preferences.article_share_count} max={maxArticleShareCount} onChange={value => set('article_share_count', value)} />
            <ArticleSharePreference id="article-share-summary-limit" label="默认摘要字数"
              value={preferences.article_share_summary_limit} max={maxArticleShareSummaryLimit} onChange={value => set('article_share_summary_limit', value)} />
          </SettingCard>
        </SettingSection>
        <Suspense fallback={null}><NotificationSettings /></Suspense>
      </>}
      {tab === 'data' && <>
        <MediaPreferences />
        <TransferPanel />
      </>}
      {tab === 'categories' && <CategoryManager />}
      {tab === 'ai' && <>
        <AISettings />
        <SettingSection title="聊天显示">
          <SettingCard>
            <SettingRow>
              <FieldLabel htmlFor="chat-code-theme" className="font-normal">聊天代码块主题</FieldLabel>
              <SearchableSelect id="chat-code-theme" label="聊天代码块主题" className="min-w-44"
                options={codeThemeOptions}
                value={availableCodeTheme(preferences.chat_code_theme)}
                onValueChange={value => set('chat_code_theme', value)} />
            </SettingRow>
            <SettingRow>
              <FieldLabel htmlFor="chat-max-retries" className="font-normal">失败自动重试次数</FieldLabel>
              <SearchableSelect id="chat-max-retries" label="失败自动重试次数" className="min-w-44"
                options={chatRetryOptions}
                value={String(preferences.chat_max_retries ?? 5)}
                onValueChange={value => set('chat_max_retries', Number(value))} />
            </SettingRow>
          </SettingCard>
        </SettingSection>
        <SettingSection title="工具审批">
          <SettingCard>
            <SettingRow>
              <FieldLabel htmlFor="chat-approval-presentation" className="font-normal">审批提示方式</FieldLabel>
              <SearchableSelect id="chat-approval-presentation" label="审批提示方式" className="min-w-44"
                options={approvalPresentations} value={preferences.chat_approval_presentation}
                onValueChange={value => set('chat_approval_presentation', value)} />
            </SettingRow>
            <SettingRow>
              <FieldLabel htmlFor="chat-approval-mode" className="font-normal">默认审批方式</FieldLabel>
              <SearchableSelect id="chat-approval-mode" label="默认审批方式" className="min-w-44"
                options={approvalModes} value={preferences.chat_approval_mode}
                onValueChange={value => set('chat_approval_mode', value)} />
            </SettingRow>
          </SettingCard>
        </SettingSection>
      </>}
      {tab === 'article-ai' && <ArticleAISettings />}
      {tab === 'mcp' && <MCPSettings />}
      {tab === 'archived-chat' && <ArchivedConversations />}
      {tab === 'skills' && <SkillsSettings />}
      {tab === 'about' && <AboutPanel developerMode={preferences.developer_mode}
        onDeveloperModeChange={enabled => set('developer_mode', enabled)} />}
    </div>,
    contentHost,
  )
  const status = (saveStatus !== 'saved' || saveError || reloadError) && (
    <div className="settings-save" aria-live="polite">
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
  )
  return <>
    {content}
    {fullscreen ? <>
      {sidebar && createPortal(
        <aside ref={workspaceSidebar} id="workspace-sidebar" className="glass-sidebar settings-workspace-sidebar" aria-label="设置侧栏"
          inert={collapsed} aria-hidden={collapsed || undefined}>
          <div className="sidebar-titlebar" />
          <div className="system-nav">
            <SidebarAction id="settings-back" label="返回" meaning="chevron-left" shortcut="Escape" onClick={close} />
          </div>
          {navigation}
        </aside>,
        sidebar,
      )}
      <main ref={workspaceMain} className="reading-workspace settings-workspace" aria-label="设置">
        <header className="workspace-toolbar settings-workspace-heading" data-settings-collapsed={collapsed}>
          {toggle}
          {collapsed && <SidebarAction label="返回" meaning="chevron-left" shortcut="Escape" onClick={close} />}
          <h1 className="truncate text-sm font-semibold tracking-[-0.01em]">{title}</h1>
        </header>
        <RefreshStatus />
        {body}
        {status}
      </main>
    </> : (
      <Dialog open onOpenChange={value => { if (!value) close() }}>
        <DialogContent aria-describedby={undefined} className="settings-dialog" onCloseAutoFocus={event => event.preventDefault()}>
          <DialogHeader><div className="flex items-center gap-2">{toggle}<DialogTitle>设置</DialogTitle></div></DialogHeader>
          <div className="settings-expand">
            <SemanticButton icon={Maximize} meaning="maximize" label="全屏设置"
              onClick={() => useWorkspace.setState({ settingsPresentation: 'full' })} />
          </div>
          <RefreshStatus />
          <div className="settings-layout" data-sidebar-collapsed={collapsed}
            style={{ '--settings-sidebar-width': `${width}px` } as CSSProperties}>
            {!collapsed && <>
              <div id="settings-dialog-sidebar" className="settings-dialog-sidebar">{navigation}</div>
              <Splitter label="调整设置侧栏宽度" value={width} min={160} max={300} onResize={setSidebarDraft} onCommit={next => {
                set('settings_sidebar_width', next)
                setSidebarDraft(null)
              }} onCollapse={() => {
                set('settings_sidebar_collapsed', true)
                document.getElementById('settings-sidebar-toggle')?.focus()
              }} />
            </>}
            {body}
          </div>
          {status}
        </DialogContent>
      </Dialog>
    )}
  </>
}

function ArticleSharePreference({ id, label, value, max, onChange }: {
  id: string; label: string; value: number; max: number; onChange: (value: number) => void
}) {
  const [draft, setDraft] = useState<string | null>(null)
  return <SettingRow>
    <FieldLabel htmlFor={id} className="font-normal">{label}</FieldLabel>
    <Input id={id} className="w-24" type="number" min={1} max={max} step={1} value={draft ?? value}
      onChange={event => setDraft(event.target.value)}
      onBlur={event => {
        const next = event.currentTarget.valueAsNumber
        if (Number.isInteger(next) && next >= 1 && next <= max && next !== value) onChange(next)
        setDraft(null)
      }}
      onKeyDown={event => { if (event.key === 'Enter') event.currentTarget.blur() }} />
  </SettingRow>
}

function NotificationPreferences({ mode, refreshEnabled, autoDismiss, patch }: {
  mode: string
  refreshEnabled: boolean
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

  return (
    <SettingSection title="桌面提醒">
      <SettingCard>
        <ChoiceRow name="notification-mode" label="通知方式" value={mode} disabled={authorizing}
          choices={[['toast', '应用内通知', 'panel-right-open'], ['native', '系统通知', 'monitor-cog']]}
          onChange={next => { void selectMode(next) }} />
        <SettingRow>
          <FieldLabel htmlFor="refresh-notifications-enabled" className="font-normal">订阅更新通知</FieldLabel>
          <Switch id="refresh-notifications-enabled" checked={refreshEnabled}
            onCheckedChange={checked => patch({ refresh_notifications_enabled: checked })} />
        </SettingRow>
        {refreshEnabled && (
          <SettingRow>
            <FieldLabel htmlFor="refresh-toast-auto-dismiss" className="font-normal">自动收起应用内订阅更新通知</FieldLabel>
            <Switch id="refresh-toast-auto-dismiss" checked={autoDismiss}
              onCheckedChange={checked => patch({ refresh_toast_auto_dismiss: checked })} />
          </SettingRow>
        )}
      </SettingCard>
    </SettingSection>
  )
}
