import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { native } from '@/lib/native'
import { navigationAPI, navigationQueryKey } from '@/lib/navigation-api'
import { notify } from '@/lib/notifications'
import { ConfirmDialog, SettingCard, SettingEmpty, SettingRow, SettingSection, StatusBadge } from './settings-primitives'
import { getToolMeta } from '@/features/chat/tool-meta'
import { MCPClientEditor, MCPClientLogDialog, MCPToolList, newClientLogAttempt } from './MCPClientEditor'
import type { ClientLogTarget } from './MCPClientEditor'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api'
import type { ClientConfig, MCPGrant, ServerConfig, Skill, Tool } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/models'

const platformKey = ['ai-platform']
export const skillsKey = [...platformKey, 'skills']

const scopedArticleTools = new Set([
  'article.list', 'article.search', 'article.get', 'article.get_metadata', 'article.extract',
  'article.mark_read', 'article.mark_unread', 'article.favorite', 'article.unfavorite',
])

const auditStatuses: Record<string, { label: string; tone: 'active' | 'muted' | 'warning' | 'error' }> = {
  complete: { label: '已返回', tone: 'active' },
  failed: { label: '失败', tone: 'error' },
  cancelled: { label: '已取消', tone: 'muted' },
  expired: { label: '已超时', tone: 'warning' },
  denied: { label: '已拒绝', tone: 'warning' },
  unknown: { label: '结果未知', tone: 'muted' },
}

const newClient: ClientConfig = { name: '', endpoint: '', command: '', args: [] }
function newSkillDraft(skills: Skill[]): Pick<Skill, 'markdown' | 'enabled' | 'resources'> {
  const names = new Set(skills.map(skill => skill.name))
  let name = 'my-skill'
  for (let suffix = 2; names.has(name); suffix++) name = `my-skill-${suffix}`
  return {
    markdown: `---\nname: ${name}\ndescription: 从用户提供的材料中提炼要点\n---\n\n用简洁的中文列出主要观点。材料不足时，指出缺少的信息。\n`,
    enabled: true,
  }
}

const formatAuditTime = (value: string) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'short' }).format(date)
}

function toolOption(name: string, tool?: Tool) {
  const description = tool?.description
  const meta = getToolMeta(name)
  return {
    value: name,
    label: name,
    graphic: <AnimatedGlyph meaning={meta.icon} />,
    keywords: description ? [description, meta.label, meta.summary] : [meta.label, meta.summary],
    description: description && tool?.requiresApproval && !description.includes('确认')
      ? `${description}\n运行前需确认` : description,
  }
}

function CopyButton({ value, label }: { value: string; label: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <Button
      type="button"
      size="icon-sm"
      variant="ghost"
      aria-label={label}
      title={label}
      onClick={() => {
        void native.copy(value).then(() => {
          setCopied(true)
          void notify('已复制到剪贴板')
          window.setTimeout(() => setCopied(false), 1600)
        }).catch(() => {
          void notify('复制失败', undefined, true)
        })
      }}
    >
      <AnimatedGlyph meaning={copied ? 'check' : 'copy'} />
    </Button>
  )
}

type GrantDraft = { name: string; tools: string[]; scope: string; editing: boolean }
type ClientDraft = { config: ClientConfig; editing: boolean }

const connectionErrors: Record<string, string> = {
  InvalidClientSettings: '连接配置有误，请检查名称和各字段。',
  InvalidEndpoint: '请输入有效的 HTTP 或 HTTPS 地址。',
  InvalidMCPProxy: '请检查代理地址、端口和登录信息。',
  HTTPSRequiredForCredential: '此地址需要使用 HTTPS 才能发送凭据。',
  MCPClientNameExists: '已有同名连接，请更换名称。',
  MCPClientNotFound: '此连接已移除，请重新添加。',
  MCPConnectionFailed: '无法连接 MCP 服务，请检查连接配置。',
  MCPConnectionTimeout: 'MCP 连接超时。',
  MCPDiscoveryFailed: '无法读取 MCP 工具目录。',
  MCPInvalidToolCatalog: 'MCP 服务返回了无效的工具分页。',
  MCPToolCatalogLimit: '工具目录超出容量，连接未更新。',
  ClientCredentialUnavailable: '已保存的凭据不可用，请重新输入。',
  CredentialError: '无法访问系统凭据库。',
}

