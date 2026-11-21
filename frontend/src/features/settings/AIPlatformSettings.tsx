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
const newSkill: Pick<Skill, 'markdown' | 'enabled' | 'resources'> = {
  markdown: '---\nname: my-skill\ndescription: 描述何时使用\n---\n',
  enabled: true,
}

const formatAuditTime = (value: string) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'short' }).format(date)
}

function toolOption(name: string, tool?: Tool) {
  const description = tool?.description
  return {
    value: name,
    label: name,
    keywords: description ? [description] : undefined,
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
type ClientDraft = { config: ClientConfig; kind: 'http' | 'stdio'; args: string; editing: boolean }

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
  const [grantEditor, setGrantEditor] = useState<GrantDraft | null>(null)
  const [revokingGrant, setRevokingGrant] = useState<MCPGrant | null>(null)
  const [clientEditor, setClientEditor] = useState<ClientDraft | null>(null)
  const [forgettingClient, setForgettingClient] = useState<string | null>(null)
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
  const busy = mutation.isPending
  const mutationError = mutation.isError
    ? `MCP 设置失败（${mutation.error instanceof Error ? mutation.error.message : '请检查配置'}）`
    : undefined

  const [scopeKind, scopeId = ''] = (grantEditor?.scope ?? 'all').split(':')
  const allowedGrantTools = (server.data?.config.tools ?? []).filter(name =>
    scopeKind === 'all' || scopedArticleTools.has(name) || name === `${scopeKind}.get`,
  )

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
          <SettingRow key={grant.name}>
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
              <Button size="sm" variant="ghost" disabled={busy} onClick={() => { mutation.reset(); setRevokingGrant(grant) }}>撤销</Button>
            </div>
          </SettingRow>
        ))}
      </SettingCard>
    </SettingSection>

    <SettingSection
      title="外部 MCP 连接"
      actions={
        <Button size="sm" variant="outline" disabled={busy} onClick={() => {
          mutation.reset()
          setClientEditor({ config: newClient, kind: 'http', args: '', editing: false })
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
          const expanded = expandedClients.has(item.name)
          const summaryParts = [
            toolCount > 0 ? `${toolCount} 项工具` : '',
            resourceCount > 0 ? `${resourceCount} 项资源` : '',
            promptCount > 0 ? `${promptCount} 项提示词` : '',
          ].filter(Boolean)
          return (
            <div key={item.name} className="space-y-2 px-3.5 py-2.5">
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
                        onClick={() => toggleClientDetails(item.name)}
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
                <div className="flex shrink-0 items-center gap-1">
                  {item.connected
                    ? <Button size="sm" variant="ghost" disabled={busy} onClick={() => mutation.mutate(() => api.DisconnectMCP(item.name))}>断开</Button>
                    : <Button size="sm" variant="ghost" disabled={busy} onClick={() => mutation.mutate(() => api.ConnectMCP(item.config))}>重连</Button>}
                  <Button size="sm" variant="ghost" disabled={busy} onClick={() => {
                    mutation.reset()
                    setClientEditor({
                      config: item.config,
                      kind: item.config.command ? 'stdio' : 'http',
                      args: item.config.args?.join('\n') ?? '',
                      editing: true,
                    })
                  }}>编辑</Button>
                  <Button size="sm" variant="ghost" disabled={busy} onClick={() => { mutation.reset(); setForgettingClient(item.name) }}>移除</Button>
                </div>
              </div>
              {expanded && hasDetails && (
                <div className="space-y-1.5 rounded-lg border border-border bg-muted/30 p-2.5 text-xs">
                  {toolCount > 0 && (
                    <div className="flex flex-wrap items-center gap-1">
                      <span className="mr-1 text-muted-foreground">工具</span>
                      {item.tools!.map(name => <code key={name} className="rounded bg-muted px-1.5 py-0.5 font-mono text-[11px]">{name}</code>)}
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

    {mutationError && !grantEditor && !clientEditor && !revokingGrant && !forgettingClient && <FieldError>{mutationError}</FieldError>}

    <Dialog open={Boolean(grantEditor)} onOpenChange={open => { if (!open && !busy) { setGrantEditor(null); mutation.reset() } }}>
      {grantEditor && (
        <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{grantEditor.editing ? '编辑客户端授权' : '新建客户端授权'}</DialogTitle>
          </DialogHeader>
          <form onSubmit={event => {
            event.preventDefault()
            mutation.mutate(async () => {
              setToken(await api.SaveGrant({
                name: grantEditor.name,
                tools: grantEditor.tools.filter(name => allowedGrantTools.includes(name)),
                scope: { kind: scopeKind === 'all' ? '' : scopeKind, id: scopeId },
              }, false))
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
                  placeholder={grantEditor.tools.length ? `已选 ${grantEditor.tools.length} 项` : '选择工具'}
                  value={grantEditor.tools}
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
                <Button type="submit" size="sm" disabled={busy || grantEditor.tools.length === 0}>生成客户端令牌</Button>
              </DialogFooter>
            </FieldGroup>
          </form>
        </DialogContent>
      )}
    </Dialog>

    <Dialog open={Boolean(clientEditor)} onOpenChange={open => { if (!open && !busy) { setClientEditor(null); mutation.reset() } }}>
      {clientEditor && (
        <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{clientEditor.editing ? '编辑 MCP 连接' : '添加 MCP 连接'}</DialogTitle>
          </DialogHeader>
          <form onSubmit={event => {
            event.preventDefault()
            mutation.mutate(async () => {
              await api.ConnectMCP({
                ...clientEditor.config,
                command: clientEditor.kind === 'stdio' ? clientEditor.config.command : '',
                endpoint: clientEditor.kind === 'http' ? clientEditor.config.endpoint : '',
                bearer: clientEditor.kind === 'http' ? clientEditor.config.bearer : undefined,
                args: clientEditor.args.split('\n').filter(Boolean),
              })
              setClientEditor(null)
            })
          }}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="mcp-client-name">连接名称</FieldLabel>
                <Input
                  id="mcp-client-name"
                  required
                  disabled={busy}
                  value={clientEditor.config.name}
                  onChange={event => setClientEditor({ ...clientEditor, config: { ...clientEditor.config, name: event.target.value } })}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="mcp-client-transport">传输方式</FieldLabel>
                <SearchableSelect
                  id="mcp-client-transport"
                  label="传输方式"
                  className="w-full"
                  value={clientEditor.kind}
                  onValueChange={value => { if (value === 'http' || value === 'stdio') setClientEditor({ ...clientEditor, kind: value }) }}
                  options={[{ value: 'http', label: 'Streamable HTTP' }, { value: 'stdio', label: 'stdio' }]}
                />
              </Field>
              {clientEditor.kind === 'http' ? (
                <>
                  <Field>
                    <FieldLabel htmlFor="mcp-endpoint">MCP 地址</FieldLabel>
                    <Input
                      id="mcp-endpoint"
                      type="url"
                      required
                      value={clientEditor.config.endpoint}
                      onChange={event => setClientEditor({ ...clientEditor, config: { ...clientEditor.config, endpoint: event.target.value } })}
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="mcp-client-token">Bearer 令牌（可选）</FieldLabel>
                    <Input
                      id="mcp-client-token"
                      type="password"
                      autoComplete="off"
                      value={clientEditor.config.bearer ?? ''}
                      onChange={event => setClientEditor({ ...clientEditor, config: { ...clientEditor.config, bearer: event.target.value } })}
                    />
                  </Field>
                </>
              ) : (
                <>
                  <Field>
                    <FieldLabel htmlFor="mcp-command">本地可执行文件</FieldLabel>
                    <Input
                      id="mcp-command"
                      required
                      value={clientEditor.config.command}
                      onChange={event => setClientEditor({ ...clientEditor, config: { ...clientEditor.config, command: event.target.value } })}
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="mcp-args">启动参数（每行一项）</FieldLabel>
                    <Textarea
                      id="mcp-args"
                      className="font-mono text-xs"
                      rows={3}
                      value={clientEditor.args}
                      onChange={event => setClientEditor({ ...clientEditor, args: event.target.value })}
                    />
                  </Field>
                </>
              )}
              {mutationError && <FieldError>{mutationError}</FieldError>}
              <DialogFooter>
                <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => { setClientEditor(null); mutation.reset() }}>取消</Button>
                <Button type="submit" size="sm" disabled={busy}>连接</Button>
              </DialogFooter>
            </FieldGroup>
          </form>
        </DialogContent>
      )}
    </Dialog>

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
          setToken('')
          setRevokingGrant(null)
        })
      }}
    />

    <ConfirmDialog
      open={Boolean(forgettingClient)}
      title={forgettingClient ? `移除连接“${forgettingClient}”？` : ''}
      description="将断开并删除此 MCP 连接配置。"
      confirmLabel="确认移除"
      busy={busy}
      error={forgettingClient ? mutationError : undefined}
      onOpenChange={open => { if (!open) { setForgettingClient(null); mutation.reset() } }}
      onConfirm={() => {
        if (!forgettingClient) return
        mutation.mutate(async () => {
          await api.ForgetMCP(forgettingClient)
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
  const mutationError = mutation.isError
    ? `Skill 操作失败（${mutation.error instanceof Error ? mutation.error.message : '请检查内容'}）`
    : undefined

  return <div className="space-y-6">
    <SettingSection
      title="Skills"
      actions={
        <>
          <Button size="sm" variant="outline" disabled={busy} onClick={() => mutation.mutate(async () => { setSkill(await api.ImportSkill()) })}>
            <AnimatedGlyph meaning="upload" />导入 Skill
          </Button>
          <Button size="sm" variant="outline" disabled={busy} onClick={() => { mutation.reset(); setSkill(newSkill) }}>
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
