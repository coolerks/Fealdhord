import { SearchableSelect } from '@/components/ui/searchable-select'
import { Checkbox } from '@/components/ui/checkbox'
import { Textarea } from '@/components/ui/textarea'
import { useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, BookOpen, MessageSquare, Plus, Settings, Sparkles, Square } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { SemanticButton } from '@/components/semantic-button'
import { Input } from '@/components/ui/input'
import { libraryAPI } from '@/lib/library-api'
import type { Article } from '@/lib/library'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useWorkspace } from '@/stores/workspace'
import { useChatTarget } from './chat-scope'
import { aiConfigurationKey } from '@/lib/ai-configuration'
import { useReaderCodeTheme } from '@/features/reader/code-themes'
import { ChatMarkdown } from './ChatMarkdown'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/api'
import { List as listAIConfiguration } from '../../../bindings/github.com/coolerks/Fealdhord/internal/aiconfig/api'
import './chat-workspace.css'

const globalTarget = { kind: 'global', id: '', label: 'AI 助手' }
const defaultEffortOption = { value: ':default', label: '使用服务默认' }
type WorkspaceView = { sidebar: HTMLElement | null; collapsed: boolean; theme: string }

const statusLabels: Record<string, string> = {
  running: '正在回复…',
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
  ChatReasoningUnavailable: '所选思考档位已不可用，请重新选择。',
  ChatReasoningToolsUnavailable: '这个服务的自定义思考强度暂不支持本轮读取正文。请使用服务默认，或取消本轮读取授权。',
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

export function GlobalChat({ sidebar, collapsed, theme }: WorkspaceView) {
  function returnToReading() {
    useChatTarget.setState({ target: null })
    requestAnimationFrame(() => document.getElementById(collapsed ? 'sidebar-toggle' : 'open-global-chat')?.focus({ preventScroll: true }))
  }
  return <main className="reading-workspace chat-workspace" aria-label="AI 助手工作区">
    <ChatSession target={globalTarget} close={returnToReading} workspace={{ sidebar, collapsed, theme }} />
  </main>
}

function ChatSession({ target, close, workspace }: {
  target: NonNullable<ReturnType<typeof useChatTarget.getState>['target']>; close: () => void; workspace?: WorkspaceView
}) {
  const cache = useQueryClient()
  const showSettings = useWorkspace(state => state.showSettings)
  const [chosen, setChosen] = useState<string | null>(null)
  const [draft, setDraft] = useState('')
  const [includeContext, setIncludeContext] = useState(false)
  const [allowArticleReads, setAllowArticleReads] = useState(false)
  const [selection, setSelection] = useState(target.selection ?? '')
  const [sources, setSources] = useState<Article[]>([])
  const [picking, setPicking] = useState(false)
  const [modelChoice, setModelChoice] = useState<string | null>(null)
  const [effortChoices, setEffortChoices] = useState<Record<string, string>>({})
  const [following, setFollowing] = useState(true)
  const composer = useRef<HTMLTextAreaElement>(null)
  const timeline = useRef<HTMLDivElement>(null)
  const follow = useRef(true)
  const scope = { kind: target.kind, id: target.id }
  const listKey = ['chat', 'conversations', scope.kind, scope.id] as const
  const history = useQuery({ queryKey: listKey, queryFn: () => api.List(scope) })
  const model = useQuery({ queryKey: ['chat', 'model'], queryFn: () => api.Model(), enabled: !workspace, retry: false })
  const configuration = useQuery({ queryKey: aiConfigurationKey, queryFn: listAIConfiguration, enabled: Boolean(workspace), retry: false })
  const providers = configuration.data?.providers ?? []
  const modelOptions = (configuration.data?.models ?? []).flatMap(item => {
    const provider = providers.find(service => service.id === item.providerId && service.enabled)
    return item.enabled && provider ? [{
      value: item.id, label: item.displayName, keywords: [item.name, provider.name],
      accessibleLabel: `${item.displayName} · ${provider.name} · ${item.name}`,
    }] : []
  })
  const assigned = configuration.data?.assignments?.find(item => item.task === 'chat')?.modelId
  const defaultModelID = modelOptions.find(option => option.value === assigned)?.value ?? modelOptions[0]?.value
  const modelID = modelChoice ?? defaultModelID
  const modelReady = workspace ? modelOptions.some(option => option.value === modelID) : Boolean(model.data)
  const effortLevels = configuration.data?.models?.find(item => item.id === modelID)?.reasoningLevels ?? []
  const reasoningEffort = modelID ? effortChoices[modelID] : undefined
  const invalidEffort = Boolean(reasoningEffort && !effortLevels.some(option => option.value === reasoningEffort))
  const conversationId = chosen ?? history.data?.[0]?.id ?? ''
  const messagesKey = ['chat', 'messages', conversationId] as const
  const messages = useQuery({
    queryKey: messagesKey,
    queryFn: () => api.Messages(conversationId),
    enabled: Boolean(conversationId),
    refetchInterval: query => query.state.data?.some(message => message.status === 'running') ? 250 : false,
  })
  const active = messages.data?.some(message => message.status === 'running') ?? false
  const theme = useReaderCodeTheme(workspace?.theme ?? 'app', Boolean(workspace && messages.data?.some(message => message.role === 'assistant')))
  const send = useMutation({
    mutationFn: async () => {
      if (workspace && modelID) setModelChoice(modelID)
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
        ...(workspace && reasoningEffort ? { reasoningEffort } : {}),
      })
      await Promise.all([
        cache.invalidateQueries({ queryKey: ['chat', 'messages', id] }),
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
    onSuccess: (_result, id) => cache.invalidateQueries({ queryKey: ['chat', 'messages', id] }),
  })
  useLayoutEffect(() => {
    const element = timeline.current
    if (element && follow.current) element.scrollTop = element.scrollHeight
  }, [messages.data, conversationId, theme, following])
  const inWorkspace = Boolean(workspace)
  useLayoutEffect(() => {
    if (inWorkspace) composer.current?.focus({ preventScroll: true })
  }, [inWorkspace])
  function choose(id: string) {
    follow.current = true
    setFollowing(true)
    setChosen(id)
    if (workspace && id !== conversationId) {
      setDraft('')
      setSources([])
      setPicking(false)
      setAllowArticleReads(false)
      send.reset()
      cancel.reset()
    }
    if (workspace && !id) composer.current?.focus({ preventScroll: true })
  }
  const canSend = !active && Boolean(draft.trim()) && modelReady && !invalidEffort && !send.isPending && !history.isPending &&
    !(conversationId && messages.isPending)
  const retryHistory = <Button size="sm" variant="ghost" onClick={() => void history.refetch()}>重新读取历史</Button>
  return <div className={workspace ? 'chat-session reader-html' : 'flex min-h-0 flex-col gap-3'} data-code-theme={workspace ? theme : undefined}>
    {workspace?.sidebar && createPortal(
      <aside id="workspace-sidebar" className="glass-sidebar chat-sidebar" aria-label="AI 会话导航" inert={workspace.collapsed}
        aria-hidden={workspace.collapsed || undefined}>
        <div className="sidebar-titlebar" />
        <div className="chat-sidebar-actions">
          <Button variant="outline" disabled={send.isPending} onClick={() => choose('')}><Plus />新对话</Button>
        </div>
        <nav className="chat-conversations" aria-label="会话">
          <h2>会话</h2>
          {history.isPending && <p role="status">正在读取会话…</p>}
          {history.isError && retryHistory}
          <ul>{history.data?.map(item => <li key={item.id}>
            <Button variant={item.id === conversationId ? 'secondary' : 'ghost'} title={item.title}
              aria-current={item.id === conversationId ? 'page' : undefined} disabled={send.isPending} onClick={() => choose(item.id)}>
              <MessageSquare /><span>{item.title}</span>
            </Button>
          </li>)}</ul>
        </nav>
        <footer className="sidebar-footer"><div className="sidebar-bottom">
          <Button variant="ghost" className="sidebar-ai" onClick={close}><AnimatedGlyph icon={BookOpen} meaning="book" />阅读</Button>
          <SemanticButton icon={Settings} meaning="settings" label="AI 设置" onClick={() => showSettings('ai')} />
        </div></footer>
      </aside>, workspace.sidebar)}
    {workspace ? <header className="workspace-toolbar chat-header" data-sidebar-collapsed={workspace.collapsed}>
      {workspace.collapsed && <SemanticButton icon={BookOpen} meaning="book" label="阅读" onClick={close} />}
      <Sparkles className="chat-heading-icon" aria-hidden="true" />
      <h1>{history.data?.find(item => item.id === conversationId)?.title ?? (conversationId ? 'AI 助手' : '新对话')}</h1>
      {workspace.collapsed && <Button variant="ghost" size="icon" aria-label="新对话" disabled={send.isPending} onClick={() => choose('')}><Plus /></Button>}
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
      {history.isError && retryHistory}
    </div>}
    <div className={workspace ? 'chat-timeline' : 'h-[min(40vh,400px)] min-h-20 space-y-3 overflow-y-auto'}
      ref={timeline} role={workspace ? 'region' : undefined} aria-label="对话消息" onScroll={event => {
      const { scrollHeight, scrollTop, clientHeight } = event.currentTarget
      follow.current = scrollHeight - scrollTop - clientHeight < 32
      if (workspace) setFollowing(follow.current)
    }}>
      {messages.isError && <Button size="sm" variant="ghost" onClick={() => void messages.refetch()}>重新读取消息</Button>}
      {messages.data?.map(message => <section key={message.id} className={workspace ? 'chat-message' : message.role === 'user' ? 'ml-auto w-fit max-w-[90%] rounded-xl bg-muted p-3' : 'mr-3 rounded-xl border p-3'} data-role={workspace ? message.role : undefined}>
        {(!workspace || message.role !== 'user') && <p className="text-xs text-muted-foreground">{message.role === 'user' ? '你' : message.modelName}</p>}
        {workspace && message.role === 'assistant' ? <ChatMarkdown text={message.text} /> : <p className="whitespace-pre-wrap break-words">{message.text}</p>}
        {!!message.tools?.length && <ul className="mt-2 text-xs text-muted-foreground" aria-label="工具活动">
          {message.tools.map(tool => <li key={tool.id}>{toolStatusLabels[tool.status] ?? '文章工具状态未知'}</li>)}
        </ul>}
        {statusLabels[message.status] && <p className="mt-1 text-sm text-muted-foreground">
          {statusLabels[message.status]}{errorLabels[message.errorCode] && ` · ${errorLabels[message.errorCode]}`}
        </p>}
      </section>)}
    </div>
    {workspace && !following && <Button className="chat-latest" variant="outline" size="icon" aria-label="回到最新" onClick={() => {
      follow.current = true
      setFollowing(true)
    }}><ArrowDown /></Button>}
    <div className={workspace ? 'chat-composer' : 'flex flex-col gap-3'}>
      {workspace && (sources.length > 0 || picking) && <div className="chat-context">
        {sources.map(article => <div key={article.id} className="flex items-center justify-between gap-2">
          <span className="truncate" title={article.title}>{article.title}</span>
          <Button size="sm" variant="ghost" aria-label={`移除 ${article.title}`} disabled={send.isPending} onClick={() => { setSources(sources.filter(item => item.id !== article.id)); setAllowArticleReads(false) }}>移除</Button>
        </div>)}
        {picking && <ArticlePicker selected={sources} disabled={send.isPending} onPick={article => {
          setSources(items => items.length < 3 && !items.some(item => item.id === article.id) ? [...items, article] : items)
          setAllowArticleReads(false)
        }} />}
      </div>}
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
        <Textarea ref={composer} className={workspace ? 'chat-input' : 'max-h-40 resize-y'} aria-label="消息" placeholder="输入消息…" value={draft} onChange={event => setDraft(event.target.value)} maxLength={2700}
          readOnly={Boolean(workspace && send.isPending)} disabled={!workspace && send.isPending}
          onKeyDown={workspace ? event => {
            if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing && event.keyCode !== 229) {
              event.preventDefault()
              if (canSend) send.mutate()
            }
          } : undefined} />
        <div className={workspace ? 'chat-composer-toolbar' : undefined}>
          {workspace &&
            <Button size="icon" type="button" variant="ghost" aria-label="附加文章" title="附加文章" aria-expanded={picking} disabled={send.isPending} onClick={() => setPicking(!picking)}><Plus /></Button>
          }
          <div className={workspace ? 'chat-composer-actions' : undefined}>
            {workspace && (modelOptions.length ? <SearchableSelect label="聊天模型" className="chat-model" value={modelID ?? ''}
              options={modelOptions} variant="ghost" announceSelection placeholder="选择模型" disabled={send.isPending} onValueChange={id => { setModelChoice(id); send.reset() }} />
              : configuration.isError ? <Button type="button" variant="ghost" onClick={() => void configuration.refetch()}>重新读取模型</Button>
                : configuration.isPending ? <span role="status">正在读取模型…</span>
                  : <Button type="button" variant="ghost" onClick={() => showSettings('ai')}>配置模型</Button>)}
            {workspace && modelID && modelReady && (effortLevels.length > 0 || reasoningEffort) &&
              <SearchableSelect label="思考强度" className="chat-effort" value={reasoningEffort || defaultEffortOption.value}
                variant="ghost" announceSelection options={[defaultEffortOption, ...effortLevels]} placeholder="选择档位"
                disabled={send.isPending} onValueChange={effort => {
                  setEffortChoices(current => ({ ...current, [modelID]: effort === defaultEffortOption.value ? '' : effort }))
                  send.reset()
                }} />}
            {workspace ? (active
              ? <Button type="button" size="icon" className="rounded-full" aria-label="停止回复" title="停止回复" disabled={cancel.isPending} onClick={() => cancel.mutate(conversationId)}><Square fill="currentColor" /></Button>
              : <Button type="submit" size="icon" className="rounded-full" aria-label="发送" title="发送" disabled={!canSend}><ArrowUp /></Button>)
              : active ? <Button type="button" variant="outline" disabled={cancel.isPending} onClick={() => cancel.mutate(conversationId)}>停止回复</Button>
                : <Button type="submit" disabled={!canSend}>发送</Button>}
          </div>
        </div>
      </form>
      {workspace && modelChoice && !modelReady && !send.isError && <p role="alert">{errorLabels.ChatModelUnavailable}</p>}
      {workspace && invalidEffort && !send.isError && <p role="alert">{errorLabels.ChatReasoningUnavailable}</p>}
      {(send.isError || cancel.isError) && <p role="alert">{errorText(send.error ?? cancel.error)}</p>}
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
