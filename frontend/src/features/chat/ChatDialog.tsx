import { clearSelectionWithin } from '@/app/TextSelection'
import { ChatSlashMenu } from './ChatSlashMenu'
import type { SlashItem, SlashPage } from './ChatSlashMenu'
import { AppMark, pickGreeting } from './ChatGreeting'
import { ChatContext } from './ChatContext'
import { ScheduledTasks } from './ScheduledTasks'
import { ModelBrand } from '@/features/settings/ModelMetadata'
import { skillsKey } from '@/features/settings/AIPlatformSettings'
import { Skills } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { ChatModelMenu } from './ChatModelMenu'
import { ChatTimelineRail } from './ChatTimelineRail'
import { Checkbox } from '@/components/ui/checkbox'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useEffect, useEffectEvent, useLayoutEffect, useRef, useState } from 'react'
import type { ClipboardEvent } from 'react'
import { createPortal } from 'react-dom'
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import { motion, useIsPresent } from 'motion/react'
import { ArrowDown, ArrowUp, BookOpen, Box, Brain, CalendarClock, Check, ClipboardList, Copy, CornerDownRight, File as FileIcon, Folder, Gauge, Image as ImageIcon, Pin, Plus, Settings, Sparkles, Square, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { SemanticButton } from '@/components/semantic-button'
import { SidebarAction } from '@/components/sidebar-action'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useWorkspace } from '@/stores/workspace'
import { cn } from '@/lib/utils'
import { native } from '@/lib/native'
import { moveSidebarFocus } from '@/lib/sidebar-navigation'
import { useChatTarget } from './chat-scope'
import { aiConfigurationKey } from '@/lib/ai-configuration'
import { useReaderCodeTheme } from '@/features/reader/code-themes'
import { chatMessagesKey, useChatMessages } from './useChatMessages'
import { ReplyActions } from './ReplyActions'
import { ReplyBody } from './ReplyProgress'
import { ToolApproval, useInlineApproval } from './AIApprovals'
import { UserQuestionPrompt, useInlineQuestion } from './UserQuestionPrompt'
import { PlanConfirmPrompt, useInlinePlan } from './PlanConfirmPrompt'
import { ChatQueueBar } from './ChatQueueBar'
import type { QueuedChatMessage } from './ChatQueueBar'
import { formatMessageTime, formatMessageTimeTitle } from './format-time'
import { approvalModes } from './approval-options'
import { ConversationConfirmation, ConversationMenu, RenameConversationDialog, archivedConversationsKey, conversationActionError } from './ConversationMenu'
import type { ConversationAction } from './ConversationMenu'
import { archivedGroupsKey, ConversationGroupDialog, GroupDisclosure, GroupIcon, GroupMenu, groupActionError, groupHistoryKey, groupKey, useConversationFolds } from './ConversationGroups'
import type { GroupAction } from './ConversationGroups'
import { ConfirmDialog } from '@/features/settings/settings-primitives'
import { notify } from '@/lib/notifications'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/api'
import type { Conversation, Group, SendAttachment } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'
import { List as listAIConfiguration } from '../../../bindings/github.com/coolerks/Fealdhord/internal/aiconfig/api'
import './chat-workspace.css'

type ComposerAttachment = SendAttachment & { id: string; preview?: string }

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

