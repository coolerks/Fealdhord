import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { HoverCard, HoverCardContent, HoverCardTrigger } from '@/components/ui/hover-card'
import { Input } from '@/components/ui/input'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { Switch } from '@/components/ui/switch'
import { aiConfigurationKey as queryKey } from '@/lib/ai-configuration'
import { notify } from '@/lib/notifications'
import { invalidateRSS } from '@/lib/refresh'
import { ModelCatalogField } from './ModelCatalogField'
import { ModelBrand, ModelCapabilities, ModelDetails } from './ModelMetadata'
import { ConfirmDialog, SettingCard, SettingEmpty, SettingRow, SettingSection, StatusBadge } from './settings-primitives'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/aiconfig/api'
import type { Model, Provider } from '../../../bindings/github.com/coolerks/Fealdhord/internal/aiconfig/models'

const protocols = [
  ['chat_completions', 'Chat Completions'],
  ['responses', 'Responses'],
  ['messages', 'Messages'],
] as const

const protocolLabels = Object.fromEntries(protocols)

const tasks = [
  ['chat', '聊天'], ['classification', '分类'], ['summary', '摘要'],
  ['translation', '翻译'], ['featured', '精选'],
] as const

const newProvider: Provider = { id: '', revision: 0, name: '', protocol: 'chat_completions', baseUrl: '', enabled: true, hasCredential: false }
const defaultReasoningLevels = [
  { label: '低', value: 'low' }, { label: '中', value: 'medium' },
  { label: '高', value: 'high' }, { label: '超高', value: 'xhigh' },
]
const maxReasoningLevels = 16
const commonBrands = ['anthropic', 'openai', 'google', 'deepseek', 'openrouter', 'nvidia', 'meta', 'mistral', 'alibaba', 'moonshotai', 'xai', 'zai']

function configError(error: unknown) {
  const message = error instanceof Error ? error.message : ''
  if (message.includes('CredentialRequired')) return '更换服务地址或协议时，请重新输入密钥或清除旧密钥。'
  if (message.includes('CredentialError')) return '系统凭据存储不可用，无法保存密钥。'
  if (message.includes('AuthenticationError')) return '服务拒绝了当前密钥。'
  if (message.includes('RateLimitError')) return '服务限流，请稍后再试。'
  if (message.includes('Timeout')) return '连接测试超时。'
  if (message.includes('Provider')) return '服务请求失败，请检查地址和模型 ID。'
  if (message.includes('Conflict')) return '配置已变更或模型已存在，请刷新后重试。'
  if (message.includes('ConfigurationError')) return '请选择已启用的服务和模型。'
  if (message.includes('InvalidReasoningLevels')) return '请检查思考档位名称和参数值。参数值不能重复，仅支持字母、数字、连字符和下划线。'
  if (message.includes('ValidationError')) return '请检查名称、地址和密钥格式。'
  return '无法保存 AI 设置，请重试。'
}