function clientConnectionError(error: unknown) {
  const code = error instanceof Error ? error.message : ''
  const missing = /^MissingEnvironmentVariable: ([A-Za-z_][A-Za-z0-9_]*)$/u.exec(code)
  return missing ? `系统环境变量 ${missing[1]} 未设置。`
    : connectionErrors[code] ?? '无法保存 MCP 连接，请检查配置后重试。'
}

export function MCPSettings() {
  const cache = useQueryClient()
  const server = useQuery({ queryKey: [...platformKey, 'server'], queryFn: () => api.MCPServer(), retry: false })
  const audit = useQuery({ queryKey: [...platformKey, 'audit'], queryFn: () => api.RecentAudit(), enabled: false, retry: false })
  const tools = useQuery({ queryKey: [...platformKey, 'tools'], queryFn: () => api.ToolList(), retry: false })
  const grants = useQuery({ queryKey: [...platformKey, 'grants'], queryFn: () => api.Grants(), retry: false })
  const clients = useQuery({ queryKey: [...platformKey, 'clients'], queryFn: () => api.MCPClients(), retry: false })
  const navigation = useQuery({ queryKey: navigationQueryKey, queryFn: () => navigationAPI.list() })

  const [draft, setDraft] = useState<ServerConfig>()
  const [token, setToken] = useState('')
  const [grantToken, setGrantToken] = useState<{ name: string; value: string } | null>(null)
  const [grantEditor, setGrantEditor] = useState<GrantDraft | null>(null)
  const [revokingGrant, setRevokingGrant] = useState<MCPGrant | null>(null)
  const [rotatingGrant, setRotatingGrant] = useState<MCPGrant | null>(null)
  const [clientEditor, setClientEditor] = useState<ClientDraft | null>(null)
  const [connecting, setConnecting] = useState(false)
  const [connectError, setConnectError] = useState<string>()
  const [forgettingClient, setForgettingClient] = useState<{ id: string; name: string } | null>(null)
  const [clientLog, setClientLog] = useState<ClientLogTarget | null>(null)
  const [expandedClients, setExpandedClients] = useState<Set<string>>(() => new Set())

  const scopeOptions = [
    { value: 'all', label: '所有订阅及分类' },
    ...(navigation.data?.feeds ?? []).map(feed => ({ value: `feed:${feed.id}`, label: `订阅 · ${feed.title}` })),
    ...(navigation.data?.categories ?? []).map(category => ({ value: `category:${category.id}`, label: `分类 · ${category.name}` })),
  ]

  const mutation = useMutation({
    mutationFn: async (save: () => Promise<unknown>) => { await save() },
    onSuccess: () => cache.invalidateQueries({ queryKey: platformKey }),
  })

  const config = draft ?? server.data?.config
  const busy = mutation.isPending || connecting
  const errorCode = mutation.error?.message
  const mutationError = errorCode === 'ClientGrantExists' ? '该名称已有客户端授权，请更换名称或编辑现有授权。'
    : errorCode === 'ClientGrantNotFound' ? '该客户端授权已不存在。'
    : mutation.isError ? `MCP 设置失败（${errorCode ?? '请检查配置'}）`
    : undefined

  const [scopeKind, scopeId = ''] = (grantEditor?.scope ?? 'all').split(':')
  const allowedGrantTools = (server.data?.config.tools ?? []).filter(name =>
    scopeKind === 'all' || scopedArticleTools.has(name) || name === `${scopeKind}.get`,
  )
  const selectedGrantTools = grantEditor?.tools.filter(name => allowedGrantTools.includes(name)) ?? []

  function scopeLabel(grant: MCPGrant) {
    if (!grant.scope.kind) return '所有订阅及分类'
    const key = `${grant.scope.kind}:${grant.scope.id}`
    return scopeOptions.find(item => item.value === key)?.label ?? key
  }

  function toggleClientDetails(name: string) {
    setExpandedClients(current => {
      const next = new Set(current)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }

  function closeClientEditor() {
    setClientEditor(null)
    setConnectError(undefined)
  }

  async function connectClient(next: ClientConfig) {
    setConnecting(true)
    setConnectError(undefined)
    try {
      await api.ConnectMCP(next)
      closeClientEditor()
      await cache.invalidateQueries({ queryKey: platformKey })
    } catch (error) {
      setConnectError(clientConnectionError(error))
    } finally {
      setConnecting(false)
    }
  }

  return <div className="space-y-6">
    <SettingSection
      title="MCP 服务端"
      actions={server.isError && <Button size="sm" variant="outline" onClick={() => void server.refetch()}>重试读取服务端</Button>}
    >
      {config && (
        <form onSubmit={event => {
          event.preventDefault()
          mutation.mutate(async () => { await api.SaveMCPServer(config); setDraft(undefined) })
        }}>
          <SettingCard>
            <SettingRow>
              <div className="flex items-center gap-2">
                <FieldLabel htmlFor="mcp-serve" className="font-normal">启用 MCP 服务</FieldLabel>
                <StatusBadge tone={server.data?.config.enabled ? 'active' : 'muted'}>
                  {server.data?.config.enabled ? '运行中' : '已停用'}
                </StatusBadge>
              </div>
              <Switch id="mcp-serve" checked={config.enabled} onCheckedChange={enabled => setDraft({ ...config, enabled })} />
            </SettingRow>
            {server.data?.url && (
              <SettingRow>
                <span className="text-xs text-muted-foreground">服务地址</span>
                <div className="flex min-w-0 items-center gap-1">
                  <output className="select-text truncate font-mono text-xs">{server.data.url}</output>
                  <CopyButton value={server.data.url} label="复制 MCP 服务地址" />
                </div>
              </SettingRow>
            )}
            <SettingRow>
              <FieldLabel htmlFor="mcp-port" className="font-normal">监听端口</FieldLabel>
              <Input
                id="mcp-port"
                type="number"
                className="w-28"
                min={1024}
                max={65535}
                required
                value={config.port || ''}
                onChange={event => setDraft({ ...config, port: Number(event.target.value) })}
              />
            </SettingRow>
            <SettingRow>
              <FieldLabel htmlFor="mcp-tools" className="font-normal">公开工具</FieldLabel>
              <SearchableSelect
                id="mcp-tools"
                label="公开工具"
                className="min-w-48"
                value={config.tools ?? []}
                placeholder={config.tools?.length ? `已选 ${config.tools.length} 项` : '选择工具'}
                options={(tools.data ?? []).map(tool => toolOption(tool.name, tool))}
                onValueChange={name => {
                  const selected = config.tools ?? []
                  setDraft({ ...config, tools: selected.includes(name) ? selected.filter(item => item !== name) : [...selected, name] })
                }}
              />
            </SettingRow>
            <SettingRow>
              <FieldLabel htmlFor="mcp-external" className="font-normal">允许本机外接入</FieldLabel>
              <Switch id="mcp-external" checked={config.external} onCheckedChange={external => setDraft({ ...config, external })} />
            </SettingRow>
            {config.external && (
              <div className="px-3.5 py-2.5">
                <Field>
                  <FieldLabel htmlFor="mcp-address">监听 IP</FieldLabel>
                  <Input id="mcp-address" value={config.address} onChange={event => setDraft({ ...config, address: event.target.value })} />
                </Field>
              </div>
            )}
            <SettingRow>
              <FieldLabel htmlFor="mcp-tls" className="font-normal">启用 HTTPS</FieldLabel>
              <Switch id="mcp-tls" checked={config.tls} onCheckedChange={tls => setDraft({ ...config, tls })} />
            </SettingRow>
            {config.tls && (
              <div className="grid gap-3 px-3.5 py-2.5 sm:grid-cols-2">
                <Field>
                  <FieldLabel htmlFor="mcp-cert">证书路径</FieldLabel>
                  <Input id="mcp-cert" required value={config.certificate} onChange={event => setDraft({ ...config, certificate: event.target.value })} />
                </Field>
                <Field>
                  <FieldLabel htmlFor="mcp-key">私钥路径</FieldLabel>
                  <Input id="mcp-key" required value={config.privateKey} onChange={event => setDraft({ ...config, privateKey: event.target.value })} />
                </Field>
              </div>
            )}
            <SettingRow className="bg-muted/25">
              <Button size="sm" type="button" variant="outline" disabled={busy}
                onClick={() => mutation.mutate(async () => setToken(await api.RotateMCPToken()))}>
                生成或更换令牌
              </Button>
              <Button size="sm" type="submit" disabled={busy || !draft}>保存服务端</Button>
            </SettingRow>
          </SettingCard>
        </form>
      )}
      {token && (
        <SettingCard divided={false} className="space-y-2 p-3.5">
          <div className="flex items-center justify-between gap-2">
            <FieldLabel htmlFor="mcp-token">访问令牌</FieldLabel>
            <StatusBadge tone="warning">仅显示本次</StatusBadge>
          </div>
          <div className="flex items-center gap-1.5">
            <Input id="mcp-token" readOnly value={token} autoComplete="off" className="font-mono text-xs" />
            <CopyButton value={token} label="复制访问令牌" />
          </div>
        </SettingCard>
      )}
    </SettingSection>

    <SettingSection
      title="客户端授权"
      actions={
        <Button size="sm" variant="outline" disabled={busy} onClick={() => {
          mutation.reset()
          setGrantEditor({ name: '', tools: [], scope: 'all', editing: false })
        }}>
          <AnimatedGlyph meaning="plus" />新建授权
        </Button>
      }
    >
      <SettingCard>
        {(grants.data ?? []).length === 0 ? (
          <SettingEmpty icon={<AnimatedGlyph meaning="key" />} title="还没有客户端授权" />
        ) : (grants.data ?? []).map(grant => (
          <div key={grant.name}>
            <SettingRow>
              <div className="min-w-0 flex-1 space-y-0.5">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="truncate font-medium">{grant.name}</span>
                  <StatusBadge tone="muted">{grant.tools?.length ?? 0} 项工具</StatusBadge>
                </div>
                <p className="truncate text-xs text-muted-foreground">{scopeLabel(grant)}</p>
              </div>
              <div className="flex shrink-0 items-center gap-1">
                <Button size="sm" variant="ghost" disabled={busy} onClick={() => {
                  mutation.reset()
                  setGrantEditor({
                    name: grant.name,
                    tools: grant.tools ?? [],
                    scope: grant.scope.kind ? `${grant.scope.kind}:${grant.scope.id}` : 'all',
                    editing: true,
                  })
                }}>编辑</Button>
                <Button size="sm" variant="ghost" disabled={busy} onClick={() => { mutation.reset(); setRotatingGrant(grant) }}>更换令牌</Button>
                <Button size="sm" variant="ghost" disabled={busy} onClick={() => { mutation.reset(); setRevokingGrant(grant) }}>撤销</Button>
              </div>
            </SettingRow>
            {grantToken?.name === grant.name && (
              <div className="space-y-2 px-3.5 pb-3">
                <div className="flex items-center justify-between gap-2">
                  <FieldLabel htmlFor="mcp-client-token">{grant.name}的客户端令牌</FieldLabel>
                  <StatusBadge tone="warning">仅显示本次</StatusBadge>
                </div>
                <div className="flex items-center gap-1.5">
                  <Input id="mcp-client-token" readOnly value={grantToken.value} autoComplete="off" className="font-mono text-xs" />
                  <CopyButton value={grantToken.value} label="复制客户端令牌" />
                </div>
              </div>
            )}
          </div>
        ))}
      </SettingCard>
    </SettingSection>

    <SettingSection
      title="外部 MCP 连接"
      actions={
        <Button size="sm" variant="outline" disabled={busy} onClick={() => {
          mutation.reset()
          setClientEditor({ config: newClient, editing: false })
        }}>
          <AnimatedGlyph meaning="plus" />添加连接
        </Button>
      }
    >
      <SettingCard>
        {(clients.data ?? []).length === 0 ? (
          <SettingEmpty icon={<AnimatedGlyph meaning="webhook" />} title="还没有外部 MCP 连接" />
        ) : (clients.data ?? []).map(item => {
          const toolCount = item.tools?.length ?? 0
          const resourceCount = item.resources?.length ?? 0
          const promptCount = item.prompts?.length ?? 0
          const hasDetails = toolCount > 0 || resourceCount > 0 || promptCount > 0
          const expanded = expandedClients.has(item.id)
          const summaryParts = [
            toolCount > 0 ? `${toolCount} 项工具` : '',
            resourceCount > 0 ? `${resourceCount} 项资源` : '',
            promptCount > 0 ? `${promptCount} 项提示词` : '',
          ].filter(Boolean)
          return (
            <div key={item.id} className="space-y-2 px-3.5 py-2.5">
              <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
                <div className="min-w-0 flex-1 space-y-0.5">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="truncate font-medium">{item.name}</span>
                    <StatusBadge tone={item.connected ? 'active' : 'muted'}>
                      {item.connected ? '已连接' : '已断开'}
                    </StatusBadge>
                    {hasDetails && (
                      <Button
                        type="button"
                        size="xs"
                        variant="ghost"
                        aria-expanded={expanded}
                        onClick={() => toggleClientDetails(item.id)}
                      >
                        <span className="inline-flex" style={{ rotate: expanded ? '90deg' : undefined }}>
                          <AnimatedGlyph meaning="chevron-right" />
                        </span>
                        {summaryParts.join(' · ')}
                      </Button>
                    )}
                  </div>
                  <p className="truncate font-mono text-xs text-muted-foreground">
                    {item.config.command || item.config.endpoint}
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-1">
                  {item.config.command && <Button size="sm" variant="ghost"
                    onClick={() => setClientLog({ id: item.id, name: item.name })}>查看日志</Button>}
                  {item.connected
                    ? <Button size="sm" variant="ghost" disabled={busy} onClick={() => mutation.mutate(() => api.DisconnectMCP(item.id))}>断开</Button>
                    : <Button size="sm" variant="ghost" disabled={busy} onClick={() => mutation.mutate(async () => {
                      const attemptId = item.config.command ? newClientLogAttempt() : ''
                      try {
                        await api.ConnectMCP({ ...item.config, attemptId })
                      } catch (error) {
                        if (attemptId) setClientLog({ id: attemptId, name: item.name, attempt: true })
                        throw error
                      }
                    })}>重连</Button>}
                  <Button size="sm" variant="ghost" disabled={busy} onClick={() => {
                    mutation.reset()
                    setClientEditor({
                      config: item.config,
                      editing: true,
                    })
                  }}>编辑</Button>
                  <Button size="sm" variant="ghost" disabled={busy} onClick={() => { mutation.reset(); setForgettingClient({ id: item.id, name: item.name }) }}>移除</Button>
                </div>
              </div>
              {expanded && hasDetails && (
                <div className="space-y-1.5 rounded-lg border border-border bg-muted/30 p-2.5 text-xs">
                  {toolCount > 0 && (
                    <div className="flex flex-wrap items-center gap-1">
                      <span className="mr-1 text-muted-foreground">工具</span>
                      <MCPToolList tools={item.tools ?? []} />
                    </div>
                  )}
                  {resourceCount > 0 && (
                    <div className="flex flex-wrap items-center gap-1">
                      <span className="mr-1 text-muted-foreground">资源</span>
                      {item.resources!.map(name => <code key={name} className="rounded bg-muted px-1.5 py-0.5 font-mono text-[11px]">{name}</code>)}
                    </div>
                  )}
                  {promptCount > 0 && (
                    <div className="flex flex-wrap items-center gap-1">
                      <span className="mr-1 text-muted-foreground">提示词</span>
                      {item.prompts!.map(name => <code key={name} className="rounded bg-muted px-1.5 py-0.5 font-mono text-[11px]">{name}</code>)}
                    </div>
                  )}
                </div>
              )}
            </div>
          )
        })}
      </SettingCard>
    </SettingSection>

    <SettingSection
      title="工具记录"
      actions={
        <Button size="sm" variant="outline" disabled={audit.isFetching} onClick={() => void audit.refetch()}>
          <AnimatedGlyph meaning="history" />{audit.data ? '刷新记录' : '查看最近记录'}
        </Button>
      }
    >
      {audit.data && (
        <SettingCard>
          {audit.data.length === 0 ? (
            <SettingEmpty icon={<AnimatedGlyph meaning="history" />} title="暂无工具记录" />
          ) : audit.data.map(entry => {
            const badge = auditStatuses[entry.status] ?? { label: entry.status, tone: 'muted' as const }
            const when = formatAuditTime(entry.startedAt)
            return (
              <SettingRow key={`${entry.startedAt}${entry.tool}`}>
                <div className="min-w-0 flex-1 space-y-0.5">
                  <div className="flex flex-wrap items-center gap-2">
                    <code className="font-mono text-xs font-medium">{entry.tool}</code>
                    <span className="text-xs text-muted-foreground">{entry.principal}</span>
                  </div>
                  {when && <p className="text-[11px] text-muted-foreground tabular-nums">{when}</p>}
                </div>
                <StatusBadge tone={badge.tone}>{badge.label}</StatusBadge>
              </SettingRow>
            )
          })}
        </SettingCard>
      )}
    </SettingSection>

    {mutationError && !grantEditor && !clientEditor && !revokingGrant && !rotatingGrant && !forgettingClient && <FieldError>{mutationError}</FieldError>}

    <Dialog open={Boolean(grantEditor)} onOpenChange={open => { if (!open && !busy) { setGrantEditor(null); mutation.reset() } }}>
      {grantEditor && (
        <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{grantEditor.editing ? '编辑客户端授权' : '新建客户端授权'}</DialogTitle>
          </DialogHeader>
          <form onSubmit={event => {
            event.preventDefault()
            mutation.mutate(async () => {
              const grant = {
                name: grantEditor.name,
                tools: selectedGrantTools,
                scope: { kind: scopeKind === 'all' ? '' : scopeKind, id: scopeId },
              }
              if (grantEditor.editing) await api.UpdateGrant(grant)
              else setGrantToken({ name: grant.name, value: await api.SaveGrant(grant, false) })
              setGrantEditor(null)
            })
          }}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="mcp-grant-name">客户端名称</FieldLabel>
                <Input
                  id="mcp-grant-name"
                  required
                  maxLength={32}
                  disabled={busy || grantEditor.editing}
                  value={grantEditor.name}
                  onChange={event => setGrantEditor({ ...grantEditor, name: event.target.value })}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="mcp-grant-scope">数据范围</FieldLabel>
                <SearchableSelect
                  id="mcp-grant-scope"
                  label="数据范围"
                  className="w-full"
                  value={grantEditor.scope}
                  disabled={busy}
                  options={scopeOptions}
                  onValueChange={scope => setGrantEditor({ ...grantEditor, scope, tools: [] })}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="mcp-grant-tools">允许的工具</FieldLabel>
                <SearchableSelect
                  id="mcp-grant-tools"
                  label="允许的工具"
                  className="w-full"
                  placeholder={selectedGrantTools.length ? `已选 ${selectedGrantTools.length} 项` : '选择工具'}
                  value={selectedGrantTools}
                  disabled={busy}
                  options={allowedGrantTools.map(name => toolOption(name, tools.data?.find(tool => tool.name === name)))}
                  onValueChange={name => setGrantEditor({
                    ...grantEditor,
                    tools: grantEditor.tools.includes(name)
                      ? grantEditor.tools.filter(item => item !== name)
                      : [...grantEditor.tools, name],
                  })}
                />
              </Field>
              {mutationError && <FieldError>{mutationError}</FieldError>}
              <DialogFooter>
                <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => { setGrantEditor(null); mutation.reset() }}>取消</Button>
                <Button type="submit" size="sm" disabled={busy || selectedGrantTools.length === 0}>
                  {grantEditor.editing ? '保存授权' : '生成客户端令牌'}
                </Button>
              </DialogFooter>
            </FieldGroup>
          </form>
        </DialogContent>
      )}
    </Dialog>

    <Dialog open={Boolean(clientEditor)} onOpenChange={open => {
      if (!open && !busy) closeClientEditor()
    }}>
      {clientEditor && <MCPClientEditor initial={clientEditor.config} editing={clientEditor.editing}
        busy={busy} error={connectError} onClose={closeClientEditor}
        onConnect={config => { void connectClient(config) }} />}
    </Dialog>

    <MCPClientLogDialog target={clientLog} onClose={() => setClientLog(null)} />

    <ConfirmDialog
      open={Boolean(revokingGrant)}
      title={revokingGrant ? `撤销“${revokingGrant.name}”授权？` : ''}
      description="该客户端持有的令牌将立即失效。"
      confirmLabel="确认撤销"
      busy={busy}
      error={revokingGrant ? mutationError : undefined}
      onOpenChange={open => { if (!open) { setRevokingGrant(null); mutation.reset() } }}
      onConfirm={() => {
        if (!revokingGrant) return
        mutation.mutate(async () => {
          await api.SaveGrant(revokingGrant, true)
          setGrantToken(current => current?.name === revokingGrant.name ? null : current)
          setRevokingGrant(null)
        })
      }}
    />

    <ConfirmDialog
      open={Boolean(rotatingGrant)}
      title={rotatingGrant ? `更换“${rotatingGrant.name}”的令牌？` : ''}
      description="旧令牌会立即失效。客户端需要使用新令牌重新连接。"
      confirmLabel="确认更换"
      busy={busy}
      error={rotatingGrant ? mutationError : undefined}
      onOpenChange={open => { if (!open) { setRotatingGrant(null); mutation.reset() } }}
      onConfirm={() => {
        if (!rotatingGrant) return
        mutation.mutate(async () => {
          const name = rotatingGrant.name
          setGrantToken({ name, value: await api.RotateGrantToken(name) })
          setRotatingGrant(null)
        })
      }}
    />

    <ConfirmDialog
      open={Boolean(forgettingClient)}
      title={forgettingClient ? `移除连接“${forgettingClient.name}”？` : ''}
      description="将断开并删除此 MCP 连接配置。"
      confirmLabel="确认移除"
      busy={busy}
      error={forgettingClient ? mutationError : undefined}
      onOpenChange={open => { if (!open) { setForgettingClient(null); mutation.reset() } }}
      onConfirm={() => {
        if (!forgettingClient) return
        mutation.mutate(async () => {
          await api.ForgetMCP(forgettingClient.id)
          setForgettingClient(null)
        })
      }}
    />
  </div>
}

export function SkillsSettings() {
  const cache = useQueryClient()
  const skills = useQuery({ queryKey: skillsKey, queryFn: () => api.Skills(), retry: false })
  const [skill, setSkill] = useState<Pick<Skill, 'markdown' | 'enabled' | 'resources'> | null>(null)
  const [deleting, setDeleting] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: async (save: () => Promise<unknown>) => { await save() },
    onSuccess: () => cache.invalidateQueries({ queryKey: platformKey }),
  })

  const busy = mutation.isPending
  const skillError = mutation.error instanceof Error ? mutation.error.message : ''
  const mutationError = mutation.isError
    ? skillError === 'SkillBodyRequired'
      ? '请在第二条 --- 下方填写 Skill 的正文。'
      : `Skill 操作失败（${skillError || '请检查内容'}）`
    : undefined

  return <div className="space-y-6">
    <SettingSection
      title="Skills"
      actions={
        <>
          <Button size="sm" variant="outline" disabled={busy} onClick={() => mutation.mutate(async () => { setSkill(await api.ImportSkill()) })}>
            <AnimatedGlyph meaning="upload" />导入 Skill
          </Button>
          <Button size="sm" variant="outline" disabled={busy || skills.isPending}
            onClick={() => { mutation.reset(); setSkill(newSkillDraft(skills.data ?? [])) }}>
            <AnimatedGlyph meaning="plus" />添加 Skill
          </Button>
        </>
      }
    >
      <SettingCard>
        {(skills.data ?? []).length === 0 ? (
          <SettingEmpty icon={<AnimatedGlyph meaning="blocks" />} title="还没有已安装的 Skill" />
        ) : (skills.data ?? []).map(item => (
          <SettingRow key={item.name}>
            <div className="min-w-0 flex-1 space-y-0.5">
              <div className="flex flex-wrap items-center gap-2">
                <span className="truncate font-medium">{item.name}</span>
                {!item.enabled && <StatusBadge tone="muted">已停用</StatusBadge>}
              </div>
              {item.description && <p className="truncate text-xs text-muted-foreground">{item.description}</p>}
            </div>
            <div className="flex shrink-0 items-center gap-1.5">
              <Switch
                aria-label={`启用 ${item.name}`}
                checked={item.enabled}
                disabled={busy}
                onCheckedChange={enabled => mutation.mutate(() => api.SaveSkill(item.markdown, enabled))}
              />
              <Button size="sm" variant="ghost" disabled={busy} onClick={() => { mutation.reset(); setSkill(item) }}>编辑</Button>
              <Button size="sm" variant="ghost" disabled={busy} onClick={() => { mutation.reset(); setDeleting(item.name) }}>删除</Button>
            </div>
          </SettingRow>
        ))}
      </SettingCard>
      {mutationError && !skill && !deleting && <FieldError>{mutationError}</FieldError>}
    </SettingSection>

    <Dialog open={Boolean(skill)} onOpenChange={open => { if (!open && !busy) { setSkill(null); mutation.reset() } }}>
      {skill && (
        <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>编辑 Skill</DialogTitle>
          </DialogHeader>
          <form onSubmit={event => {
            event.preventDefault()
            mutation.mutate(async () => {
              await api.SaveSkillPackage({ name: '', description: '', tools: [], ...skill })
              setSkill(null)
            })
          }}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="ai-skill-markdown">SKILL.md</FieldLabel>
                <Textarea
                  id="ai-skill-markdown"
                  className="font-mono text-xs leading-relaxed"
                  rows={12}
                  value={skill.markdown}
                  required
                  onChange={event => setSkill({ ...skill, markdown: event.target.value })}
                />
              </Field>
              {skill.resources?.length ? (
                <div className="space-y-1">
                  <span className="text-xs font-medium text-muted-foreground">附带资源</span>
                  <ul className="space-y-1 rounded-lg border border-border bg-muted/30 p-2.5 font-mono text-xs text-muted-foreground">
                    {skill.resources.map(resource => <li key={resource.path}>{resource.path}</li>)}
                  </ul>
                </div>
              ) : null}
              {mutationError && <FieldError>{mutationError}</FieldError>}
              <DialogFooter>
                <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => { setSkill(null); mutation.reset() }}>取消</Button>
                <Button type="submit" size="sm" disabled={busy}>保存 Skill</Button>
              </DialogFooter>
            </FieldGroup>
          </form>
        </DialogContent>
      )}
    </Dialog>

    <ConfirmDialog
      open={Boolean(deleting)}
      title={deleting ? `删除 Skill“${deleting}”？` : ''}
      description="删除后无法在对话中调用此 Skill。"
      confirmLabel="确认删除"
      busy={busy}
      error={deleting ? mutationError : undefined}
      onOpenChange={open => { if (!open) { setDeleting(null); mutation.reset() } }}
      onConfirm={() => {
        if (!deleting) return
        mutation.mutate(async () => {
          await api.DeleteSkill(deleting)
          setDeleting(null)
        })
      }}
    />
  </div>
}
