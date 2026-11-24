import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api'
import type { ClientConfig, ClientValue } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/models'

export type ClientLogTarget = { id: string; name: string; attempt?: boolean }

export function newClientLogAttempt() {
  return Array.from(globalThis.crypto.getRandomValues(new Uint8Array(16)), value => value.toString(16).padStart(2, '0')).join('')
}

function ClientLogPanel({ target }: { target: ClientLogTarget }) {
  const logs = useQuery({
    queryKey: ['mcp-client-logs', target.id, Boolean(target.attempt)],
    queryFn: () => api.MCPClientLogs(target.id, Boolean(target.attempt)),
    refetchInterval: 1_000, gcTime: 0, retry: false,
  })
  return <div role="region" aria-label="STDIO 日志"
    className="max-h-[50dvh] min-h-28 overflow-auto rounded-lg border bg-muted/30 p-3" tabIndex={0}>
    {logs.isError ? <FieldError>无法读取日志。</FieldError> : logs.data?.length ? (
      <pre className="select-text break-all whitespace-pre-wrap font-mono text-xs">
        {logs.data.map(entry => [
          new Date(entry.time).toLocaleTimeString(), entry.source, entry.level, entry.message,
        ].filter(Boolean).join('  ')).join('\n')}
      </pre>
    ) : <p className="text-xs text-muted-foreground">{logs.isPending ? '正在读取日志…' : '暂无日志'}</p>}
  </div>
}

export function MCPClientLogDialog({ target, onClose }: { target: ClientLogTarget | null; onClose: () => void }) {
  return <Dialog open={target !== null} onOpenChange={open => { if (!open) onClose() }}>
    {target && <DialogContent aria-describedby={undefined} className="sm:max-w-2xl">
      <DialogHeader><DialogTitle>{target.name} · STDIO 日志</DialogTitle></DialogHeader>
      <ClientLogPanel target={target} />
    </DialogContent>}
  </Dialog>
}

function ClientValues({ label, values = [], onChange, disabled }: {
  label: string
  values?: ClientValue[] | null
  onChange: (values: ClientValue[]) => void
  disabled: boolean
}) {
  const rows = values ?? []
  function update(index: number, patch: Partial<ClientValue>) {
    onChange(rows.map((row, position) => position === index ? { ...row, ...patch } : row))
  }
  return <fieldset className="grid gap-2">
    <legend className="mb-2 text-sm font-medium">{label}</legend>
    {rows.map((row, index) => <div key={index} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] gap-2">
      <Input aria-label={`${label}名称 ${index + 1}`} placeholder="名称" required disabled={disabled}
        value={row.name} onChange={event => update(index, { name: event.target.value, stored: false })} />
      <Input type={row.stored && row.value ? 'text' : 'password'} aria-label={`${label}值 ${index + 1}`} placeholder={row.stored ? '已保存' : '${NAME} 或输入值'}
        autoComplete="off" disabled={disabled} value={row.value ?? ''}
        onChange={event => update(index, { value: event.target.value, stored: false })} />
      <Button type="button" size="icon-sm" variant="ghost" aria-label={`删除${label} ${index + 1}`} disabled={disabled}
        onClick={() => onChange(rows.filter((_, position) => position !== index))}><AnimatedGlyph meaning="x" /></Button>
    </div>)}
    <Button type="button" size="sm" variant="outline" className="w-fit" disabled={disabled}
      onClick={() => onChange([...rows, { name: '', value: '' }])}>
      <AnimatedGlyph meaning="plus" />添加{label}
    </Button>
  </fieldset>
}

const levels = [
  { value: 'debug', label: '调试' }, { value: 'info', label: '信息' },
  { value: 'notice', label: '通知' }, { value: 'warning', label: '警告' },
  { value: 'error', label: '错误' }, { value: 'critical', label: '严重' },
  { value: 'alert', label: '警报' }, { value: 'emergency', label: '紧急' },
]