export function AISettings() {
  const cache = useQueryClient()
  const configuration = useQuery({ queryKey, queryFn: () => api.List() })
  const mutation = useMutation({
    mutationFn: async (save: () => Promise<unknown>) => { await save() },
    onSettled: async () => { await cache.invalidateQueries({ queryKey }) },
    onSuccess: () => invalidateRSS(cache),
  })
  const [provider, setProvider] = useState<Provider | null>(null)
  const [model, setModel] = useState<Model | null>(null)
  const [key, setKey] = useState('')
  const [clearKey, setClearKey] = useState(false)
  const [deletingProvider, setDeletingProvider] = useState<Provider | null>(null)
  const [deletingModel, setDeletingModel] = useState<Model | null>(null)
  const [savingProvider, setSavingProvider] = useState(false)
  const [providerError, setProviderError] = useState('')
  const [testingModel, setTestingModel] = useState<Model | null>(null)
  const [testRunning, setTestRunning] = useState(false)
  const testRequest = useRef<ReturnType<typeof api.TestConnection> | null>(null)
  const editorTrigger = useRef<HTMLButtonElement | null>(null)
  const pendingEditorFocus = useRef<HTMLButtonElement | null>(null)
  const busy = mutation.isPending || savingProvider || testRunning

  useEffect(() => {
    if (!busy && pendingEditorFocus.current) {
      pendingEditorFocus.current.focus()
      pendingEditorFocus.current = null
    }
  }, [busy])

  useEffect(() => () => {
    const pending = testRequest.current
    testRequest.current = null
    if (pending) void pending.cancel()
  }, [])

  async function runTest(id: string) {
    const pending = api.TestConnection(id)
    testRequest.current = pending
    setTestRunning(true)
    try {
      await pending
      if (testRequest.current === pending) void notify('连接成功')
    } catch (error) {
      if (testRequest.current === pending) void notify('连接测试失败', configError(error), true)
    } finally {
      if (testRequest.current === pending) {
        testRequest.current = null
        setTestingModel(null)
        setTestRunning(false)
      }
    }
  }

  function closeTestDialog() {
    const pending = testRequest.current
    testRequest.current = null
    if (pending) void pending.cancel()
    setTestingModel(null)
    setTestRunning(false)
  }

  function editProvider(next: Provider | null) {
    setProviderError('')
    setProvider(next)
    setKey('')
    setClearKey(false)
  }

  function openNewModel(providerId: string, trigger: HTMLButtonElement) {
    editorTrigger.current = trigger
    mutation.reset()
    setModel({ id: '', revision: 0, providerId, name: '', displayName: '', enabled: true, reasoningLevels: defaultReasoningLevels })
  }

  const providers = configuration.data?.providers ?? []
  const models = configuration.data?.models ?? []
  const brands = useQuery({ queryKey: ['ai-model-brands'], queryFn: () => api.Brands(), enabled: Boolean(model), retry: false, staleTime: 60 * 60 * 1000 })
  const modelProvider = providers.find(item => item.id === model?.providerId)
  const reasoningLevels = model?.reasoningLevels ?? []
  const activeModels = models.filter(item => item.enabled && providers.some(service => service.id === item.providerId && service.enabled))

  if (configuration.isPending) return <p role="status" className="text-[13px] text-muted-foreground">正在读取 AI 设置…</p>
  if (configuration.isError) return (
    <SettingCard divided={false}>
      <SettingEmpty title="无法读取 AI 设置" action={<Button size="sm" variant="outline" onClick={() => void configuration.refetch()}>重新读取 AI 设置</Button>} />
    </SettingCard>
  )

  function restoreEditorFocus(event: Event) {
    event.preventDefault()
    const trigger = editorTrigger.current
    editorTrigger.current = null
    if (!trigger?.isConnected) return
    if (trigger.disabled) pendingEditorFocus.current = trigger
    else trigger.focus()
  }
  const groupedModels = providers.map(service => ({
    provider: service,
    items: models.filter(item => item.providerId === service.id),
  })).filter(group => group.items.length > 0)
  const orphanModels = models.filter(item => !providers.some(service => service.id === item.providerId))

  return <div className="space-y-6">
    <SettingSection
      title="AI 服务"
      actions={<Button size="sm" variant="outline" disabled={busy} onClick={event => { editorTrigger.current = event.currentTarget; editProvider(newProvider) }}><AnimatedGlyph meaning="plus" />添加服务</Button>}
    >
      <SettingCard>
        {providers.length === 0 ? (
          <SettingEmpty icon={<AnimatedGlyph meaning="server" />} title="还没有配置 AI 服务" />
        ) : providers.map(item => (
          <SettingRow key={item.id}>
            <div className="min-w-0 flex-1 space-y-0.5">
              <div className="flex flex-wrap items-center gap-2">
                <span className="truncate font-medium">{item.name}</span>
                <StatusBadge tone="muted">{protocolLabels[item.protocol] ?? item.protocol}</StatusBadge>
                {!item.enabled && <StatusBadge tone="muted">已停用</StatusBadge>}
              </div>
              {item.baseUrl && <p className="truncate text-xs text-muted-foreground">{item.baseUrl}</p>}
            </div>
            <div className="flex shrink-0 flex-wrap items-center gap-1">
              <Button size="sm" variant="ghost" disabled={busy} onClick={event => openNewModel(item.id, event.currentTarget)}>添加模型</Button>
              <Button size="sm" variant="ghost" disabled={busy} onClick={event => { editorTrigger.current = event.currentTarget; editProvider(item) }}>编辑</Button>
              <Button size="sm" variant="ghost" disabled={busy} onClick={() => { mutation.reset(); setDeletingProvider(item) }}>删除</Button>
            </div>
          </SettingRow>
        ))}
      </SettingCard>
    </SettingSection>

    <SettingSection title="模型">
      {models.length === 0 ? (
        <SettingCard divided={false}>
          <SettingEmpty icon={<AnimatedGlyph meaning="box" />} title={providers.length ? '还没有添加模型' : '先添加 AI 服务，再添加模型'} />
        </SettingCard>
      ) : (
        <div className="space-y-3">
          {groupedModels.map(({ provider: service, items }) => (
            <SettingCard key={service.id}>
              <div className="flex items-center justify-between gap-2 bg-muted/35 px-3.5 py-2">
                <div className="flex min-w-0 items-center gap-2">
                  <span className="truncate text-xs font-medium text-muted-foreground">{service.name}</span>
                  {!service.enabled && <StatusBadge tone="muted">服务已停用</StatusBadge>}
                </div>
                <Button size="xs" variant="ghost" disabled={busy} onClick={event => openNewModel(service.id, event.currentTarget)}>
                  <AnimatedGlyph meaning="plus" />添加模型
                </Button>
              </div>
              <ul className="divide-y divide-border">
                {items.map(item => renderModelRow(item, service.name))}
              </ul>
            </SettingCard>
          ))}
          {orphanModels.length > 0 && (
            <SettingCard>
              <ul className="divide-y divide-border">
                {orphanModels.map(item => renderModelRow(item, ''))}
              </ul>
            </SettingCard>
          )}
        </div>
      )}
    </SettingSection>

    <SettingSection title="用途分配">
      <SettingCard>
        {tasks.map(([task, label]) => {
          const selected = configuration.data.assignments?.find(item => item.task === task)?.modelId ?? ''
          const inactive = selected && !activeModels.some(item => item.id === selected)
          return (
            <SettingRow key={task}>
              <FieldLabel htmlFor={`ai-task-${task}`} className="font-normal">{label}</FieldLabel>
              <SearchableSelect
                id={`ai-task-${task}`}
                label={label}
                className="min-w-48 max-w-[65%]"
                disabled={busy}
                value={selected || 'unassigned'}
                onValueChange={value => {
                  mutation.mutate(() => api.Assign(task, value === 'unassigned' ? '' : value))
                }}
                options={[
                  { value: 'unassigned', label: '未分配' },
                  ...(inactive ? [{ value: selected, label: `${models.find(item => item.id === selected)?.displayName ?? '模型'}（已停用）`, disabled: true }] : []),
                  ...activeModels.map(item => ({
                    value: item.id,
                    label: `${item.displayName} · ${providers.find(service => service.id === item.providerId)?.name ?? ''}`,
                    keywords: [item.name],
                    graphic: <ModelBrand brand={item.metadata?.brand} override={item.metadata?.brandOverride} />,
                  })),
                ]}
              />
            </SettingRow>
          )
        })}
      </SettingCard>
    </SettingSection>

    {mutation.isError && !model && !deletingProvider && !deletingModel && (
      <FieldError>{configError(mutation.error)}</FieldError>
    )}

    <Dialog open={Boolean(provider)} onOpenChange={open => { if (!open && !savingProvider) editProvider(null) }}>
      {provider && (
        <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto sm:max-w-md" onCloseAutoFocus={restoreEditorFocus}>
          <DialogHeader>
            <DialogTitle>{provider.id ? '编辑服务' : '添加服务'}</DialogTitle>
          </DialogHeader>
          <form onSubmit={event => {
            event.preventDefault()
            if (busy) return
            setSavingProvider(true)
            setProviderError('')
            void api.SaveProvider({ ...provider, apiKey: clearKey ? '' : key || null }).then(async () => {
              editProvider(null)
              invalidateRSS(cache)
              await cache.invalidateQueries({ queryKey })
            }).catch(error => setProviderError(configError(error))).finally(() => setSavingProvider(false))
          }}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="ai-provider-name">服务名称</FieldLabel>
                <Input id="ai-provider-name" required value={provider.name} onChange={event => setProvider({ ...provider, name: event.target.value })} />
              </Field>
              <Field>
                <FieldLabel htmlFor="ai-protocol">API 类型</FieldLabel>
                <SearchableSelect id="ai-protocol" label="API 类型" className="w-full" value={provider.protocol} disabled={busy}
                  onValueChange={protocol => setProvider({ ...provider, protocol })}
                  options={protocols.map(([value, label]) => ({ value, label }))} />
              </Field>
              <Field>
                <FieldLabel htmlFor="ai-base-url">Base URL</FieldLabel>
                <Input id="ai-base-url" type="url" required placeholder="https://api.example.com/v1" value={provider.baseUrl}
                  onChange={event => setProvider({ ...provider, baseUrl: event.target.value })} />
              </Field>
              <Field>
                <FieldLabel htmlFor="ai-key">API 密钥</FieldLabel>
                <Input id="ai-key" type="password" autoComplete="off" disabled={clearKey}
                  placeholder={provider.hasCredential ? '已保存' : undefined}
                  value={key} onChange={event => setKey(event.target.value)} />
              </Field>
              {provider.hasCredential && (
                <Field orientation="horizontal">
                  <Checkbox id="ai-clear-key" checked={clearKey} disabled={busy} onCheckedChange={value => setClearKey(value === true)} />
                  <FieldLabel htmlFor="ai-clear-key" className="font-normal">清除已保存的密钥</FieldLabel>
                </Field>
              )}
              <Field orientation="horizontal">
                <FieldLabel htmlFor="ai-provider-enabled" className="font-normal">启用服务</FieldLabel>
                <Switch id="ai-provider-enabled" checked={provider.enabled} disabled={busy}
                  onCheckedChange={enabled => setProvider({ ...provider, enabled })} />
              </Field>
              {providerError && <FieldError>{providerError}</FieldError>}
              <DialogFooter>
                <Button size="sm" type="button" variant="outline" disabled={busy} onClick={() => editProvider(null)}>取消</Button>
                <Button size="sm" type="submit" disabled={busy}>保存服务</Button>
              </DialogFooter>
            </FieldGroup>
          </form>
        </DialogContent>
      )}
    </Dialog>

    <Dialog open={Boolean(model)} onOpenChange={open => { if (!open && !busy) { setModel(null); mutation.reset() } }}>
      {model && (
        <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto sm:max-w-lg" onCloseAutoFocus={restoreEditorFocus}>
          <DialogHeader>
            <DialogTitle>{model.id ? '编辑模型' : '添加模型'}</DialogTitle>
          </DialogHeader>
          <form onSubmit={event => {
            event.preventDefault()
            if (busy) return
            mutation.mutate(async () => {
              await api.SaveModel(model)
              setModel(null)
            })
          }}>
            <FieldGroup>
              {modelProvider && (
                <ModelCatalogField
                  provider={modelProvider}
                  value={model.name}
                  disabled={busy}
                  excluded={models.filter(item => item.providerId === model.providerId && item.id !== model.id).map(item => item.name)}
                  onSelect={(name, displayName, metadata) => setModel({ ...model, name, displayName, metadata })}
                />
              )}
              <Field>
                <FieldLabel htmlFor="ai-model-id">模型 ID</FieldLabel>
                <Input id="ai-model-id" required value={model.name} onChange={event => setModel({ ...model, name: event.target.value, metadata: undefined })} />
              </Field>
              <Field>
                <FieldLabel htmlFor="ai-model-display">显示名称</FieldLabel>
                <Input id="ai-model-display" value={model.displayName} onChange={event => setModel({ ...model, displayName: event.target.value })} />
              </Field>
              <div className="grid gap-3 sm:grid-cols-2">
                {(['context', 'outputLimit'] as const).map((field, index) => (
                  <Field key={field}>
                    <FieldLabel htmlFor={`ai-model-${field}`}>{index === 0 ? '上下文大小（令牌）' : '最大输出（令牌）'}</FieldLabel>
                    <Input
                      id={`ai-model-${field}`}
                      type="number"
                      min={1}
                      max={10_000_000}
                      placeholder="自动获取"
                      value={model.metadata?.[field] ?? ''}
                      onChange={event => setModel({
                        ...model,
                        metadata: {
                          ...model.metadata,
                          [field]: event.target.value ? Number(event.target.value) : undefined,
                          source: '手动',
                        },
                      })}
                    />
                  </Field>
                ))}
              </div>
              <Field>
                <FieldLabel htmlFor="ai-model-brand">模型图标</FieldLabel>
                <SearchableSelect
                  id="ai-model-brand"
                  label="模型图标"
                  className="w-full"
                  value={model.metadata?.brandOverride || 'auto'}
                  options={[
                    { value: 'auto', label: '自动匹配' },
                    { value: 'none', label: '通用图标' },
                    ...Array.from(new Set([...commonBrands, ...(brands.data ?? []), ...(model.metadata?.brand ? [model.metadata.brand] : [])])).map(brand => ({
                      value: brand,
                      label: brand,
                      graphic: <ModelBrand brand={brand} />,
                    })),
                  ]}
                  onValueChange={brand => setModel({ ...model, metadata: { ...model.metadata, brandOverride: brand === 'auto' ? undefined : brand } })}
                />
              </Field>
              {model.metadata && (
                <div className="rounded-lg border border-border bg-muted/35 p-3">
                  <ModelDetails metadata={model.metadata} />
                </div>
              )}
              <fieldset className="space-y-2.5 rounded-lg border border-border p-3" disabled={busy}>
                <legend className="px-1 text-xs font-medium text-muted-foreground">思考档位</legend>
                {reasoningLevels.map((level, index) => (
                  <div key={index} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] items-end gap-2">
                    <Field>
                      <FieldLabel htmlFor={`ai-effort-label-${index}`}>显示名称</FieldLabel>
                      <Input id={`ai-effort-label-${index}`} required maxLength={32} value={level.label}
                        onChange={event => setModel({ ...model, reasoningLevels: reasoningLevels.map((entry, i) => i === index ? { ...entry, label: event.target.value } : entry) })} />
                    </Field>
                    <Field>
                      <FieldLabel htmlFor={`ai-effort-value-${index}`}>参数值</FieldLabel>
                      <Input id={`ai-effort-value-${index}`} required maxLength={64} value={level.value}
                        onChange={event => setModel({ ...model, reasoningLevels: reasoningLevels.map((entry, i) => i === index ? { ...entry, value: event.target.value } : entry) })} />
                    </Field>
                    <Button type="button" size="icon" variant="ghost" aria-label={`删除档位 ${level.label || index + 1}`}
                      onClick={() => setModel({ ...model, reasoningLevels: reasoningLevels.filter((_, i) => i !== index) })}>
                      <AnimatedGlyph meaning="delete" />
                    </Button>
                  </div>
                ))}
                <Button type="button" size="sm" variant="outline" disabled={reasoningLevels.length >= maxReasoningLevels}
                  onClick={() => setModel({ ...model, reasoningLevels: [...reasoningLevels, { label: '', value: '' }] })}>
                  <AnimatedGlyph meaning="plus" />添加档位
                </Button>
              </fieldset>
              <Field orientation="horizontal">
                <FieldLabel htmlFor="ai-model-vision" className="font-normal">支持图片输入（多模态）</FieldLabel>
                <Switch id="ai-model-vision" checked={Boolean(model.metadata?.modalities?.includes('image'))} disabled={busy}
                  onCheckedChange={checked => {
                    const current = model.metadata?.modalities ?? ['text']
                    const next = checked
                      ? Array.from(new Set([...current, 'text', 'image']))
                      : current.filter(item => item !== 'image')
                    setModel({
                      ...model,
                      metadata: { ...model.metadata, modalities: next, source: model.metadata?.source || '手动' },
                    })
                  }} />
              </Field>
              <Field orientation="horizontal">
                <FieldLabel htmlFor="ai-model-enabled" className="font-normal">启用模型</FieldLabel>
                <Switch id="ai-model-enabled" checked={model.enabled} disabled={busy} onCheckedChange={enabled => setModel({ ...model, enabled })} />
              </Field>
              {mutation.isError && <FieldError>{configError(mutation.error)}</FieldError>}
              <DialogFooter>
                <Button size="sm" type="button" variant="outline" disabled={busy} onClick={() => { setModel(null); mutation.reset() }}>取消</Button>
                <Button size="sm" type="submit" disabled={busy}>保存模型</Button>
              </DialogFooter>
            </FieldGroup>
          </form>
        </DialogContent>
      )}
    </Dialog>

    <ConfirmDialog
      open={Boolean(deletingProvider)}
      title={deletingProvider ? `删除服务“${deletingProvider.name}”？` : ''}
      description="同时删除该服务的模型和用途分配。"
      confirmLabel="确认删除"
      busy={busy}
      error={mutation.isError && deletingProvider ? configError(mutation.error) : undefined}
      onOpenChange={open => { if (!open) { setDeletingProvider(null); mutation.reset() } }}
      onConfirm={() => {
        if (!deletingProvider) return
        mutation.mutate(async () => {
          await api.DeleteProvider(deletingProvider.id, deletingProvider.revision)
          setDeletingProvider(null)
          editProvider(null)
          setModel(null)
        })
      }}
    />

    <ConfirmDialog
      open={Boolean(deletingModel)}
      title={deletingModel ? `删除模型“${deletingModel.displayName}”？` : ''}
      description="同时清除该模型的用途分配。"
      confirmLabel="确认删除"
      busy={busy}
      error={mutation.isError && deletingModel ? configError(mutation.error) : undefined}
      onOpenChange={open => { if (!open) { setDeletingModel(null); mutation.reset() } }}
      onConfirm={() => {
        if (!deletingModel) return
        mutation.mutate(async () => {
          await api.DeleteModel(deletingModel.id, deletingModel.revision)
          setDeletingModel(null)
          setModel(null)
        })
      }}
    />

    <ConfirmDialog
      open={Boolean(testingModel)}
      title={testingModel ? `测试“${testingModel.displayName}”连接？` : ''}
      description="将向服务发送一条测试消息，可能产生费用。"
      confirmLabel={testRunning ? '正在测试…' : '开始测试'}
      destructive={false}
      busy={testRunning}
      onOpenChange={open => { if (!open) closeTestDialog() }}
      onConfirm={() => { if (testingModel) void runTest(testingModel.id) }}
    />
  </div>

  function renderModelRow(item: Model, providerName: string) {
    return (
      <li key={item.id} className="px-3.5 py-2.5">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
          <HoverCard openDelay={250}>
            <HoverCardTrigger asChild>
              <div className="flex min-w-0 flex-[1_1_12rem] items-start gap-2.5">
                <span className="mt-0.5 shrink-0">
                  <ModelBrand brand={item.metadata?.brand} override={item.metadata?.brandOverride} />
                </span>
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className="truncate font-medium">{item.displayName}</span>
                    {!item.enabled && <StatusBadge tone="muted">已停用</StatusBadge>}
                  </div>
                  <div className="mt-0.5 flex flex-wrap items-center gap-x-2.5 gap-y-1 text-xs text-muted-foreground">
                    {providerName && <span>{providerName}</span>}
                    <ModelCapabilities metadata={item.metadata} />
                  </div>
                </div>
              </div>
            </HoverCardTrigger>
            <HoverCardContent align="start" side="top">
              <div className="flex flex-col gap-2">
                <p className="wrap-anywhere font-medium">{item.displayName}</p>
                <ModelDetails metadata={item.metadata} />
              </div>
            </HoverCardContent>
          </HoverCard>
          <div className="ml-auto flex shrink-0 items-center gap-1">
            <Button size="sm" variant="ghost" disabled={busy || !activeModels.includes(item)} onClick={() => setTestingModel(item)}>测试连接</Button>
            <Button size="sm" variant="ghost" disabled={busy} onClick={event => { editorTrigger.current = event.currentTarget; mutation.reset(); setModel(item) }}>编辑</Button>
            <Button size="sm" variant="ghost" disabled={busy} onClick={() => { mutation.reset(); setDeletingModel(item) }}>删除</Button>
          </div>
        </div>
      </li>
    )
  }
}
