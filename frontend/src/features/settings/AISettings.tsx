import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { Switch } from '@/components/ui/switch'
import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/aiconfig/api'
import type { Model, Provider } from '../../../bindings/github.com/coolerks/Fealdhord/internal/aiconfig/models'

const queryKey = ['ai-configuration'] as const
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
  if (message.includes('ValidationError')) return '请检查名称、地址和密钥格式。'
  return '无法保存 AI 设置，请重试。'
}

export function AISettings() {
  const cache = useQueryClient()
  const configuration = useQuery({ queryKey, queryFn: () => api.List() })
  const mutation = useMutation({
    mutationFn: async (save: () => Promise<unknown>) => { await save() },
    onSettled: async () => { await cache.invalidateQueries({ queryKey }) },
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
  const [testStatus, setTestStatus] = useState('')
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
    setTestStatus('')
    try {
      await pending
      if (testRequest.current === pending) setTestStatus('连接成功')
    } catch (error) {
      if (testRequest.current === pending) setTestStatus(configError(error))
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
        <Button size="sm" variant="ghost" disabled={busy} onClick={() => setModel({ id: '', revision: 0, providerId: item.id, name: '', displayName: '', enabled: true })}>添加模型</Button>
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
          await cache.invalidateQueries({ queryKey })
        }).catch(error => setProviderError(configError(error))).finally(() => setSavingProvider(false))
      }}>
        <Field><FieldLabel htmlFor="ai-provider-name">服务名称</FieldLabel><Input id="ai-provider-name" required value={provider.name} onChange={event => setProvider({ ...provider, name: event.target.value })} /></Field>
        <Field><FieldLabel htmlFor="ai-protocol">API 类型</FieldLabel><Select value={provider.protocol} disabled={busy} onValueChange={protocol => setProvider({ ...provider, protocol })}>
          <SelectTrigger id="ai-protocol" className="w-full"><SelectValue /></SelectTrigger>
          <SelectContent position="popper"><SelectGroup>{protocols.map(([id, label]) => <SelectItem key={id} value={id}>{label}</SelectItem>)}</SelectGroup></SelectContent>
        </Select></Field>
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
      {models.map(item => <div key={item.id} className="flex flex-wrap items-center gap-2">
        <span className="mr-auto">{item.displayName} · {providers.find(service => service.id === item.providerId)?.name}{!item.enabled && '（已停用）'}</span>
        <Button size="sm" variant="ghost" disabled={busy} onClick={() => setModel(item)}>编辑</Button>
        <Button size="sm" variant="ghost" disabled={busy} onClick={() => setDeleting(item.id)}>删除</Button>
        <Button size="sm" variant="ghost" disabled={busy || !activeModels.includes(item) || !!testing} onClick={() => { setTesting(item.id); setTestStatus('') }}>测试连接</Button>
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
      </div>)}
      {testStatus && <p role="status">{testStatus}</p>}
      {model && <form className="space-y-3" onSubmit={event => {
        event.preventDefault()
        mutation.mutate(async () => {
          await api.SaveModel(model)
          setModel(null)
        })
      }}>
        <Field><FieldLabel htmlFor="ai-model-id">模型 ID</FieldLabel><Input id="ai-model-id" required value={model.name} onChange={event => setModel({ ...model, name: event.target.value })} /></Field>
        <Field><FieldLabel htmlFor="ai-model-display">显示名称</FieldLabel><Input id="ai-model-display" value={model.displayName} onChange={event => setModel({ ...model, displayName: event.target.value })} /></Field>
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
          <Select disabled={busy} value={selected || 'unassigned'} onValueChange={value => {
            mutation.mutate(() => api.Assign(task, value === 'unassigned' ? '' : value))
          }}>
            <SelectTrigger id={`ai-task-${task}`} className="min-w-0 max-w-[65%]"><SelectValue /></SelectTrigger>
            <SelectContent position="popper"><SelectGroup>
              <SelectItem value="unassigned">未分配</SelectItem>
              {inactive && <SelectItem value={selected} disabled>{models.find(item => item.id === selected)?.displayName ?? '模型'}（已停用）</SelectItem>}
              {activeModels.map(item => <SelectItem value={item.id} key={item.id}>{item.displayName} · {providers.find(service => service.id === item.providerId)?.name}</SelectItem>)}
            </SelectGroup></SelectContent>
          </Select>
        </Field>
      })}
    </section>
    {mutation.isError && <p role="alert">{configError(mutation.error)}</p>}
  </FieldGroup>
}