export function MCPClientEditor({ initial, editing, busy, error, onConnect, onClose }: {
  initial: ClientConfig
  editing: boolean
  busy: boolean
  error?: string
  onConnect: (config: ClientConfig) => void
  onClose: () => void
}) {
  const [config, setConfig] = useState(initial)
  const [kind, setKind] = useState(initial.command ? 'stdio' : 'http')
  const [args, setArgs] = useState(initial.args?.join('\n') ?? '')
  const [advanced, setAdvanced] = useState(false)
  const [attempt, setAttempt] = useState('')
  const [showLogs, setShowLogs] = useState(false)
  const isStdio = kind === 'stdio'
  const isHTTP = /^http:\/\//iu.test(config.endpoint.trim())
  function patch(values: Partial<ClientConfig>) {
    setConfig(current => ({ ...current, ...values }))
  }
  function timeout(id: 'startupTimeout' | 'connectionTimeout' | 'toolTimeout' | 'connectionRetries', label: string, fallback: number) {
    const retry = id === 'connectionRetries'
    return <Field>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Input id={id} type="number" min={retry ? 0 : 1} max={retry ? 10 : 3600} step={1}
        placeholder={String(fallback)} disabled={busy} value={config[id] ?? ''}
        onChange={event => patch({ [id]: event.target.value === '' ? undefined : Number(event.target.value) })} />
    </Field>
  }
  return <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto sm:max-w-lg">
    <DialogHeader><DialogTitle>{editing ? '编辑 MCP 连接' : '添加 MCP 连接'}</DialogTitle></DialogHeader>
    <form onSubmit={event => {
      event.preventDefault()
      const attemptId = isStdio ? newClientLogAttempt() : ''
      setAttempt(attemptId)
      setShowLogs(false)
      onConnect({
        ...config, name: config.name.trim(), newConnection: !editing,
        command: isStdio ? config.command.trim() : '', endpoint: isStdio ? '' : config.endpoint.trim(),
        args: isStdio && args ? args.split(/\r?\n/u) : [],
        skipTLSVerify: !isStdio && !isHTTP && config.skipTLSVerify, attemptId,
      })
    }}>
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor="mcp-client-name">连接名称</FieldLabel>
          <Input id="mcp-client-name" required maxLength={128} disabled={busy}
            value={config.name} onChange={event => patch({ name: event.target.value })} />
        </Field>
        <Field>
          <FieldLabel htmlFor="mcp-client-transport">传输方式</FieldLabel>
          <SearchableSelect id="mcp-client-transport" label="传输方式" className="w-full" value={kind} disabled={busy}
            onValueChange={value => setKind(value)}
            options={[{ value: 'http', label: 'Streamable HTTP' }, { value: 'stdio', label: 'STDIO' }]} />
        </Field>
        {isStdio ? <>
          <Field>
            <FieldLabel htmlFor="mcp-command">命令</FieldLabel>
            <Input id="mcp-command" required disabled={busy} value={config.command}
              onChange={event => patch({ command: event.target.value })} />
          </Field>
          <Field>
            <FieldLabel htmlFor="mcp-args">参数（每行一项）</FieldLabel>
            <Textarea id="mcp-args" rows={3} className="font-mono text-xs" disabled={busy} value={args}
              onChange={event => setArgs(event.target.value)} />
          </Field>
          <ClientValues label="环境变量" values={config.environment} disabled={busy}
            onChange={environment => patch({ environment })} />
          <Field>
            <FieldLabel htmlFor="mcp-directory">工作目录</FieldLabel>
            <Input id="mcp-directory" disabled={busy} value={config.workingDirectory ?? ''}
              onChange={event => patch({ workingDirectory: event.target.value })} />
          </Field>
        </> : <>
          <Field>
            <FieldLabel htmlFor="mcp-endpoint">URL</FieldLabel>
            <Input id="mcp-endpoint" required placeholder="https://example.com/mcp 或 ${MCP_URL}" disabled={busy}
              value={config.endpoint} onChange={event => patch({ endpoint: event.target.value })} />
          </Field>
          <ClientValues label="请求头" values={config.headers} disabled={busy} onChange={headers => patch({ headers })} />
        </>}
        <Button type="button" variant="ghost" className="w-fit" aria-controls="mcp-client-advanced" aria-expanded={advanced}
          onClick={() => setAdvanced(value => !value)}>
          <AnimatedGlyph meaning="chevron-right" className={advanced ? 'rotate-90' : undefined} />高级设置
        </Button>
        <div id="mcp-client-advanced" hidden={!advanced}>
          <FieldGroup>
            {isStdio ? <>
              {timeout('startupTimeout', '启动超时（秒）', 15)}
              {timeout('toolTimeout', '工具调用超时（秒）', 120)}
              <Field>
                <FieldLabel htmlFor="mcp-log-level">日志等级</FieldLabel>
                <SearchableSelect id="mcp-log-level" label="日志等级" className="w-full" disabled={busy}
                  value={config.logLevel ?? 'warning'} options={levels} onValueChange={logLevel => patch({ logLevel })} />
              </Field>
            </> : <>
              {timeout('connectionTimeout', '连接超时（秒）', 120)}
              {timeout('toolTimeout', '工具调用超时（秒）', 120)}
              <fieldset className="grid gap-3">
                <legend className="mb-2 text-sm font-medium">HTTP 代理</legend>
                <div className="grid grid-cols-[minmax(0,2fr)_minmax(0,1fr)] gap-2">
                  <Field>
                    <FieldLabel htmlFor="mcp-proxy-host">地址</FieldLabel>
                    <Input id="mcp-proxy-host" disabled={busy} value={config.proxyHost ?? ''}
                      placeholder="留空时直连" onChange={event => patch({ proxyHost: event.target.value })} />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="mcp-proxy-port">端口</FieldLabel>
                    <Input id="mcp-proxy-port" disabled={busy} value={config.proxyPort ?? ''}
                      onChange={event => patch({ proxyPort: event.target.value })} />
                  </Field>
                </div>
                <div className="grid grid-cols-2 gap-2">
                  <Field>
                    <FieldLabel htmlFor="mcp-proxy-user">用户名</FieldLabel>
                    <Input id="mcp-proxy-user" disabled={busy} value={config.proxyUsername ?? ''}
                      autoComplete="off" onChange={event => patch({ proxyUsername: event.target.value })} />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="mcp-proxy-password">密码</FieldLabel>
                    <Input id="mcp-proxy-password" type="password" autoComplete="off" disabled={busy}
                      placeholder={config.proxyPasswordSet ? '已保存' : ''} value={config.proxyPassword ?? ''}
                      onChange={event => patch({ proxyPassword: event.target.value, proxyPasswordSet: false })} />
                    {config.proxyPasswordSet && <Button type="button" size="xs" variant="ghost" disabled={busy}
                      onClick={() => patch({ proxyPasswordSet: false, proxyPassword: '' })}>清除已保存密码</Button>}
                  </Field>
                </div>
              </fieldset>
              {!isHTTP && <div className="flex justify-between gap-3">
                <FieldLabel htmlFor="mcp-tls">TLS 证书验证</FieldLabel>
                <Switch id="mcp-tls" aria-label="TLS 证书验证" checked={!config.skipTLSVerify} disabled={busy}
                  onCheckedChange={checked => patch({ skipTLSVerify: !checked })} />
              </div>}
              {timeout('connectionRetries', '连接重试次数', 3)}
            </>}
          </FieldGroup>
        </div>
        {error && <FieldError>{error}</FieldError>}
        {isStdio && attempt && (busy || error) && <>
          <Button type="button" size="sm" variant="outline" className="w-fit" aria-expanded={showLogs}
            onClick={() => setShowLogs(value => !value)}>查看启动日志</Button>
          {showLogs && <ClientLogPanel target={{ id: attempt, name: config.name, attempt: true }} />}
        </>}
        <DialogFooter>
          <Button type="button" size="sm" variant="outline" disabled={busy} onClick={onClose}>取消</Button>
          <Button type="submit" size="sm" disabled={busy}>{busy ? '连接中…' : '连接'}</Button>
        </DialogFooter>
      </FieldGroup>
    </form>
  </DialogContent>
}

export function MCPToolList({ tools }: { tools: string[] }) {
  const [search, setSearch] = useState('')
  const query = search.trim().toLowerCase()
  const filtered = tools.filter(tool => tool.toLowerCase().includes(query))
  return <div className="grid w-full gap-2">
    {tools.length > 16 && <Input aria-label="搜索连接工具" placeholder="搜索工具…" value={search}
      onChange={event => setSearch(event.target.value)} />}
    {filtered.length ? <div className="flex max-h-40 flex-wrap items-center gap-1 overflow-auto">
      {filtered.map(name => <code key={name} className="wrap-anywhere rounded bg-muted px-1.5 py-0.5 text-[11px]">{name}</code>)}
    </div> : <p className="text-muted-foreground">没有匹配工具</p>}
  </div>
}
