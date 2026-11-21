import { HoverCard, HoverCardContent, HoverCardTrigger } from '@/components/ui/hover-card'
import { ModelBrand, ModelCapabilities, ModelDetails } from './ModelMetadata'
import { ModelCatalogField } from './ModelCatalogField'
import { notify } from '@/lib/notifications'
import { invalidateRSS } from '@/lib/refresh'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { Checkbox } from '@/components/ui/checkbox'
import { Switch } from '@/components/ui/switch'
import { useEffect, useRef, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/aiconfig/api'
import type { Model, Provider } from '../../../bindings/github.com/coolerks/Fealdhord/internal/aiconfig/models'

import { aiConfigurationKey as queryKey } from '@/lib/ai-configuration'
const protocols = [
  ['chat_completions', 'Chat Completions'],
  ['responses', 'Responses'],
  ['messages', 'Messages'],
]
const tasks = [
  ['chat', '聊天'], ['classification', '分类'], ['summary', '摘要'],
  ['translation', '翻译'], ['featured', '精选'],
]
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
  const [deleting, setDeleting] = useState('')
  const [savingProvider, setSavingProvider] = useState(false)
  const [providerError, setProviderError] = useState('')
  const [testing, setTesting] = useState('')
  const [testRunning, setTestRunning] = useState(false)
  const testRequest = useRef<ReturnType<typeof api.TestConnection> | null>(null)
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
        setTesting('')
        setTestRunning(false)
      }
    }
  }
  function editProvider(next: Provider | null) {
    setProviderError('')
    setProvider(next)
    setKey('')
    setClearKey(false)
  }
  const providers = configuration.data?.providers ?? []
  const models = configuration.data?.models ?? []
  const brands = useQuery({ queryKey: ['ai-model-brands'], queryFn: () => api.Brands(), enabled: Boolean(model), retry: false, staleTime: 60 * 60 * 1000 })
  const modelProvider = providers.find(item => item.id === model?.providerId)
  const reasoningLevels = model?.reasoningLevels ?? []
  const activeModels = models.filter(item => item.enabled && providers.some(service => service.id === item.providerId && service.enabled))
  if (configuration.isPending) return <p role="status">正在读取 AI 设置…</p>
  if (configuration.isError) return <Button onClick={() => void configuration.refetch()}>重新读取 AI 设置</Button>
  const busy = mutation.isPending || savingProvider || testRunning
  return <FieldGroup>
    <section className="space-y-3" aria-label="AI 服务">
      <div className="flex items-center justify-between"><h3>AI 服务</h3><Button size="sm" variant="outline" disabled={busy} onClick={() => editProvider(newProvider)}>添加服务</Button></div>
      {providers.map(item => <div key={item.id} className="flex flex-wrap items-center gap-2">
        <span className="mr-auto">{item.name}{!item.enabled && '（已停用）'}</span>
        <Button size="sm" variant="ghost" disabled={busy} onClick={() => editProvider(item)}>编辑</Button>
        <Button size="sm" variant="ghost" disabled={busy} onClick={() => {
          mutation.reset()
          setModel({ id: '', revision: 0, providerId: item.id, name: '', displayName: '', enabled: true, reasoningLevels: defaultReasoningLevels })
        }}>添加模型</Button>
        <Button size="sm" variant="ghost" disabled={busy} onClick={() => setDeleting(item.id)}>删除</Button>
        {deleting === item.id && <div className="w-full space-x-2">
          <span>同时删除该服务的模型和用途分配。</span>
          <Button size="sm" variant="destructive" disabled={busy} onClick={() => mutation.mutate(async () => {
            await api.DeleteProvider(item.id, item.revision)
            setDeleting('')
            editProvider(null)
            setModel(null)
          })}>确认删除</Button>
          <Button size="sm" variant="ghost" onClick={() => setDeleting('')}>取消</Button>
        </div>}
      </div>)}
      {provider && <form className="space-y-3" onSubmit={event => {
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
        <Field><FieldLabel htmlFor="ai-provider-name">服务名称</FieldLabel><Input id="ai-provider-name" required value={provider.name} onChange={event => setProvider({ ...provider, name: event.target.value })} /></Field>
        <Field><FieldLabel htmlFor="ai-protocol">API 类型</FieldLabel><SearchableSelect id="ai-protocol" label="API 类型" className="w-full" value={provider.protocol} disabled={busy}
          onValueChange={protocol => setProvider({ ...provider, protocol })}
          options={protocols.map(([value, label]) => ({ value, label }))} /></Field>
        <Field><FieldLabel htmlFor="ai-base-url">Base URL</FieldLabel><Input id="ai-base-url" type="url" required placeholder="https://api.example.com/v1" value={provider.baseUrl} onChange={event => setProvider({ ...provider, baseUrl: event.target.value })} /></Field>
        <Field><FieldLabel htmlFor="ai-key">API 密钥</FieldLabel><Input id="ai-key" type="password" autoComplete="off" disabled={clearKey} placeholder={provider.hasCredential ? '已保存' : undefined} value={key} onChange={event => setKey(event.target.value)} /></Field>
        {provider.hasCredential && <Field orientation="horizontal"><Checkbox id="ai-clear-key" checked={clearKey} disabled={busy} onCheckedChange={value => setClearKey(value === true)} /><FieldLabel htmlFor="ai-clear-key">清除已保存的密钥</FieldLabel></Field>}
        <Field orientation="horizontal"><Switch id="ai-provider-enabled" checked={provider.enabled} disabled={busy} onCheckedChange={enabled => setProvider({ ...provider, enabled })} /><FieldLabel htmlFor="ai-provider-enabled">启用服务</FieldLabel></Field>
        {providerError && <p role="alert">{providerError}</p>}
        <div className="flex gap-2"><Button size="sm" type="submit" disabled={busy}>保存服务</Button><Button size="sm" type="button" variant="ghost" disabled={busy} onClick={() => editProvider(null)}>取消</Button></div>
      </form>}
    </section>
    <Separator />
    <section className="space-y-3" aria-label="模型">
      <h3>模型</h3>
      <ul className="divide-y divide-border">
      {models.map(item => <li key={item.id} className="flex flex-col gap-2 py-2.5">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <HoverCard openDelay={250}>
            <HoverCardTrigger asChild>
              <div className="flex min-w-0 flex-[1_1_12rem] items-start gap-2">
                <span className="mt-0.5 shrink-0"><ModelBrand brand={item.metadata?.brand} override={item.metadata?.brandOverride} /></span>
                <div className="min-w-0">
                  <span className="block truncate font-medium">{item.displayName}</span>
                  <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1 text-xs text-muted-foreground">
                    <span>{providers.find(service => service.id === item.providerId)?.name}{!item.enabled && '（已停用）'}</span>
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
            <Button size="sm" variant="ghost" disabled={busy} onClick={() => { mutation.reset(); setModel(item) }}>编辑</Button>
            <Button size="sm" variant="ghost" disabled={busy} onClick={() => setDeleting(item.id)}>删除</Button>
            <Button size="sm" variant="ghost" disabled={busy || !activeModels.includes(item) || !!testing} onClick={() => setTesting(item.id)}>测试连接</Button>
          </div>
        </div>
        {testing === item.id && <div className="w-full space-x-2">
          <span>将向服务发送一条测试消息，可能产生费用。</span>
          <Button size="sm" variant="outline" disabled={busy} onClick={() => void runTest(item.id)}>{testRunning ? '正在测试…' : '开始测试'}</Button>
          <Button size="sm" variant="ghost" onClick={() => {
            const pending = testRequest.current
            testRequest.current = null
            if (pending) void pending.cancel()
            setTesting('')
            setTestRunning(false)
          }}>取消</Button>
        </div>}
        {deleting === item.id && <div className="w-full space-x-2">
          <span>同时清除该模型的用途分配。</span>
          <Button size="sm" variant="destructive" disabled={busy} onClick={() => mutation.mutate(async () => {
            await api.DeleteModel(item.id, item.revision)
            setDeleting('')
            setModel(null)
          })}>确认删除</Button>
          <Button size="sm" variant="ghost" onClick={() => setDeleting('')}>取消</Button>
        </div>}
      </li>)}
      </ul>
      {model && <form className="space-y-3" onSubmit={event => {
        event.preventDefault()
        if (busy) return
        mutation.mutate(async () => {
          await api.SaveModel(model)
          setModel(null)
        })
      }}>
        {modelProvider && <ModelCatalogField provider={modelProvider} value={model.name} disabled={busy}
          excluded={models.filter(item => item.providerId === model.providerId && item.id !== model.id).map(item => item.name)}
          onSelect={(name, displayName, metadata) => setModel({ ...model, name, displayName, metadata })} />}
        <Field><FieldLabel htmlFor="ai-model-id">模型 ID</FieldLabel><Input id="ai-model-id" required value={model.name} onChange={event => setModel({ ...model, name: event.target.value, metadata: undefined })} /></Field>
        <Field><FieldLabel htmlFor="ai-model-display">显示名称</FieldLabel><Input id="ai-model-display" value={model.displayName} onChange={event => setModel({ ...model, displayName: event.target.value })} /></Field>
        <div className="grid gap-2 sm:grid-cols-2">
          {(['context', 'outputLimit'] as const).map((field, index) => <Field key={field}>
            <FieldLabel htmlFor={`ai-model-${field}`}>{index === 0 ? '上下文大小（令牌）' : '最大输出（令牌）'}</FieldLabel>
            <Input id={`ai-model-${field}`} type="number" min={1} max={10_000_000} placeholder="自动获取"
              value={model.metadata?.[field] ?? ''} onChange={event => setModel({ ...model, metadata: {
                ...model.metadata, [field]: event.target.value ? Number(event.target.value) : undefined, source: '手动',
              } })} />
          </Field>)}
        </div>
        <Field><FieldLabel htmlFor="ai-model-brand">模型图标</FieldLabel>
          <SearchableSelect id="ai-model-brand" label="模型图标" value={model.metadata?.brandOverride || 'auto'}
            options={[{ value: 'auto', label: '自动匹配' }, { value: 'none', label: '通用图标' }, ...Array.from(new Set([...commonBrands, ...(brands.data ?? []), ...(model.metadata?.brand ? [model.metadata.brand] : [])])).map(brand => ({ value: brand, label: brand, graphic: <ModelBrand brand={brand} /> }))]}
            onValueChange={brand => setModel({ ...model, metadata: { ...model.metadata, brandOverride: brand === 'auto' ? undefined : brand } })} />
        </Field>
        <ModelDetails metadata={model.metadata} />
        <fieldset className="space-y-2" disabled={busy}>
          <legend className="font-medium">思考档位</legend>
          {reasoningLevels.map((level, index) => <div key={index} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] items-end gap-2">
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
              onClick={() => setModel({ ...model, reasoningLevels: reasoningLevels.filter((_, i) => i !== index) })}><Trash2 /></Button>
          </div>)}
          <Button type="button" size="sm" variant="outline" disabled={reasoningLevels.length >= maxReasoningLevels}
            onClick={() => setModel({ ...model, reasoningLevels: [...reasoningLevels, { label: '', value: '' }] })}><Plus />添加档位</Button>
        </fieldset>
        <Field orientation="horizontal"><Switch id="ai-model-enabled" checked={model.enabled} disabled={busy} onCheckedChange={enabled => setModel({ ...model, enabled })} /><FieldLabel htmlFor="ai-model-enabled">启用模型</FieldLabel></Field>
        <div className="flex gap-2"><Button size="sm" type="submit" disabled={busy}>保存模型</Button><Button size="sm" type="button" variant="ghost" disabled={busy} onClick={() => setModel(null)}>取消</Button></div>
      </form>}
    </section>
    <Separator />
    <section className="space-y-3" aria-label="用途分配">
      <h3>用途分配</h3>
      {tasks.map(([task, label]) => {
        const selected = configuration.data.assignments?.find(item => item.task === task)?.modelId ?? ''
        const inactive = selected && !activeModels.some(item => item.id === selected)
        return <Field key={task} orientation="horizontal">
          <FieldLabel htmlFor={`ai-task-${task}`}>{label}</FieldLabel>
          <SearchableSelect id={`ai-task-${task}`} label={label} className="max-w-[65%]" disabled={busy}
            value={selected || 'unassigned'} onValueChange={value => {
              mutation.mutate(() => api.Assign(task, value === 'unassigned' ? '' : value))
            }}
            options={[
              { value: 'unassigned', label: '未分配' },
              ...(inactive ? [{ value: selected, label: `${models.find(item => item.id === selected)?.displayName ?? '模型'}（已停用）`, disabled: true }] : []),
              ...activeModels.map(item => ({
                value: item.id,
                label: `${item.displayName} · ${providers.find(service => service.id === item.providerId)?.name ?? ''}`,
                keywords: [item.name],
              })),
            ]} />
        </Field>
      })}
    </section>
    {mutation.isError && <p role="alert">{configError(mutation.error)}</p>}
  </FieldGroup>
}
