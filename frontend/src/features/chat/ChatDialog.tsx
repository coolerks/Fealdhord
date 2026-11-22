import { clearSelectionWithin } from '@/app/TextSelection'
import { ChatSlashMenu } from './ChatSlashMenu'
import type { SlashItem, SlashPage } from './ChatSlashMenu'
import { ChatContext } from './ChatContext'
import { ScheduledTasks } from './ScheduledTasks'
import { ModelBrand } from '@/features/settings/ModelMetadata'
import { skillsKey } from '@/features/settings/AIPlatformSettings'
import { Switch } from '@/components/ui/switch'
import { Skills } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { Checkbox } from '@/components/ui/checkbox'
import { Textarea } from '@/components/ui/textarea'
import { useEffect, useEffectEvent, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { motion, useIsPresent } from 'motion/react'
import { ArrowDown, ArrowUp, BookOpen, Box, Brain, CalendarClock, Gauge, Paperclip, Plus, Settings, Sparkles, Square, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { SemanticButton } from '@/components/semantic-button'
import { SidebarAction } from '@/components/sidebar-action'
import { Input } from '@/components/ui/input'
import { libraryAPI } from '@/lib/library-api'
import type { Article } from '@/lib/library'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useWorkspace } from '@/stores/workspace'
import { cn } from '@/lib/utils'
import { native } from '@/lib/native'
import { moveSidebarFocus } from '@/lib/sidebar-navigation'
import { useChatTarget } from './chat-scope'
import { aiConfigurationKey } from '@/lib/ai-configuration'
import { useReaderCodeTheme } from '@/features/reader/code-themes'
import { ChatMarkdown } from './ChatMarkdown'
import { chatMessagesKey, useChatMessages } from './useChatMessages'
import { ReplyActions } from './ReplyActions'
import { notify } from '@/lib/notifications'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/api'
import { List as listAIConfiguration } from '../../../bindings/github.com/coolerks/Fealdhord/internal/aiconfig/api'
import './chat-workspace.css'

const globalTarget = { kind: 'global', id: '', label: 'AI 助手' }
const defaultEffortOption = { value: ':default', label: '使用服务默认' }
const slashCommand = /^\/([^\s/\\]*)$/u
type WorkspaceView = {
  sidebar: HTMLElement | null; collapsed: boolean; theme: string
  motionMode: 'full' | 'subtle' | 'off'; present: boolean
}

function chatTransition(mode: WorkspaceView['motionMode']) {
  const hidden = { opacity: 0, '--workspace-mode-offset': mode === 'full' ? '6px' : '0px' }
  return {
    initial: mode === 'off' ? false as const : hidden,
    animate: { opacity: 1, '--workspace-mode-offset': '0px' },
    exit: mode === 'off' ? undefined : hidden,
  }
}

const statusLabels: Record<string, string> = {
  failed: '回复失败',
  cancelled: '已取消',
  interrupted: '上次回复已中断',
}

const toolStatusLabels: Record<string, string> = {
  running: '正在读取文章',
  completed: '已读取授权文章',
  denied: '已拒绝未授权的文章读取',
  failed: '文章读取失败',
  cancelled: '已取消文章读取',
  interrupted: '文章读取已中断',
}

const errorLabels: Record<string, string> = {
  ContextUnavailable: '有已选文章无法读取，请重新选择。',
  ConfigurationError: '请先分配聊天模型。',
  ChatModelUnavailable: '所选模型已不可用，请重新选择。',
  ContextWindowExceeded: '本轮内容超过模型上下文限制，请缩短消息或减少附加内容。',
  ChatReasoningUnavailable: '所选思考档位已不可用，请重新选择。',
  ChatReasoningToolsUnavailable: '当前思考档位无法与本轮工具同时使用。请选择服务默认，或关闭本轮工具并取消正文读取授权。',
  ProviderHTTPError: '模型服务拒绝了请求，请检查模型设置及思考档位，或稍后重试。',
  CredentialError: '请检查服务的 API 密钥。',
  AuthenticationError: '服务拒绝了当前密钥。',
  RateLimitError: '服务限流，请稍后重试。',
  Conflict: '当前对话还有一条回复正在进行。',
  StorageError: '无法保存对话。',
  Timeout: '模型回复超时。',
  AgentLimitExceeded: '已达到本轮工具调用上限，已保留部分回复。',
  ToolNotAllowed: '模型请求了未授权的工具。',
  ToolCallConflict: '模型重复使用了工具调用编号。',
  ResponseLimit: '回复超过长度限制，已保留部分内容。',
  IncompleteResponse: '服务提前结束，已保留部分内容。',
  ProviderProtocolError: '服务返回的流式响应不完整或格式有误。',
  ValidationError: '请检查消息及附加内容的长度。',
}

function errorText(error: unknown) {
  const message = error instanceof Error ? error.message : ''
  return Object.entries(errorLabels).find(([code]) => message.includes(code))?.[1] ?? '操作失败，请重试。'
}

export function ChatDialog() {
  const target = useChatTarget(state => state.target)
  const close = () => useChatTarget.setState({ target: null })
  return <Dialog open={Boolean(target)} onOpenChange={value => { if (!value) close() }}>
    {target && <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto w-[min(92vw,760px)] sm:max-w-none">
      <DialogHeader><DialogTitle>AI 助手</DialogTitle></DialogHeader>
      <ChatSession key={`${target.kind}:${target.id}`} target={target} close={close} />
    </DialogContent>}
  </Dialog>
}

export function GlobalChat({ sidebar, collapsed, theme, motionMode, onReady, suspended }: Omit<WorkspaceView, 'present'> & {
  onReady: (ready: boolean) => void
  suspended: boolean
}) {
  const present = useIsPresent()
  const surface = useRef<HTMLElement>(null)
  useLayoutEffect(() => {
    if (suspended && surface.current) clearSelectionWithin(surface.current)
  }, [suspended])
  useLayoutEffect(() => {
    onReady(true)
    return () => onReady(false)
  }, [onReady])
  function returnToReading() {
    useChatTarget.setState({ target: null })
    requestAnimationFrame(() => {
      if (useChatTarget.getState().target?.kind === 'global') return
      document.getElementById(collapsed ? 'sidebar-toggle' : 'open-global-chat')?.focus({ preventScroll: true })
    })
  }
  return <motion.main ref={surface} hidden={suspended} className={cn('reading-workspace chat-workspace workspace-mode-chat', suspended && 'settings-obscured')} aria-label="AI 助手工作区"
    inert={!present || suspended} aria-hidden={!present || suspended || undefined} {...chatTransition(motionMode)}>
    <ChatSession target={globalTarget} close={returnToReading} workspace={{ sidebar, collapsed, theme, motionMode, present: present && !suspended }} />
  </motion.main>
}

function ChatSession({ target, close, workspace }: {
  target: NonNullable<ReturnType<typeof useChatTarget.getState>['target']>; close: () => void; workspace?: WorkspaceView
}) {
  const cache = useQueryClient()
  const showSettings = useWorkspace(state => state.showSettings)
  const rememberedModelID = useWorkspace(state => state.preferences?.chat_model_id)
  const patchPreferences = useWorkspace(state => state.patch)
  const [chosen, setChosen] = useState<string | null>(null)
  const [draft, setDraft] = useState('')
  const [composerFocused, setComposerFocused] = useState(false)
  const [slashDismissed, setSlashDismissed] = useState(false)
  const [slashPage, setSlashPage] = useState<SlashPage>('actions')
  const [highlightedSlash, setHighlightedSlash] = useState('')
  const [contextOpen, setContextOpen] = useState(false)
  const [tasksOpen, setTasksOpen] = useState(false)
  const slashQuery = workspace && composerFocused && !slashDismissed ? slashCommand.exec(draft)?.[1] : undefined
  const [includeContext, setIncludeContext] = useState(false)
  const [allowArticleReads, setAllowArticleReads] = useState(false)
  const [selection, setSelection] = useState(target.selection ?? '')
  const [sources, setSources] = useState<Article[]>([])
  const [disableTools, setDisableTools] = useState(false)
  const [selectedSkills, setSelectedSkills] = useState<string[]>([])
  const [picking, setPicking] = useState(false)
  const [effortChoices, setEffortChoices] = useState<Record<string, string>>({})
  const [following, setFollowing] = useState(true)
  const [conversationsScrolled, setConversationsScrolled] = useState(false)
  const chatContent = useRef<HTMLDivElement>(null)
  const composer = useRef<HTMLTextAreaElement>(null)
  const timeline = useRef<HTMLDivElement>(null)
  const follow = useRef(true)
  const scope = { kind: target.kind, id: target.id }
  const listKey = ['chat', 'conversations', scope.kind, scope.id] as const
  const history = useQuery({
    queryKey: listKey,
    queryFn: () => api.List(scope),
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  })
  const model = useQuery({ queryKey: ['chat', 'model'], queryFn: () => api.Model(), enabled: !workspace, retry: false })
  const configuration = useQuery({ queryKey: aiConfigurationKey, queryFn: listAIConfiguration, enabled: Boolean(workspace), retry: false })
  const skills = useQuery({ queryKey: skillsKey, queryFn: () => Skills(), enabled: Boolean(workspace && (picking || slashQuery !== undefined)), retry: false })
  const providers = configuration.data?.providers ?? []
  const modelOptions = (configuration.data?.models ?? []).flatMap(item => {
    const provider = providers.find(service => service.id === item.providerId && service.enabled)
    return item.enabled && provider ? [{
      value: item.id, label: item.displayName, keywords: [item.name, provider.name],
      accessibleLabel: `${item.displayName} · ${provider.name} · ${item.name}`,
      graphic: <ModelBrand brand={item.metadata?.brand} override={item.metadata?.brandOverride} />,
      reasoningLevels: item.reasoningLevels ?? [],
    }] : []
  })
  const modelSubmenus = Object.fromEntries(modelOptions.flatMap(({ value, label, reasoningLevels: levels }) => {
    const effort = effortChoices[value]
    if (levels.length === 0 && !effort) return []
    return [[value, {
      label: `${label}的思考强度`,
      options: [defaultEffortOption, ...levels],
      value: effort || defaultEffortOption.value,
      summary: levels.find(level => level.value === effort)?.label,
    }]]
  }))
  const assigned = configuration.data?.assignments?.find(item => item.task === 'chat')?.modelId
  const selectedModel = modelOptions.find(option => option.value === rememberedModelID) ??
    modelOptions.find(option => option.value === assigned) ?? modelOptions[0]
  const modelID = selectedModel?.value
  const modelReady = workspace ? Boolean(selectedModel) : Boolean(model.data)
  const selectedConfig = configuration.data?.models?.find(item => item.id === modelID)
  const effortLevels = selectedConfig?.reasoningLevels ?? []
  const reasoningEffort = modelID ? effortChoices[modelID] : undefined
  const invalidEffort = Boolean(reasoningEffort && !effortLevels.some(option => option.value === reasoningEffort))
  const conversationId = chosen ?? history.data?.[0]?.id ?? ''
  const messages = useChatMessages(conversationId)
  const active = messages.data?.some(message => message.status === 'running') ?? false
  const theme = useReaderCodeTheme(workspace?.theme ?? 'app', Boolean(workspace && messages.data?.some(message => message.role === 'assistant')))
  const send = useMutation({
    mutationFn: async () => {
      let id = conversationId
      if (!id) {
        const title = Array.from(draft.trim()).slice(0, 64).join('')
        const conversation = await api.Create(scope, title)
        id = conversation.id
        setChosen(id)
        void cache.invalidateQueries({ queryKey: listKey })
      }
      await api.Send({
        conversationId: id, text: draft, includeContext, selection, allowArticleReads,
        articleIds: sources.map(article => article.id), ...(workspace && modelID ? { modelId: modelID } : {}),
        ...(workspace && disableTools ? { disableTools: true } : {}),
        ...(workspace && selectedSkills.length ? { skills: selectedSkills } : {}),
        ...(workspace && reasoningEffort ? { reasoningEffort } : {}),
      })
      await Promise.all([
        cache.invalidateQueries({ queryKey: chatMessagesKey(id) }),
        workspace && cache.invalidateQueries({ queryKey: listKey }),
      ])
    },
    onError: () => {
      if (workspace) void cache.invalidateQueries({ queryKey: aiConfigurationKey })
    },
    onSuccess: () => {
      follow.current = true
      setFollowing(true)
      setDraft('')
      setSelection('')
      setSources([])
      setPicking(false)
      setIncludeContext(false)
      setAllowArticleReads(false)
    },
  })
  const cancel = useMutation({
    mutationFn: (id: string) => api.Cancel(id),
    onSuccess: (_result, id) => cache.invalidateQueries({ queryKey: chatMessagesKey(id) }),
  })
  const branch = useMutation({
    mutationFn: ({ id, messageId }: { id: string; messageId: string }) => api.Branch(id, messageId),
    onSuccess: (created, { id }) => {
      void cache.invalidateQueries({ queryKey: listKey })
      if (conversationId === id) {
        choose(created.id)
        composer.current?.focus({ preventScroll: true })
      }
    },
    onError: () => { void notify('无法创建聊天分支', undefined, true) },
  })
  function selectModel(id: string) {
    if (id !== rememberedModelID) patchPreferences({ chat_model_id: id })
    send.reset()
  }
  useLayoutEffect(() => {
    const element = timeline.current
    if (element && follow.current) element.scrollTop = element.scrollHeight
  }, [messages.data, conversationId, theme, following, tasksOpen])
  const inWorkspace = Boolean(workspace?.present)
  useLayoutEffect(() => {
    if (inWorkspace && !tasksOpen) composer.current?.focus({ preventScroll: true })
  }, [inWorkspace, tasksOpen])
  function openTasks() {
    if (chatContent.current) clearSelectionWithin(chatContent.current)
    setTasksOpen(true)
  }
  function choose(id: string) {
    setTasksOpen(false)
    follow.current = true
    setFollowing(true)
    setChosen(id)
    if (workspace && id !== conversationId) {
      setSlashPage('actions')
      setHighlightedSlash('')
      setDraft('')
      setSources([])
      setPicking(false)
      setAllowArticleReads(false)
      send.reset()
      cancel.reset()
    }
    if (workspace && !id) composer.current?.focus({ preventScroll: true })
  }
  const openNewConversation = useEffectEvent(() => {
    if (!send.isPending) choose('')
  })
  useEffect(() => {
    if (!inWorkspace) return
    function onKeyDown(event: KeyboardEvent) {
      const modifier = native.isMac() ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey
      if (event.defaultPrevented || event.isComposing || event.repeat || !modifier || !event.shiftKey ||
        event.altKey || event.key.toLowerCase() !== 'n') return
      const { settingsOpen, helpOpen, diagnosticsOpen, paletteOpen } = useWorkspace.getState()
      if (settingsOpen || helpOpen || diagnosticsOpen || paletteOpen ||
        (event.target instanceof Element && event.target.closest('[role=dialog], [role=menu]'))) return
      event.preventDefault()
      openNewConversation()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [inWorkspace])
  function toggleSkill(name: string) {
    setSelectedSkills(current => current.includes(name) ? current.filter(item => item !== name) :
      current.length < 3 ? [...current, name] : current)
  }
  const skillCommands: SlashItem[] = (skills.data ?? []).filter(skill => skill.enabled)
    .filter(skill => selectedSkills.length < 3 || selectedSkills.includes(skill.name))
    .map(skill => ({
      key: `skill:${skill.name}`, label: skill.name, icon: Sparkles,
      detail: selectedSkills.includes(skill.name) ? '移除此技能' : skill.description,
      action: 'skill', value: skill.name,
    }))
  const actions: SlashItem[] = [
    { key: 'skills', label: '技能', icon: Sparkles, detail: '应用已启用的技能', aliases: ['skill'], page: 'skills' },
    { key: 'new', label: '新对话', icon: Plus, detail: '开启空白对话', aliases: ['new-chat'], action: 'new' },
    { key: 'attach', label: '附加文章', icon: Paperclip, detail: '从本地文章中选择', aliases: ['article'], action: 'attach' },
    ...(modelOptions.length ? [{ key: 'model', label: '模型', icon: Box, page: 'models' } satisfies SlashItem] : []),
    ...(effortLevels.length ? [{ key: 'reasoning', label: '思考档位', icon: Brain, detail: selectedModel && modelSubmenus[selectedModel.value]?.summary,
      aliases: ['effort', '推理'], page: 'effort' } satisfies SlashItem] : []),
    ...(modelReady ? [{ key: 'context', label: '上下文', icon: Gauge, detail: '查看用量、整理或重置上下文',
      aliases: ['status', 'compact', '压缩', '整理'], action: 'context' } satisfies SlashItem] : []),
    { key: 'tasks', label: '定时任务', icon: CalendarClock, detail: '查看或修改计划', aliases: ['schedule'], action: 'tasks' },
    { key: 'settings', label: 'AI 设置', icon: Settings, action: 'settings' },
    ...skillCommands,
  ]
  const pages: Record<SlashPage, SlashItem[]> = {
    actions,
    skills: [...skillCommands, { key: 'manage-skills', label: '管理技能', icon: Settings, action: 'settings' }],
    models: modelOptions.map(option => ({ key: option.value, label: option.label, aliases: option.keywords, icon: Box,
      detail: option.value === modelID ? '当前模型' : undefined, action: 'model', value: option.value })),
    effort: modelID ? [defaultEffortOption, ...effortLevels].map(level => ({
      key: level.value, label: level.label, icon: Brain,
      detail: (reasoningEffort || defaultEffortOption.value) === level.value ? '当前档位' : undefined,
      action: 'effort', value: level.value,
    })) : [],
  }
  const slashItems = slashQuery === undefined ? [] : pages[slashPage].filter(item =>
    [item.key, item.label, item.detail ?? '', ...(item.aliases ?? [])].some(text => text.toLowerCase().includes(slashQuery.toLowerCase())),
  )
  const slashActive = slashItems.find(item => item.key === highlightedSlash) ?? slashItems[0]
  const slashOpen = slashQuery !== undefined && !send.isPending &&
    (slashQuery === '' || slashItems.length > 0 || slashPage !== 'actions')
  const slashLabel = { actions: '斜杠命令', skills: '选择技能', models: '切换模型', effort: '选择思考档位' }[slashPage]
  function chooseSlash(item: SlashItem) {
    setHighlightedSlash('')
    if ('page' in item) {
      setSlashPage(item.page)
      setDraft('/')
      return
    }
    setDraft('')
    setSlashPage('actions')
    switch (item.action) {
      case 'new': choose(''); break
      case 'attach': setPicking(true); break
      case 'context': setContextOpen(true); break
      case 'settings': showSettings(item.key === 'manage-skills' ? 'skills' : 'ai'); break
      case 'tasks': openTasks(); break
      case 'skill': toggleSkill(item.value); break
      case 'model': selectModel(item.value); break
      case 'effort':
        if (modelID) setEffortChoices(current => ({ ...current, [modelID]: item.value === defaultEffortOption.value ? '' : item.value }))
        break
    }
  }
  const newConversationShortcut = native.isMac() ? 'Meta+Shift+N' : 'Control+Shift+N'
  const canSend = !slashOpen && !active && Boolean(draft.trim()) && modelReady && !invalidEffort && !send.isPending && !history.isPending &&
    !(conversationId && messages.isPending)
  return <div className={workspace ? 'chat-session reader-html' : 'flex min-h-0 flex-col gap-3'} data-code-theme={workspace ? theme : undefined}>
    {workspace?.sidebar && createPortal(
      <motion.aside id={workspace.present ? 'workspace-sidebar' : undefined} className="glass-sidebar chat-sidebar workspace-mode-chat"
        aria-label="AI 会话导航" inert={workspace.collapsed || !workspace.present}
        aria-hidden={workspace.collapsed || !workspace.present || undefined} {...chatTransition(workspace.motionMode)}>
        <div className="sidebar-titlebar" />
        <div className="system-nav">
          <SidebarAction label="新对话" meaning="add" shortcut={newConversationShortcut} disabled={send.isPending} onClick={() => choose('')} />
          <Button variant={tasksOpen ? 'secondary' : 'ghost'} className={cn('nav-item', tasksOpen && 'is-current')}
            aria-current={tasksOpen ? 'page' : undefined} onClick={openTasks}>
            {tasksOpen && <span className="nav-selected-surface" />}
            <CalendarClock />定时任务
          </Button>
        </div>
        <div className="sidebar-organize-wrap" data-scrolled={conversationsScrolled}>
        <nav className="sidebar-organize" aria-label="会话" onKeyDown={moveSidebarFocus}
          onScroll={event => setConversationsScrolled(event.currentTarget.scrollTop > 0)}>
          <div className="nav-section-heading"><h2>会话</h2></div>
          {history.isPending && <p role="status">正在读取会话…</p>}
          <ul className="feed-nav">{history.data?.map(item => {
            const current = !tasksOpen && item.id === conversationId
            return <li key={item.id}>
              <Button data-navigation={item.id} variant={current ? 'secondary' : 'ghost'} className={cn('nav-item', current && 'is-current')}
                aria-current={current ? 'page' : undefined} disabled={send.isPending} onClick={() => choose(item.id)}>
                {current && <span className="nav-selected-surface" />}
                <AnimatedGlyph meaning="message-square" /><span className="nav-node-label">{item.title}</span>
              </Button>
            </li>
          })}</ul>
        </nav>
        </div>
        <footer className="sidebar-footer"><div className="sidebar-bottom">
          <Button variant="ghost" className="sidebar-ai" onClick={close}><AnimatedGlyph icon={BookOpen} meaning="book" />阅读</Button>
          <SemanticButton id="open-ai-settings" icon={Settings} meaning="settings" label="AI 设置" onClick={() => showSettings('ai')} />
        </div></footer>
      </motion.aside>, workspace.sidebar)}
    {workspace && tasksOpen && <ScheduledTasks collapsed={workspace.collapsed} onBack={() => setTasksOpen(false)} />}
    <div ref={chatContent} hidden={tasksOpen} className={tasksOpen ? undefined : 'contents'}>
    {workspace ? <header className="workspace-toolbar chat-header" data-sidebar-collapsed={workspace.collapsed}>
      {workspace.collapsed && <SemanticButton icon={BookOpen} meaning="book" label="阅读" onClick={close} />}
      <Sparkles className="chat-heading-icon" aria-hidden="true" />
      <h1>{history.data?.find(item => item.id === conversationId)?.title ?? (conversationId ? 'AI 助手' : '新对话')}</h1>
      {workspace.collapsed && <Button type="button" variant="ghost" size="icon" aria-label="定时任务" title="定时任务" onClick={openTasks}><CalendarClock /></Button>}
      {workspace.collapsed && <SemanticButton icon={Plus} meaning="add" label="新对话" aria-keyshortcuts={newConversationShortcut}
        disabled={send.isPending} onClick={() => choose('')} />}
    </header> : <div className="flex flex-wrap items-center gap-3">
      <span className="min-w-0 truncate" title={target.label}>{target.label}</span>
      {model.data ? <span className="text-muted-foreground">{model.data.displayName}</span> : model.isError &&
        <Button size="sm" variant="outline" onClick={() => { close(); useWorkspace.getState().showSettings('ai') }}>配置聊天模型</Button>}
      <SearchableSelect label="对话历史" className="max-w-full" value={conversationId || 'new-conversation'}
        onValueChange={value => choose(value === 'new-conversation' ? '' : value)} disabled={send.isPending}
        options={[
          { value: 'new-conversation', label: '新对话' },
          ...(history.data ?? []).map(item => ({ value: item.id, label: item.title })),
        ]} />
    </div>}
    <div className={workspace ? 'chat-timeline' : 'h-[min(40vh,400px)] min-h-20 space-y-3 overflow-y-auto'}
      ref={timeline} role={workspace ? 'region' : undefined} aria-label="对话消息" tabIndex={workspace ? -1 : undefined} onScroll={event => {
      const { scrollHeight, scrollTop, clientHeight } = event.currentTarget
      follow.current = scrollHeight - scrollTop - clientHeight < 32
      if (workspace) setFollowing(follow.current)
    }}>
      {messages.isError && <Button size="sm" variant="ghost" onClick={() => void messages.refetch()}>重新读取消息</Button>}
      {messages.data?.map(message => <section key={message.id} className={workspace ? 'chat-message' : message.role === 'user' ? 'ml-auto w-fit max-w-[90%] rounded-xl bg-muted p-3' : 'mr-3 rounded-xl border p-3'} data-role={workspace ? message.role : undefined}>
        {!workspace && message.role === 'user' && <p className="text-xs text-muted-foreground">你</p>}
        {message.text && (workspace && message.role === 'assistant'
          ? <ChatMarkdown text={message.text} streaming={message.status === 'running'}
            reveal={workspace.motionMode === 'off' ? undefined : message.reveal} />
          : <p className="whitespace-pre-wrap break-words">{message.text}</p>)}
        {!!message.tools?.length && <ul className="mt-2 text-xs text-muted-foreground" aria-label="工具活动">
          {message.tools.map(tool => <li key={tool.id}>{toolStatusLabels[tool.status] ?? '文章工具状态未知'}</li>)}
        </ul>}
        {statusLabels[message.status] && <p className="mt-1 text-sm text-muted-foreground">
          {statusLabels[message.status]}{errorLabels[message.errorCode] && ` · ${errorLabels[message.errorCode]}`}
        </p>}
        {message.role === 'assistant' && message.status !== 'running' && <ReplyActions message={message} branchDisabled={branch.isPending || send.isPending}
          onBranch={() => branch.mutate({ id: conversationId, messageId: message.id })} />}
      </section>)}
    </div>
    {workspace && !following && <Button className="chat-latest" variant="outline" size="icon" aria-label="回到最新" onClick={() => {
      timeline.current?.focus({ preventScroll: true })
      follow.current = true
      setFollowing(true)
    }}><ArrowDown /></Button>}
    <div className={workspace ? 'chat-composer' : 'flex flex-col gap-3'}>
      {workspace && selectedSkills.length > 0 && <div className="flex flex-wrap gap-1.5">
        {selectedSkills.map(name => <div key={name} className="flex items-center gap-1 rounded-md bg-muted pl-2 text-xs">
          <span>{name}</span>
          <Button type="button" size="icon-xs" variant="ghost" aria-label={`移除技能 ${name}`} disabled={send.isPending} onClick={() => toggleSkill(name)}><X /></Button>
        </div>)}
      </div>}
      {workspace && sources.length > 0 && <div className="chat-context">
        {sources.map(article => <div key={article.id} className="flex items-center justify-between gap-2">
          <span className="truncate" title={article.title}>{article.title}</span>
          <Button size="sm" variant="ghost" aria-label={`移除 ${article.title}`} disabled={send.isPending} onClick={() => { setSources(sources.filter(item => item.id !== article.id)); setAllowArticleReads(false) }}>移除</Button>
        </div>)}
      </div>}
      {workspace && <Dialog open={picking} onOpenChange={setPicking}>
        {picking && <DialogContent aria-describedby={undefined}>
          <DialogHeader><DialogTitle>附加文章</DialogTitle></DialogHeader>
        <ArticlePicker selected={sources} disabled={send.isPending} onPick={article => {
          setSources(items => items.length < 3 && !items.some(item => item.id === article.id) ? [...items, article] : items)
          setAllowArticleReads(false)
        }} />
          <div className="flex items-center gap-2">
            <Switch id="chat-tools" checked={!disableTools} onCheckedChange={value => setDisableTools(!value)} />
            <label htmlFor="chat-tools">可调用工具</label>
          </div>
          {skills.data?.some(skill => skill.enabled) && <SearchableSelect label="应用 Skills" value={selectedSkills}
            placeholder={selectedSkills.length ? `已选 ${selectedSkills.length} 个 Skill` : '应用 Skills'}
            options={skills.data.filter(skill => skill.enabled).map(skill => ({
              value: skill.name, label: skill.name, keywords: [skill.description],
              disabled: selectedSkills.length >= 3 && !selectedSkills.includes(skill.name),
            }))} onValueChange={toggleSkill} />}
        </DialogContent>}
      </Dialog>}
      {target.kind !== 'global' && <label htmlFor="chat-include-context" className="flex items-center gap-2 text-sm">
        <Checkbox id="chat-include-context" checked={includeContext} disabled={send.isPending} onCheckedChange={value => setIncludeContext(value === true)} />
        {target.kind === 'article' ? '附加订阅正文（最多 24 KiB）' : '附加当前分类最近 5 篇文章的摘要'}
      </label>}
      {(target.kind === 'article' || (target.kind === 'global' && sources.length > 0)) &&
        <label htmlFor="chat-allow-article-reads" className="flex items-center gap-2 text-sm">
          <Checkbox id="chat-allow-article-reads" checked={allowArticleReads} disabled={send.isPending}
            onCheckedChange={value => setAllowArticleReads(value === true)} />
          {target.kind === 'article' ? '允许模型读取当前文章正文' : '允许模型读取已选文章正文'}
        </label>}
      {selection && <div className="flex items-center gap-2 text-sm">
        <span className="truncate" title={selection}>选段 · {selection}</span>
        <Button size="sm" variant="ghost" disabled={send.isPending} onClick={() => setSelection('')}>移除</Button>
      </div>}
      <form className={workspace ? 'chat-composer-form' : 'flex items-end gap-2'} onSubmit={event => {
        event.preventDefault()
        if (canSend) send.mutate()
      }}>
        {slashOpen && <ChatSlashMenu composer={composer} label={slashLabel} active={slashActive} items={slashItems}
          onHighlight={setHighlightedSlash} onChoose={chooseSlash} />}
        <Textarea ref={composer} className={workspace ? 'chat-input' : 'max-h-40 resize-y'} aria-label="消息" placeholder="输入消息…" value={draft}
          onFocus={workspace ? () => setComposerFocused(true) : undefined}
          onBlur={workspace ? () => setComposerFocused(false) : undefined}
          onChange={event => {
            const next = event.target.value
            setDraft(next)
            setSlashDismissed(false)
            setHighlightedSlash('')
            if (!slashCommand.test(next)) setSlashPage('actions')
          }} maxLength={2700}
          readOnly={Boolean(workspace && send.isPending)} disabled={!workspace && send.isPending}
          onKeyDown={workspace ? event => {
            if (event.nativeEvent.isComposing || event.keyCode === 229) return
            if (slashOpen && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) {
              event.preventDefault()
              const current = slashActive ? slashItems.indexOf(slashActive) : -1
              const next = current + (event.key === 'ArrowDown' ? 1 : -1)
              if (slashItems.length) setHighlightedSlash(slashItems[(next + slashItems.length) % slashItems.length].key)
              return
            }
            if (slashOpen && (event.key === 'Escape' || event.key === 'Backspace' && slashPage !== 'actions' && draft === '/')) {
              event.preventDefault()
              event.stopPropagation()
              if (slashPage === 'actions') setSlashDismissed(true)
              else {
                setSlashPage('actions')
                setHighlightedSlash('')
                setDraft('/')
              }
              return
            }
            if (event.key === 'Enter' && !event.shiftKey) {
              event.preventDefault()
              if (slashOpen) {
                if (slashActive) chooseSlash(slashActive)
              } else if (canSend) send.mutate()
            }
          } : undefined} />
        <div className={workspace ? 'chat-composer-toolbar' : undefined}>
          {workspace &&
            <Button size="icon" type="button" variant="ghost" aria-label="附加文章" title="附加文章" aria-expanded={picking} disabled={send.isPending} onClick={() => setPicking(!picking)}><Plus /></Button>
          }
          <div className={workspace ? 'chat-composer-actions' : undefined}>
            {workspace && selectedConfig && modelID && <ChatContext id={conversationId} modelId={modelID} modelRevision={selectedConfig.revision}
              disabled={active || send.isPending} attachments={sources} open={contextOpen} onOpenChange={setContextOpen} />}
            {workspace && (modelOptions.length ? <SearchableSelect label="聊天模型" className="chat-model" value={modelID ?? ''}
              options={modelOptions} variant="ghost" announceSelection placeholder="选择模型" disabled={send.isPending}
              submenus={modelSubmenus} onValueChange={selectModel}
              onSubValueChange={(id, effort) => {
                selectModel(id)
                setEffortChoices(current => ({ ...current, [id]: effort === defaultEffortOption.value ? '' : effort }))
              }} />
              : configuration.isError ? <Button type="button" variant="ghost" onClick={() => void configuration.refetch()}>重新读取模型</Button>
                : configuration.isPending ? <span role="status">正在读取模型…</span>
                  : <Button type="button" variant="ghost" onClick={() => showSettings('ai')}>配置模型</Button>)}
            {workspace ? (active
              ? <Button type="button" size="icon" className="rounded-full" aria-label="停止回复" title="停止回复" disabled={cancel.isPending} onClick={() => cancel.mutate(conversationId)}><Square fill="currentColor" /></Button>
              : <Button type="submit" size="icon" className="rounded-full" aria-label="发送" title="发送" disabled={!canSend}><ArrowUp /></Button>)
              : active ? <Button type="button" variant="outline" disabled={cancel.isPending} onClick={() => cancel.mutate(conversationId)}>停止回复</Button>
                : <Button type="submit" disabled={!canSend}>发送</Button>}
          </div>
        </div>
      </form>
      {workspace && invalidEffort && !send.isError && <p role="alert">{errorLabels.ChatReasoningUnavailable}</p>}
      {(send.isError || cancel.isError) && <p role="alert">{errorText(send.error ?? cancel.error)}</p>}
    </div>
    </div>
  </div>
}

function ArticlePicker({ selected, disabled, onPick }: { selected: Article[]; disabled: boolean; onPick: (article: Article) => void }) {
  const [draft, setDraft] = useState('')
  const [term, setTerm] = useState('')
  const results = useQuery({
    queryKey: ['chat', 'articles', term],
    queryFn: ({ signal }) => libraryAPI.list({ scope: { kind: 'all' }, search: term, sort: 'relevance', limit: 8 }, signal),
    enabled: Boolean(term),
    retry: false,
  })
  return <div className="space-y-2">
    <form className="flex gap-2" onSubmit={event => {
      event.preventDefault()
      const value = draft.trim()
      if (value && value === term) void results.refetch()
      else setTerm(value)
    }}>
      <Input aria-label="搜索本地文章" placeholder="搜索本地文章…" value={draft} maxLength={128} onChange={event => setDraft(event.target.value)} />
      <Button size="sm" type="submit" variant="outline" disabled={!draft.trim() || disabled}>搜索</Button>
    </form>
    {term && results.isFetching && <p role="status">正在搜索…</p>}
    {results.isError && <p role="alert">无法搜索文章，请重试。</p>}
    {results.data?.items.length === 0 && <p role="status">没有匹配的文章。</p>}
    <ul className="max-h-28 space-y-1 overflow-y-auto">
      {results.data?.items.map(article => <li className="flex items-center justify-between gap-2" key={article.id}>
        <span className="truncate" title={article.title}>{article.title}</span>
        <Button size="sm" variant="ghost" aria-label={`附加 ${article.title} 的摘要`} disabled={disabled || selected.length >= 3 || selected.some(item => item.id === article.id)} onClick={() => onPick(article)}>附加摘要</Button>
      </li>)}
    </ul>
    {selected.length >= 3 && <p role="status">已选 3 篇文章。</p>}
  </div>
}