const errorLabels: Record<string, string> = {
  ContextUnavailable: '有已选文章无法读取，请重新选择。',
  ConfigurationError: '请先分配聊天模型。',
  ChatModelUnavailable: '所选模型已不可用，请重新选择。',
  ContextWindowExceeded: '本轮内容超过模型上下文限制，请缩短消息或减少附加内容。',
  ChatReasoningUnavailable: '所选思考档位已不可用，请重新选择。',
  ChatReasoningToolsUnavailable: '当前思考档位无法与本轮工具同时使用。请将思考档位设为「使用服务默认」。',
  ProviderHTTPError: '模型服务拒绝了请求，请检查模型设置及思考档位，或稍后重试。',
  CredentialError: '请检查服务的 API 密钥。',
  AuthenticationError: '服务拒绝了当前密钥。',
  RateLimitError: '服务限流，请稍后重试。',
  Conflict: '当前对话还有一条回复正在进行。',
  ChatGroupUnavailable: '分组已不可用，可取消分组后发送。',
  StorageError: '无法保存对话。',
  Timeout: '模型回复超时。',
  ArticleReadIncomplete: '正文尚未读完。可在工具记录中查看已读范围，或要求继续读取。',
  AgentLimitExceeded: '已达到本轮工具调用上限，已保留部分回复。',
  ToolNotAllowed: '模型请求了未授权的工具。',
  ToolPermissionsUnavailable: '无法读取自定义工具权限，请检查「设置 → 工具权限」。',
  ToolCallConflict: '模型重复使用了工具调用编号。',
  ResponseLimit: '回复超过长度限制，已保留部分内容。',
  IncompleteResponse: '服务提前结束，已保留部分内容。',
  ProviderProtocolError: '服务返回的流式响应不完整或格式有误。',
  ModelImageUnsupported: '当前模型不是多模态模型，不支持图片。',
  AttachmentUnavailable: '有附加文件或文件夹无法读取，请重新选择。',
  NoActiveRun: '当前回复已结束，请直接发送。',
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
  const defaultApprovalMode = useWorkspace(state => state.preferences?.chat_approval_mode) ?? 'ask'
  const approvalPresentation = useWorkspace(state => state.preferences?.chat_approval_presentation) ?? 'composer'
  const busySendMode = useWorkspace(state => state.preferences?.chat_busy_send_mode) ?? 'queue'
  const patchPreferences = useWorkspace(state => state.patch)
  const [chosen, setChosen] = useState<string | null>(null)
  const [greeting, setGreeting] = useState(() => pickGreeting())
  const [draft, setDraft] = useState('')
  const [composerFocused, setComposerFocused] = useState(false)
  const [slashDismissed, setSlashDismissed] = useState(false)
  const [slashPage, setSlashPage] = useState<SlashPage>('actions')
  const [highlightedSlash, setHighlightedSlash] = useState('')
  const [contextOpen, setContextOpen] = useState(false)
  const [tasksOpen, setTasksOpen] = useState(false)
  const [approvalChoices, setApprovalChoices] = useState<Record<string, string>>({})
  const slashQuery = workspace && composerFocused && !slashDismissed ? slashCommand.exec(draft)?.[1] : undefined
  const [includeContext, setIncludeContext] = useState(false)
  const [allowArticleReads, setAllowArticleReads] = useState(false)
  const [selection, setSelection] = useState(target.selection ?? '')
  const [attachments, setAttachments] = useState<ComposerAttachment[]>([])
  const [planMode, setPlanMode] = useState(false)
  const [queues, setQueues] = useState<Record<string, QueuedChatMessage[]>>({})
  const [selectedSkills, setSelectedSkills] = useState<string[]>([])
  const [effortChoices, setEffortChoices] = useState<Record<string, string>>({})
  const [following, setFollowing] = useState(true)
  const [conversationsScrolled, setConversationsScrolled] = useState(false)
  const [draftGroup, setDraftGroup] = useState('')
  const [groupEditor, setGroupEditor] = useState<{ id?: string } | null>(null)
  const [removingGroup, setRemovingGroup] = useState<Group | null>(null)
  const { closed, toggle } = useConversationFolds()
  const [conversationDialog, setConversationDialog] = useState<{ action: 'rename' | 'archive' | 'delete'; item: Conversation } | null>(null)
  const [conversationTitle, setConversationTitle] = useState('')
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
  const groups = useQuery({ queryKey: groupKey, queryFn: () => api.Groups(), enabled: Boolean(workspace), retry: false })
  const groupItems = groups.data ?? []
  const groupedLists = useQueries({ queries: groupItems.map(group => ({
    queryKey: [...groupHistoryKey, group.id],
    queryFn: () => api.GroupConversations(group.id),
    enabled: Boolean(workspace) && (draftGroup === group.id || (!closed.groups && !closed[group.id])),
    retry: false,
  })) })
  const model = useQuery({ queryKey: ['chat', 'model'], queryFn: () => api.Model(), enabled: !workspace, retry: false })
  const configuration = useQuery({ queryKey: aiConfigurationKey, queryFn: listAIConfiguration, enabled: Boolean(workspace), retry: false })
  const skills = useQuery({ queryKey: skillsKey, queryFn: () => Skills(), enabled: Boolean(workspace && slashQuery !== undefined), retry: false })
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
  const assigned = configuration.data?.assignments?.find(item => item.task === 'chat')?.modelId
  const selectedModel = modelOptions.find(option => option.value === rememberedModelID) ??
    modelOptions.find(option => option.value === assigned) ?? modelOptions[0]
  const modelID = selectedModel?.value
  const modelReady = workspace ? Boolean(selectedModel) : Boolean(model.data)
  const selectedConfig = configuration.data?.models?.find(item => item.id === modelID)
  const modelSupportsImage = Boolean(selectedConfig?.metadata?.modalities?.includes('image'))
  const hasImageAttachment = attachments.some(item => item.kind === 'image')
  const imageBlocked = Boolean(workspace && hasImageAttachment && !modelSupportsImage)
  const effortLevels = selectedConfig?.reasoningLevels ?? []
  const reasoningEffort = modelID ? effortChoices[modelID] : undefined
  const selectedEffortLabel = effortLevels.find(level => level.value === reasoningEffort)?.label
  const invalidEffort = Boolean(reasoningEffort && !effortLevels.some(option => option.value === reasoningEffort))
  const conversationId = chosen ?? history.data?.[0]?.id ?? ''
  const selectedGroup = groupItems.find(group => group.id === draftGroup)
  const groupChange = useMutation({
    mutationFn: ({ group, archive }: { group: Group; archive: boolean }) => archive
      ? api.SetGroupArchived(group.id, true) : api.DeleteGroup(group.id, false),
    onSuccess: async (_result, { group, archive }) => {
      if (draftGroup === group.id) {
        if (archive && conversationId) choose('')
        else if (!archive) setDraftGroup('')
      }
      setRemovingGroup(null)
      cache.setQueryData<Group[]>(groupKey, current => (current ?? []).filter(item => item.id !== group.id))
      await Promise.all([
        cache.invalidateQueries({ queryKey: groupKey }), cache.invalidateQueries({ queryKey: groupHistoryKey }),
        cache.invalidateQueries({ queryKey: archivedGroupsKey }), cache.invalidateQueries({ queryKey: listKey }),
      ])
    },
    onError: (error, { archive }) => { if (archive) void notify(groupActionError(error), undefined, true) },
  })
  const currentQueue = queues[conversationId] ?? []
  const activeConversation = (draftGroup ? groupedLists[groupItems.findIndex(group => group.id === draftGroup)]?.data : history.data)
    ?.find(item => item.id === conversationId)
  const approvalMode = approvalChoices[conversationId] ?? defaultApprovalMode
  const inlineApproval = useInlineApproval(conversationId,
    approvalPresentation === 'composer' && !tasksOpen && (!workspace || workspace.present))
  const messages = useChatMessages(conversationId)
  const active = messages.data?.some(message => message.status === 'running') ?? false
  const inlineQuestion = useInlineQuestion(conversationId, !tasksOpen && (!workspace || workspace.present), active)
  const inlinePlan = useInlinePlan(conversationId, !tasksOpen && (!workspace || workspace.present), active)
  const theme = useReaderCodeTheme(workspace?.theme ?? 'app', Boolean(workspace && messages.data?.some(message => message.role === 'assistant')))
  const send = useMutation({
    mutationFn: async (override?: { text?: string; attachments?: SendAttachment[]; planMode?: boolean; newConversation?: boolean }) => {
      const messageText = override?.text ?? draft
      const sendAttachments: SendAttachment[] = override?.attachments ?? attachments.map(({ kind, name, path, mimeType, data }) => ({
        kind, name, ...(path ? { path } : {}), ...(mimeType ? { mimeType } : {}), ...(data ? { data } : {}),
      }))
      const usePlanMode = override?.planMode ?? planMode
      let id = override?.newConversation ? '' : conversationId
      if (!id) {
        const titleSource = messageText.trim() || sendAttachments[0]?.name || '新对话'
        const title = Array.from(titleSource).slice(0, 64).join('')
        const group = override?.newConversation ? '' : draftGroup
        const conversation = group ? await api.CreateInGroup(group, title) : await api.Create(scope, title)
        id = conversation.id
        setApprovalChoices(current => {
          const { '': draftMode, ...existing } = current
          return draftMode ? { ...existing, [id]: draftMode } : current
        })
        setChosen(id)
        setDraftGroup(group)
        if (group) cache.setQueryData<Conversation[]>([...groupHistoryKey, group], items => [conversation, ...(items ?? [])])
        void cache.invalidateQueries({ queryKey: group ? [...groupHistoryKey, group] : listKey })
      }
      await api.Send({
        conversationId: id, text: messageText, includeContext, selection, allowArticleReads,
        articleIds: [], ...(workspace && modelID ? { modelId: modelID } : {}),
        ...(workspace && selectedSkills.length ? { skills: selectedSkills } : {}),
        ...(workspace && reasoningEffort ? { reasoningEffort } : {}),
        ...(workspace ? { approvalMode } : {}),
        ...(workspace && sendAttachments.length ? { attachments: sendAttachments } : {}),
        ...(workspace && usePlanMode ? { planMode: true } : {}),
      })
      await Promise.all([
        cache.invalidateQueries({ queryKey: chatMessagesKey(id) }),
        workspace && cache.invalidateQueries({ queryKey: listKey }),
        workspace && cache.invalidateQueries({ queryKey: groupHistoryKey }),
      ])
    },
    onError: () => {
      if (workspace) void cache.invalidateQueries({ queryKey: aiConfigurationKey })
    },
    onSuccess: (_result, override) => {
      follow.current = true
      setFollowing(true)
      if (!override) {
        setDraft('')
        setAttachments([])
      }
      setSelection('')
      setIncludeContext(false)
      setAllowArticleReads(false)
    },
  })
  const steer = useMutation({
    mutationFn: async (payload: { text: string; attachments?: SendAttachment[] }) => {
      await api.Steer({
        conversationId,
        text: payload.text,
        ...(payload.attachments?.length ? { attachments: payload.attachments } : {}),
      })
      await cache.invalidateQueries({ queryKey: chatMessagesKey(conversationId) })
    },
    onSuccess: () => {
      void notify('已发送引导，AI 将在下一轮次接收')
    },
    onError: error => {
      void notify(errorText(error), undefined, true)
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
      void cache.invalidateQueries({ queryKey: groupHistoryKey })
      if (conversationId === id) {
        choose(created.id, created.groupId)
        composer.current?.focus({ preventScroll: true })
      }
    },
    onError: () => { void notify('无法创建聊天分支', undefined, true) },
  })
  const manageConversation = useMutation({
    mutationFn: async ({ action, item }: { action: ConversationAction; item: Conversation }) => {
      switch (action) {
        case 'rename': return api.Rename(item.id, conversationTitle.trim())
        case 'pin': return api.SetPinned(item.id, !item.pinned)
        case 'branch': return api.Branch(item.id, '')
        case 'archive': await api.SetArchived(item.id, true); break
        case 'delete': await api.Delete(item.id, false); break
      }
    },
    onSuccess: (result, { action, item }) => {
      setConversationDialog(null)
      if (action === 'branch' && result) choose(result.id, result.groupId)
      if (action === 'archive' || action === 'delete') {
        if (conversationId === item.id) choose('')
        cache.setQueryData<Conversation[]>(listKey, current => current?.filter(entry => entry.id !== item.id))
        cache.removeQueries({ queryKey: chatMessagesKey(item.id) })
        void cache.invalidateQueries({ queryKey: archivedConversationsKey })
      }
      void cache.invalidateQueries({ queryKey: listKey })
      void cache.invalidateQueries({ queryKey: groupHistoryKey })
    },
    onError: (error, { action }) => {
      if (action === 'pin' || action === 'branch') void notify(conversationActionError(error), undefined, true)
    },
  })
  function openConversationAction(action: ConversationAction, item: Conversation) {
    manageConversation.reset()
    if (action === 'pin' || action === 'branch') {
      manageConversation.mutate({ action, item })
      return
    }
    setConversationTitle(item.title)
    setConversationDialog({ action, item })
  }
  function closeConversationAction() {
    setConversationDialog(null)
    manageConversation.reset()
  }
  function selectModel(id: string) {
    if (id !== rememberedModelID) patchPreferences({ chat_model_id: id })
    send.reset()
  }
  useLayoutEffect(() => {
    const element = timeline.current
    if (element && follow.current) element.scrollTop = element.scrollHeight
  }, [messages.data, conversationId, theme, following, tasksOpen, inlineApproval?.id, inlineQuestion?.id, inlinePlan?.id])
  const prevActive = useRef(false)
  const drainQueue = useEffectEvent((item: QueuedChatMessage) => {
    setQueues(current => {
      const list = current[conversationId] ?? []
      return { ...current, [conversationId]: list.filter(entry => entry.id !== item.id) }
    })
    send.mutate({ text: item.text, attachments: item.attachments, planMode: item.planMode })
  })
  useEffect(() => {
    const wasActive = prevActive.current
    prevActive.current = active
    if (!workspace || !wasActive || active || send.isPending) return
    const last = messages.data?.at(-1)
    if (last?.role !== 'assistant' || last.status !== 'complete') return
    const next = queues[conversationId]?.[0]
    if (!next) return
    if (next.attachments.some(att => att.kind === 'image') && !modelSupportsImage) {
      void notify('当前模型不是多模态模型，不支持图片', '排队消息含图片，请切换模型或编辑队列。', true)
      return
    }
    drainQueue(next)
  }, [active, workspace, conversationId, messages.data, queues, send.isPending, modelSupportsImage])
  const inWorkspace = Boolean(workspace?.present)
  useLayoutEffect(() => {
    if (inWorkspace && !tasksOpen) composer.current?.focus({ preventScroll: true })
  }, [inWorkspace, tasksOpen])
  function openTasks() {
    if (chatContent.current) clearSelectionWithin(chatContent.current)
    setTasksOpen(true)
  }
  function choose(id: string, group = '') {
    if (id !== conversationId && timeline.current) clearSelectionWithin(timeline.current)
    setTasksOpen(false)
    follow.current = true
    setFollowing(true)
    setChosen(id)
    setDraftGroup(group)
    if (!id && approvalChoices['']) setApprovalChoices(current => {
      const existing = { ...current }
      delete existing['']
      return existing
    })
    if (workspace && (id !== conversationId || group !== draftGroup)) {
      setSlashPage('actions')
      setHighlightedSlash('')
      setDraft('')
      setAttachments([])
      setPlanMode(false)
      setAllowArticleReads(false)
      send.reset()
      cancel.reset()
    }
    if (!id) setGreeting(pickGreeting())
    if (workspace && !id) composer.current?.focus({ preventScroll: true })
  }
  function newConversation() {
    if (!send.isPending) choose('', workspace ? selectedGroup?.id : undefined)
  }
  function manageGroup(group: Group, action: GroupAction) {
    if (action === 'edit') setGroupEditor({ id: group.id })
    if (action === 'archive') groupChange.mutate({ group, archive: true })
    if (action === 'remove') {
      groupChange.reset()
      setRemovingGroup(group)
    }
  }
  const openNewConversation = useEffectEvent(newConversation)
  const pendingPrompt = useChatTarget(state => state.pendingPrompt)
  const submitPendingPrompt = useEffectEvent((text: string) => {
    choose('')
    if (modelReady && !invalidEffort) {
      send.mutate({ text, newConversation: true })
    } else {
      setDraft(text)
    }
  })
  useEffect(() => {
    if (!workspace || !pendingPrompt || configuration.isPending) return
    useChatTarget.setState(current => (current.pendingPrompt?.id === pendingPrompt.id ? { pendingPrompt: null } : current))
    submitPendingPrompt(pendingPrompt.text)
  }, [workspace, pendingPrompt, configuration.isPending])
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
    { key: 'plan', label: '计划模式', icon: ClipboardList, detail: planMode ? '关闭计划模式' : '先澄清需求并确认计划后再执行', aliases: ['plan', '计划'], action: 'plan' },
    ...(modelOptions.length ? [{ key: 'model', label: '模型', icon: Box, page: 'models' } satisfies SlashItem] : []),
    ...(effortLevels.length ? [{ key: 'reasoning', label: '思考档位', icon: Brain, detail: selectedEffortLabel,
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
      case 'new': newConversation(); break
      case 'plan': setPlanMode(current => !current); break
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
  const hasInput = Boolean(draft.trim()) || (Boolean(workspace) && attachments.length > 0)
  const canSend = !slashOpen && !active && hasInput && modelReady && !invalidEffort && !imageBlocked && !send.isPending && !history.isPending &&
    !(conversationId && messages.isPending)
  const canBusySend = Boolean(workspace && active && !slashOpen && hasInput && !imageBlocked && !steer.isPending)
  const isEmptyWorkspace = Boolean(workspace && !tasksOpen && (!conversationId || (!messages.isPending && !messages.isError && (messages.data?.length ?? 0) === 0)))
  const hasComposerChips = Boolean(workspace && (planMode || selectedSkills.length > 0 || attachments.length > 0))

  function addAttachments(incoming: ComposerAttachment[]) {
    if (incoming.length === 0) return
    if (incoming.some(item => item.kind === 'image') && !modelSupportsImage) {
      void notify('当前模型不是多模态模型，不支持图片', '请移除图片或切换至支持多模态的模型。', true)
    }
    setAttachments(current => {
      const next = [...current]
      for (const item of incoming) {
        if (next.length >= 10) break
        if (item.path && next.some(existing => existing.path === item.path)) continue
        next.push(item)
      }
      return next
    })
  }

  async function pickAttachments(kind: 'files' | 'folders') {
    try {
      const items = await api.PickAttachments(kind)
      if (!items?.length) return
      addAttachments(items.map((item, idx) => ({
        id: `${item.path}-${Date.now()}-${idx}`,
        kind: item.kind,
        name: item.name,
        path: item.path,
        mimeType: item.mimeType,
        preview: item.preview,
      })))
    } catch {
      void notify('无法选择文件或文件夹', undefined, true)
    }
  }

  async function handleComposerPaste(event: ClipboardEvent<HTMLTextAreaElement>) {
    if (!workspace) return
    const items = Array.from(event.clipboardData?.items ?? [])
    const imageFiles = items
      .filter(item => item.kind === 'file' && item.type.startsWith('image/'))
      .map(item => item.getAsFile())
      .filter((file): file is File => Boolean(file))
    const hasPlainText = Boolean(event.clipboardData?.getData('text/plain'))
    if (imageFiles.length > 0) {
      if (!hasPlainText) event.preventDefault()
      const converted: ComposerAttachment[] = []
      for (const file of imageFiles) {
        const dataUrl = await new Promise<string>((resolve, reject) => {
          const reader = new FileReader()
          reader.onload = () => resolve(String(reader.result ?? ''))
          reader.onerror = () => reject(new Error('read'))
          reader.readAsDataURL(file)
        }).catch(() => '')
        const comma = dataUrl.indexOf(',')
        if (comma > 0) {
          converted.push({
            id: `paste-img-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
            kind: 'image',
            name: file.name || '粘贴的图片.png',
            mimeType: file.type || 'image/png',
            data: dataUrl.slice(comma + 1),
            preview: dataUrl,
          })
        }
      }
      if (converted.length > 0) {
        addAttachments(converted)
        return
      }
    }
    if (!hasPlainText && typeof api.InspectClipboardFiles === 'function') {
      try {
        const nativeFiles = await api.InspectClipboardFiles()
        if (nativeFiles?.length) {
          event.preventDefault()
          addAttachments(nativeFiles.map((item, idx) => ({
            id: `paste-path-${Date.now()}-${idx}`,
            kind: item.kind,
            name: item.name,
            path: item.path,
            mimeType: item.mimeType,
            preview: item.preview,
          })))
        }
      } catch {
        /* 忽略非桌面环境剪贴板查询失败 */
      }
    }
  }

  function submitComposer() {
    if (imageBlocked) {
      void notify('当前模型不是多模态模型，不支持图片', '请移除图片或切换至支持多模态的模型。', true)
      return
    }
    if (workspace && active) {
      if (!canBusySend) return
      const payloadAttachments: SendAttachment[] = attachments.map(({ kind, name, path, mimeType, data }) => ({
        kind, name, ...(path ? { path } : {}), ...(mimeType ? { mimeType } : {}), ...(data ? { data } : {}),
      }))
      const trimmed = draft.trim()
      if (busySendMode === 'steer') {
        steer.mutate({ text: trimmed, attachments: payloadAttachments })
      } else {
        const queued: QueuedChatMessage = {
          id: `q-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
          text: trimmed,
          attachments: payloadAttachments,
          planMode,
        }
        setQueues(current => ({ ...current, [conversationId]: [...(current[conversationId] ?? []), queued] }))
      }
      setDraft('')
      setAttachments([])
      return
    }
    if (canSend) send.mutate()
  }
  function conversationRows(items: Conversation[] | null | undefined) {
    return items?.map(item => {
      const current = !tasksOpen && item.id === conversationId
      return <li key={item.id} className="chat-sidebar-row">
        <ConversationMenu item={item} active={item.id === conversationId && active} busy={send.isPending || manageConversation.isPending}
          onAction={openConversationAction}>
          <Button type="button" data-navigation={item.id} variant={current ? 'secondary' : 'ghost'} className={cn('nav-item', current && 'is-current')}
            title={item.title} aria-description={item.pinned ? '已置顶' : undefined} aria-current={current ? 'page' : undefined} disabled={send.isPending}
            onClick={() => choose(item.id, item.groupId)}>
            <span className="nav-node-label">{item.title}</span>
            {item.pinned && <Pin className="ml-auto" aria-hidden="true" />}
          </Button>
        </ConversationMenu>
      </li>
    })
  }
  return <div className={workspace ? 'chat-session reader-html' : 'flex min-h-0 flex-col gap-3'} data-code-theme={workspace ? theme : undefined} data-empty={isEmptyWorkspace ? 'true' : undefined}>
    {workspace?.sidebar && createPortal(
      <motion.aside id={workspace.present ? 'workspace-sidebar' : undefined} className="glass-sidebar chat-sidebar workspace-mode-chat"
        aria-label="AI 会话导航" inert={workspace.collapsed || !workspace.present}
        aria-hidden={workspace.collapsed || !workspace.present || undefined} {...chatTransition(workspace.motionMode)}>
        <div className="sidebar-titlebar" />
        <div className="system-nav">
          <SidebarAction label="新对话" meaning="add" shortcut={newConversationShortcut} disabled={send.isPending} onClick={newConversation} />
          <Button variant={tasksOpen ? 'secondary' : 'ghost'} className={cn('nav-item', tasksOpen && 'is-current')}
            aria-current={tasksOpen ? 'page' : undefined} onClick={openTasks}>
            {tasksOpen && <span className="nav-selected-surface" />}
            <CalendarClock />定时任务
          </Button>
        </div>
        <div className="sidebar-organize-wrap" data-scrolled={conversationsScrolled}>
        <nav className="sidebar-organize" aria-label="会话" onKeyDown={moveSidebarFocus}
          onScroll={event => setConversationsScrolled(event.currentTarget.scrollTop > 0)}>
          <div className="chat-sidebar-row">
            <GroupDisclosure label="分组" open={!closed.groups} controls="chat-groups" onToggle={() => toggle('groups')} />
            <Button type="button" variant="ghost" size="icon-sm" aria-label="新建分组" title="新建分组"
              onClick={() => setGroupEditor({})}><Plus aria-hidden="true" /></Button>
          </div>
          <ul id="chat-groups" hidden={closed.groups} inert={closed.groups}>
            {!closed.groups && groupItems.map((group, index) => <li key={group.id}>
              <GroupMenu onAction={action => { if (!groupChange.isPending && !send.isPending) manageGroup(group, action) }}>
                <div className="chat-sidebar-row">
                  <GroupDisclosure label={group.name} group={group} open={!closed[group.id]} controls={`chat-group-${group.id}`} onToggle={() => toggle(group.id)} />
                  <Button type="button" variant="ghost" size="icon-sm" aria-label={`在${group.name}中新建对话`} disabled={send.isPending}
                    onClick={() => choose('', group.id)}><Plus aria-hidden="true" /></Button>
                </div>
              </GroupMenu>
              <ul id={`chat-group-${group.id}`} className="feed-nav chat-group-conversations" hidden={closed[group.id]} inert={closed[group.id]}>
                {!closed[group.id] && conversationRows(groupedLists[index]?.data)}
              </ul>
            </li>)}
          </ul>
          <div className="chat-sidebar-row"><GroupDisclosure label="会话" open={!closed.history} controls="chat-history" onToggle={() => toggle('history')} /></div>
          <div id="chat-history" hidden={closed.history} inert={closed.history}>
            {!closed.history && history.isPending && <p role="status">正在读取会话…</p>}
            {!closed.history && !history.isPending && !history.isError && history.data?.length === 0 && <p className="px-2 py-1.5 text-xs text-muted-foreground">暂无会话</p>}
            <ul className="feed-nav">{!closed.history && conversationRows(history.data)}</ul>
          </div>
        </nav>
        </div>
        <footer className="sidebar-footer"><div className="sidebar-bottom">
          <Button variant="ghost" className="sidebar-ai" onClick={close}><AnimatedGlyph icon={BookOpen} meaning="book" />阅读</Button>
          <SemanticButton id="open-ai-settings" icon={Settings} meaning="settings" label="AI 设置" onClick={() => showSettings('ai')} />
        </div></footer>
      </motion.aside>, workspace.sidebar)}
    {workspace && groupEditor && <ConversationGroupDialog groupID={groupEditor.id} onClose={() => setGroupEditor(null)} onSaved={group => {
      setGroupEditor(null)
      if (closed.groups) toggle('groups')
      const { folders, ...item } = group
      cache.setQueryData([...groupKey, group.id], { ...item, folders })
      cache.setQueryData<Group[]>(groupKey, current => groupEditor.id
        ? (current ?? []).map(existing => existing.id === item.id ? item : existing)
        : [item, ...(current ?? [])])
    }} />}
    {workspace && removingGroup && <ConfirmDialog open title={`移除“${removingGroup.name}”？`}
      description="磁盘目录和会话记录将保留。组内未归档的会话将移到普通会话列表。" confirmLabel="移除分组"
      busy={groupChange.isPending} error={groupChange.isError ? groupActionError(groupChange.error) : undefined}
      onOpenChange={open => { if (!open) setRemovingGroup(null) }}
      onConfirm={() => groupChange.mutate({ group: removingGroup, archive: false })} />}
    {workspace && conversationDialog && (conversationDialog.action === 'rename'
      ? <RenameConversationDialog item={conversationDialog.item} title={conversationTitle} onChange={setConversationTitle}
        busy={manageConversation.isPending} error={manageConversation.isError ? conversationActionError(manageConversation.error) : undefined}
        onClose={closeConversationAction} onConfirm={() => manageConversation.mutate(conversationDialog)} />
      : <ConversationConfirmation item={conversationDialog.item} action={conversationDialog.action}
        busy={manageConversation.isPending} error={manageConversation.isError ? conversationActionError(manageConversation.error) : undefined}
        onClose={closeConversationAction} onConfirm={() => manageConversation.mutate(conversationDialog)} />)}
    {workspace && tasksOpen && <ScheduledTasks collapsed={workspace.collapsed} onBack={() => setTasksOpen(false)} />}
    <div ref={chatContent} hidden={tasksOpen} className={tasksOpen ? undefined : 'contents'}>
    {workspace ? <header className="workspace-toolbar chat-header" data-sidebar-collapsed={workspace.collapsed}>
      {workspace.collapsed && <SemanticButton icon={BookOpen} meaning="book" label="阅读" onClick={close} />}
      <h1>{activeConversation?.title ?? (conversationId ? 'AI 助手' : draftGroup ? (groupItems.find(group => group.id === draftGroup)?.name ?? '新对话') : '新对话')}</h1>
      {workspace.collapsed && <Button type="button" variant="ghost" size="icon" aria-label="定时任务" title="定时任务" onClick={openTasks}><CalendarClock /></Button>}
      {workspace.collapsed && <SemanticButton icon={Plus} meaning="add" label="新对话" aria-keyshortcuts={newConversationShortcut}
        disabled={send.isPending} onClick={newConversation} />}
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
      {workspace && conversationId && messages.isPending && <div className="chat-message space-y-4 py-2" aria-busy="true">
        <Skeleton className="ml-auto h-10 w-2/5 rounded-2xl" />
        <div className="space-y-2">
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-4 w-11/12" />
          <Skeleton className="h-4 w-2/3" />
        </div>
      </div>}
      {messages.isError && (workspace
        ? <div className="chat-message flex items-center justify-between gap-3 rounded-xl border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm">
            <span className="text-destructive">暂时无法读取消息。</span>
            <Button size="sm" variant="outline" onClick={() => void messages.refetch()}>重新读取消息</Button>
          </div>
        : <Button size="sm" variant="ghost" onClick={() => void messages.refetch()}>重新读取消息</Button>)}
      {isEmptyWorkspace && <div className="chat-empty-state">
        <AppMark className="size-14" />
        <p className="text-lg font-medium text-foreground">{greeting}</p>
      </div>}
      {messages.data?.map(message => <section key={message.id} data-message-id={message.id} className={workspace ? 'chat-message group/message' : message.role === 'user' ? 'ml-auto w-fit max-w-[90%] rounded-xl bg-muted p-3' : 'mr-3 rounded-xl border p-3'} data-role={workspace ? message.role : undefined}>
        {!workspace && message.role === 'user' && <p className="text-xs text-muted-foreground">你</p>}
        {message.role === 'assistant'
          ? <ReplyBody message={message} markdown={Boolean(workspace)}
            reveal={workspace?.motionMode === 'off' ? undefined : message.reveal} />
          : (message.text || (message.attachments?.length ?? 0) > 0) && (workspace
            ? <div className="chat-user-stack">
                {message.attachments && message.attachments.length > 0 && <div className="chat-user-attachments">
                  {message.attachments.map((att, idx) => att.kind === 'image' && att.preview
                    ? <img key={`${att.name}-${idx}`} src={att.preview} alt={att.name} className="chat-user-image" />
                    : <span key={`${att.path ?? att.name}-${idx}`} className="chat-chip max-w-64 pr-2" title={att.path || att.name}>
                        {att.kind === 'folder' ? <Folder className="size-3 shrink-0 text-muted-foreground" aria-hidden="true" />
                          : att.kind === 'image' ? <ImageIcon className="size-3 shrink-0 text-muted-foreground" aria-hidden="true" />
                            : <FileIcon className="size-3 shrink-0 text-muted-foreground" aria-hidden="true" />}
                        <span className="truncate">{att.name}</span>
                      </span>)}
                </div>}
                {message.text && <p className="whitespace-pre-wrap break-words">{message.text}</p>}
                <div className="chat-user-meta">
                  {formatMessageTime(message.createdAt) && <time className="text-xs tabular-nums text-muted-foreground/80" title={formatMessageTimeTitle(message.createdAt)}>
                    {formatMessageTime(message.createdAt)}
                  </time>}
                  {message.text && <UserCopyButton text={message.text} />}
                </div>
              </div>
            : <p className="whitespace-pre-wrap break-words">{message.text}</p>)}
        {statusLabels[message.status] && <p className={cn('mt-1 text-sm', message.status === 'failed' ? 'text-destructive' : 'text-muted-foreground')}>
          {statusLabels[message.status]}{errorLabels[message.errorCode] && ` · ${errorLabels[message.errorCode]}`}
          {message.status === 'failed' && Boolean(message.details?.work?.retryCount) && ` · 已自动重试 ${message.details?.work?.retryCount} 次`}
        </p>}
        {message.role === 'assistant' && message.status !== 'running' && <ReplyActions message={message} branchDisabled={branch.isPending || send.isPending}
          onBranch={() => branch.mutate({ id: conversationId, messageId: message.id })} />}
      </section>)}
    </div>
    {workspace && !isEmptyWorkspace && <ChatTimelineRail messages={messages.data} viewport={timeline} />}
    {workspace && !following && <Button className="chat-latest" variant="outline" size="icon" aria-label="回到最新" onClick={() => {
      timeline.current?.focus({ preventScroll: true })
      follow.current = true
      setFollowing(true)
    }}><ArrowDown /></Button>}
    <div className={workspace ? 'chat-composer' : 'flex flex-col gap-3'}>
      {target.kind !== 'global' && <label htmlFor="chat-include-context" className="flex items-center gap-2 text-sm">
        <Checkbox id="chat-include-context" checked={includeContext} disabled={send.isPending} onCheckedChange={value => setIncludeContext(value === true)} />
        {target.kind === 'article' ? '附加订阅正文（最多 24 KiB）' : '附加当前分类最近 5 篇文章的摘要'}
      </label>}
      {target.kind === 'article' &&
        <label htmlFor="chat-allow-article-reads" className="flex items-center gap-2 text-sm">
          <Checkbox id="chat-allow-article-reads" checked={allowArticleReads} disabled={send.isPending}
            onCheckedChange={value => setAllowArticleReads(value === true)} />
          允许模型读取当前文章正文
        </label>}
      {selection && <div className="flex items-center gap-2 text-sm">
        <span className="truncate" title={selection}>选段 · {selection}</span>
        <Button size="sm" variant="ghost" disabled={send.isPending} onClick={() => setSelection('')}>移除</Button>
      </div>}
      {workspace && currentQueue.length > 0 && <ChatQueueBar
        items={currentQueue}
        busySteer={!active || steer.isPending}
        onSteer={item => {
          if (item.attachments.some(att => att.kind === 'image') && !modelSupportsImage) {
            void notify('当前模型不是多模态模型，不支持图片', '请切换模型或移除图片后再引导。', true)
            return
          }
          setQueues(current => ({ ...current, [conversationId]: (current[conversationId] ?? []).filter(entry => entry.id !== item.id) }))
          steer.mutate({ text: item.text, attachments: item.attachments })
        }}
        onEdit={(itemId, nextText) => {
          setQueues(current => ({
            ...current,
            [conversationId]: (current[conversationId] ?? []).map(entry => entry.id === itemId ? { ...entry, text: nextText } : entry),
          }))
        }}
        onDelete={itemId => {
          setQueues(current => ({
            ...current,
            [conversationId]: (current[conversationId] ?? []).filter(entry => entry.id !== itemId),
          }))
        }}
      />}
      {inlinePlan ? <section aria-label="计划确认" className={cn('flex flex-col gap-3 px-4 py-3', workspace ? 'chat-composer-form chat-composer-approval' : 'rounded-xl border')}>
        <PlanConfirmPrompt key={inlinePlan.id} pending={inlinePlan}
          onDecision={decision => { if (decision === 'approve' || decision === 'reject') setPlanMode(false) }}
          onStop={() => cancel.mutate(conversationId)} stopping={cancel.isPending} />
      </section> : inlineQuestion ? <section aria-label="AI 提问" className={cn('flex flex-col gap-3 px-4 py-3', workspace ? 'chat-composer-form chat-composer-approval' : 'rounded-xl border')}>
        <UserQuestionPrompt key={inlineQuestion.id} pending={inlineQuestion}
          onStop={() => cancel.mutate(conversationId)} stopping={cancel.isPending} />
      </section> : inlineApproval ? <section aria-label="工具审批" className={cn('flex flex-col gap-3 px-4 py-3', workspace ? 'chat-composer-form chat-composer-approval' : 'rounded-xl border')}>
        <ToolApproval key={inlineApproval.id} action={inlineApproval} inline
          onStop={() => cancel.mutate(conversationId)} stopping={cancel.isPending} />
      </section> : <form className={workspace ? 'chat-composer-form' : 'flex items-end gap-2'} onSubmit={event => {
        event.preventDefault()
        submitComposer()
      }}>
        {slashOpen && <ChatSlashMenu composer={composer} label={slashLabel} active={slashActive} items={slashItems}
          onHighlight={setHighlightedSlash} onChoose={chooseSlash}
          onBack={slashPage !== 'actions' ? () => { setSlashPage('actions'); setHighlightedSlash(''); setDraft('/') } : undefined} />}
        {hasComposerChips && <div className="chat-composer-chips">
          {planMode && <div className="chat-chip border-primary/40 bg-primary/10 text-primary">
            <ClipboardList className="size-3 shrink-0" aria-hidden="true" />
            <span>计划模式</span>
            <Button type="button" size="icon-xs" variant="ghost" className="size-5 rounded-full" aria-label="关闭计划模式" disabled={send.isPending} onClick={() => setPlanMode(false)}><X /></Button>
          </div>}
          {selectedSkills.map(name => <div key={name} className="chat-chip">
            <Sparkles className="size-3 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className="truncate">{name}</span>
            <Button type="button" size="icon-xs" variant="ghost" className="size-5 rounded-full" aria-label={`移除技能 ${name}`} disabled={send.isPending} onClick={() => toggleSkill(name)}><X /></Button>
          </div>)}
          {attachments.map(item => <div key={item.id} className={cn('chat-chip max-w-56', item.kind === 'image' && !modelSupportsImage && 'border-destructive/60 text-destructive')}>
            {item.kind === 'image' && item.preview
              ? <img src={item.preview} alt="" className="chat-attachment-thumb" />
              : item.kind === 'folder'
                ? <Folder className="size-3 shrink-0 text-muted-foreground" aria-hidden="true" />
                : item.kind === 'image'
                  ? <ImageIcon className="size-3 shrink-0 text-muted-foreground" aria-hidden="true" />
                  : <FileIcon className="size-3 shrink-0 text-muted-foreground" aria-hidden="true" />}
            <span className="truncate" title={item.path || item.name}>{item.name}</span>
            <Button type="button" size="icon-xs" variant="ghost" className="size-5 rounded-full" aria-label={`移除 ${item.name}`} disabled={send.isPending}
              onClick={() => setAttachments(current => current.filter(entry => entry.id !== item.id))}><X /></Button>
          </div>)}
        </div>}
        <Textarea ref={composer} className={workspace ? 'chat-input' : 'max-h-40 resize-y'} aria-label="消息"
          placeholder={active && workspace ? (busySendMode === 'steer' ? '输入引导消息，AI 将在下一轮次接收…' : '输入消息加入排队，当前完成后自动发送…') : '输入消息…（输入 / 打开命令，支持粘贴文件/文件夹/图片）'}
          value={draft}
          onPaste={workspace ? event => void handleComposerPaste(event) : undefined}
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
              } else {
                submitComposer()
              }
            }
          } : undefined} />
        <div className={workspace ? 'chat-composer-toolbar' : undefined}>
          {workspace && <div className="flex min-w-0 items-center gap-1">
            <DropdownMenu>
              <Tooltip>
                <TooltipTrigger asChild>
                  <DropdownMenuTrigger asChild>
                    <Button size="icon-sm" type="button" variant="ghost" aria-label="添加文件或图片" disabled={send.isPending}><Plus /></Button>
                  </DropdownMenuTrigger>
                </TooltipTrigger>
                <TooltipContent>添加文件、文件夹或图片</TooltipContent>
              </Tooltip>
              <DropdownMenuContent align="start" side="top">
                <DropdownMenuItem onSelect={() => void pickAttachments('files')}>
                  <FileIcon className="size-4" />添加文件或图片
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => void pickAttachments('folders')}>
                  <Folder className="size-4" />添加文件夹
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            <SearchableSelect label="审批方式" className="chat-toolbar-select" value={approvalMode} options={approvalModes} variant="ghost"
              announceSelection disabled={active || send.isPending}
              onValueChange={value => setApprovalChoices(current => ({ ...current, [conversationId]: value }))} />
          </div>}
          <div className={workspace ? 'chat-composer-actions' : undefined}>
            {workspace && draft.length >= 2400 && <span className={cn('px-1 text-xs tabular-nums', draft.length >= 2700 ? 'text-destructive' : 'text-muted-foreground')}>
              {draft.length.toLocaleString()} / 2,700
            </span>}
            {workspace && selectedConfig && modelID && <ChatContext id={conversationId} modelId={modelID} modelRevision={selectedConfig.revision}
              disabled={active || send.isPending} attachments={[]} open={contextOpen} onOpenChange={setContextOpen} />}
            {workspace && (modelOptions.length ? <ChatModelMenu
              modelOptions={modelOptions}
              selectedModel={selectedModel}
              effortOptions={effortLevels}
              selectedEffort={reasoningEffort}
              selectedEffortLabel={selectedEffortLabel}
              defaultEffortValue={defaultEffortOption.value}
              disabled={send.isPending}
              onSelectModel={selectModel}
              onSelectEffort={effort => {
                if (modelID) setEffortChoices(current => ({ ...current, [modelID]: effort === defaultEffortOption.value ? '' : effort }))
              }}
            />
              : configuration.isError ? <Button type="button" size="sm" variant="ghost" onClick={() => void configuration.refetch()}>重新读取模型</Button>
                : configuration.isPending ? <span role="status" className="px-2 text-xs text-muted-foreground">正在读取模型…</span>
                  : <Button type="button" size="sm" variant="ghost" onClick={() => showSettings('ai')}>配置模型</Button>)}
            {workspace ? (active
              ? <>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button type="submit" size="icon-sm" variant="secondary" className="rounded-full"
                        aria-label={busySendMode === 'steer' ? '发送引导' : '加入排队'} disabled={!canBusySend}>
                        {busySendMode === 'steer' ? <CornerDownRight /> : <ArrowUp />}
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>{busySendMode === 'steer' ? '发送引导（下一轮次接收）' : '加入排队（本次完成后发送）'}</TooltipContent>
                  </Tooltip>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button type="button" size="icon-sm" className="rounded-full" aria-label="停止回复" disabled={cancel.isPending} onClick={() => cancel.mutate(conversationId)}><Square className="size-2.5" fill="currentColor" /></Button>
                    </TooltipTrigger>
                    <TooltipContent>停止回复</TooltipContent>
                  </Tooltip>
                </>
              : <Tooltip>
                  <TooltipTrigger asChild>
                    <Button type="submit" size="icon-sm" className="rounded-full" aria-label="发送" disabled={!canSend}><ArrowUp /></Button>
                  </TooltipTrigger>
                  <TooltipContent>发送</TooltipContent>
                </Tooltip>)
              : active ? <Button type="button" variant="outline" disabled={cancel.isPending} onClick={() => cancel.mutate(conversationId)}>停止回复</Button>
                : <Button type="submit" disabled={!canSend}>发送</Button>}
          </div>
        </div>
      </form>}
      {workspace && !conversationId && draftGroup && <div className="chat-group-context" role="group" aria-label="新对话分组">
        {selectedGroup && <GroupIcon group={selectedGroup} />}
        <span className="min-w-0 flex-1 truncate" title={selectedGroup?.name}>{selectedGroup?.name ?? '分组已不可用'}</span>
        <Button type="button" variant="ghost" size="icon-xs" aria-label="取消新对话的分组" disabled={send.isPending}
          onClick={() => { setDraftGroup(''); composer.current?.focus({ preventScroll: true }) }}><X /></Button>
      </div>}
      {workspace && imageBlocked && <p role="alert">{errorLabels.ModelImageUnsupported}</p>}
      {workspace && invalidEffort && !send.isError && <p role="alert">{errorLabels.ChatReasoningUnavailable}</p>}
      {(send.isError || cancel.isError) && <p role="alert">{errorText(send.error ?? cancel.error)}</p>}
    </div>
    </div>
  </div>
}

function UserCopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  useEffect(() => {
    if (!copied) return
    const timer = window.setTimeout(() => setCopied(false), 1600)
    return () => window.clearTimeout(timer)
  }, [copied])
  async function copy() {
    try {
      await native.copy(text)
      setCopied(true)
      void notify('已复制消息')
    } catch {
      void notify('无法复制消息', undefined, true)
    }
  }
  return <Tooltip>
    <TooltipTrigger asChild>
      <Button type="button" variant="ghost" size="icon-xs" className="chat-user-copy" aria-label="复制消息" onClick={() => void copy()}>
        {copied ? <Check /> : <Copy />}
      </Button>
    </TooltipTrigger>
    <TooltipContent>{copied ? '已复制' : '复制'}</TooltipContent>
  </Tooltip>
}
