import { useLayoutEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { libraryAPI } from '@/lib/library-api'
import type { Article } from '@/lib/library'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useWorkspace } from '@/stores/workspace'
import { useChatTarget } from './chat-scope'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/api'

const statusLabels: Record<string, string> = {
  running: '正在回复…',
  failed: '回复失败',
  cancelled: '已取消',
  interrupted: '上次回复已中断',
}

const errorLabels: Record<string, string> = {
  ContextUnavailable: '有已选文章无法读取，请重新选择。',
  ConfigurationError: '请先分配聊天模型。',
  CredentialError: '请检查服务的 API 密钥。',
  AuthenticationError: '服务拒绝了当前密钥。',
  RateLimitError: '服务限流，请稍后重试。',
  Conflict: '当前对话还有一条回复正在进行。',
  StorageError: '无法保存对话。',
  Timeout: '模型回复超时。',
  ResponseLimit: '回复超过长度限制，已保留部分内容。',
  IncompleteResponse: '服务提前结束，已保留部分内容。',
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

function ChatSession({ target, close }: { target: NonNullable<ReturnType<typeof useChatTarget.getState>['target']>; close: () => void }) {
  const cache = useQueryClient()
  const [chosen, setChosen] = useState<string | null>(null)
  const [draft, setDraft] = useState('')
  const [includeContext, setIncludeContext] = useState(false)
  const [selection, setSelection] = useState(target.selection ?? '')
  const [sources, setSources] = useState<Article[]>([])
  const [picking, setPicking] = useState(false)
  const timeline = useRef<HTMLDivElement>(null)
  const follow = useRef(true)
  const scope = { kind: target.kind, id: target.id }
  const listKey = ['chat', 'conversations', scope.kind, scope.id] as const
  const history = useQuery({ queryKey: listKey, queryFn: () => api.List(scope) })
  const model = useQuery({ queryKey: ['chat', 'model'], queryFn: () => api.Model(), retry: false })
  const conversationId = chosen ?? history.data?.[0]?.id ?? ''
  const messagesKey = ['chat', 'messages', conversationId] as const
  const messages = useQuery({
    queryKey: messagesKey,
    queryFn: () => api.Messages(conversationId),
    enabled: Boolean(conversationId),
    refetchInterval: query => query.state.data?.some(message => message.status === 'running') ? 250 : false,
  })
  const active = messages.data?.some(message => message.status === 'running') ?? false
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
      await api.Send({ conversationId: id, text: draft, includeContext, selection, articleIds: sources.map(article => article.id) })
      await cache.invalidateQueries({ queryKey: ['chat', 'messages', id] })
    },
    onSuccess: () => {
      follow.current = true
      setDraft('')
      setSelection('')
      setSources([])
      setPicking(false)
      setIncludeContext(false)
    },
  })
  const cancel = useMutation({
    mutationFn: () => api.Cancel(conversationId),
    onSuccess: () => cache.invalidateQueries({ queryKey: messagesKey }),
  })
  useLayoutEffect(() => {
    const element = timeline.current
    if (element && follow.current) element.scrollTop = element.scrollHeight
  }, [messages.data])
  function choose(id: string) {
    follow.current = true
    setChosen(id)
  }
  return <div className="flex min-h-0 flex-col gap-3">
    <div className="flex flex-wrap items-center gap-3">
      <span className="min-w-0 truncate" title={target.label}>{target.label}</span>
      {model.data ? <span className="text-muted-foreground">{model.data.displayName}</span> : model.isError &&
        <Button size="sm" variant="outline" onClick={() => { close(); useWorkspace.getState().showSettings('ai') }}>配置聊天模型</Button>}
      <select aria-label="对话历史" value={conversationId} onChange={event => choose(event.target.value)} disabled={send.isPending}>
        <option value="">新对话</option>
        {history.data?.map(item => <option key={item.id} value={item.id}>{item.title}</option>)}
      </select>
      {history.isError && <Button size="sm" variant="ghost" onClick={() => void history.refetch()}>重新读取历史</Button>}
    </div>
    <div className="h-[min(40vh,400px)] min-h-20 space-y-3 overflow-y-auto" ref={timeline} aria-label="对话消息" onScroll={event => {
      const { scrollHeight, scrollTop, clientHeight } = event.currentTarget
      follow.current = scrollHeight - scrollTop - clientHeight < 32
    }}>
      {messages.isError && <Button size="sm" variant="ghost" onClick={() => void messages.refetch()}>重新读取消息</Button>}
      {messages.data?.map(message => <section key={message.id} className={message.role === 'user' ? 'ml-auto w-fit max-w-[90%] rounded-xl bg-muted p-3' : 'mr-3 rounded-xl border p-3'}>
        <p className="text-xs text-muted-foreground">{message.role === 'user' ? '你' : message.modelName}</p>
        <p className="whitespace-pre-wrap break-words">{message.text}</p>
        {statusLabels[message.status] && <p className="mt-1 text-sm text-muted-foreground">
          {statusLabels[message.status]}{errorLabels[message.errorCode] && ` · ${errorLabels[message.errorCode]}`}
        </p>}
      </section>)}
    </div>
    {target.kind === 'global' && <div className="space-y-2">
      <Button size="sm" variant="outline" aria-expanded={picking} disabled={send.isPending} onClick={() => setPicking(!picking)}>附加文章</Button>
      {sources.map(article => <div key={article.id} className="flex items-center justify-between gap-2">
        <span className="truncate" title={article.title}>{article.title}</span>
        <Button size="sm" variant="ghost" aria-label={`移除 ${article.title}`} disabled={send.isPending} onClick={() => setSources(sources.filter(item => item.id !== article.id))}>移除</Button>
      </div>)}
      {picking && <ArticlePicker selected={sources} disabled={send.isPending} onPick={article => setSources(items => items.length < 3 && !items.some(item => item.id === article.id) ? [...items, article] : items)} />}
    </div>}
    {target.kind !== 'global' && <label className="flex gap-2 text-sm">
      <input type="checkbox" checked={includeContext} disabled={send.isPending} onChange={event => setIncludeContext(event.target.checked)} />
      {target.kind === 'article' ? '附加订阅正文（最多 24 KiB）' : '附加当前分类最近 5 篇文章的摘要'}
    </label>}
    {selection && <div className="flex items-center gap-2 text-sm">
      <span className="truncate" title={selection}>选段 · {selection}</span>
      <Button size="sm" variant="ghost" disabled={send.isPending} onClick={() => setSelection('')}>移除</Button>
    </div>}
    <form className="flex items-end gap-2" onSubmit={event => {
      event.preventDefault()
      if (!active && draft.trim() && model.data && !send.isPending) send.mutate()
    }}>
      <textarea className="min-h-16 w-full resize-y rounded-md border bg-background p-2" aria-label="消息" placeholder="输入消息…" value={draft} onChange={event => setDraft(event.target.value)} maxLength={2700} disabled={send.isPending} />
      {active ? <Button type="button" variant="outline" disabled={cancel.isPending} onClick={() => cancel.mutate()}>停止回复</Button> :
        <Button type="submit" disabled={!draft.trim() || !model.data || send.isPending || history.isPending || (Boolean(conversationId) && messages.isPending)}>发送</Button>}
    </form>
    {(send.isError || cancel.isError) && <p role="alert">{errorText(send.error ?? cancel.error)}</p>}
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
