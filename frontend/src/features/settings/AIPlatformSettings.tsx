import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api'
import type { ClientConfig, ServerConfig, Skill } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/models'

const platformKey = ['ai-platform']
export const skillsKey = [...platformKey, 'skills']
const newClient: ClientConfig = { name: '', endpoint: '', command: '', args: [] }
const newSkill = { markdown: '---\nname: my-skill\ndescription: 描述何时使用\n---\n', enabled: true }

export function AIPlatformSettings() {
  const cache = useQueryClient()
  const server = useQuery({ queryKey: [...platformKey, 'server'], queryFn: () => api.MCPServer(), retry: false })
  const audit = useQuery({ queryKey: [...platformKey, 'audit'], queryFn: () => api.RecentAudit(), enabled: false, retry: false })
  const tools = useQuery({ queryKey: [...platformKey, 'tools'], queryFn: () => api.ToolList(), retry: false })
  const skills = useQuery({ queryKey: skillsKey, queryFn: () => api.Skills(), retry: false })
  const grants = useQuery({ queryKey: [...platformKey, 'grants'], queryFn: () => api.Grants(), retry: false })
  const clients = useQuery({ queryKey: [...platformKey, 'clients'], queryFn: () => api.MCPClients(), retry: false })
  const [draft, setDraft] = useState<ServerConfig>()
  const [token, setToken] = useState('')
  const [grantName, setGrantName] = useState('')
  const [grantTools, setGrantTools] = useState<string[]>([])
  const [skill, setSkill] = useState<Pick<Skill, 'markdown' | 'enabled'> | null>(null)
  const [client, setClient] = useState(newClient)
  const [args, setArgs] = useState('')
  const [kind, setKind] = useState('http')
  const config = draft ?? server.data?.config
  const mutation = useMutation({
    mutationFn: async (save: () => Promise<unknown>) => { await save() },
    onSuccess: () => cache.invalidateQueries({ queryKey: platformKey }),
  })
  const busy = mutation.isPending
  return <>
    <Separator />
    <section className="space-y-3" aria-label="MCP 服务端">
      <h3>MCP 服务端</h3>
      {server.isError && <Button size="sm" variant="outline" onClick={() => void server.refetch()}>重试读取服务端</Button>}
      {server.data?.url && <output className="select-text break-all text-sm">{server.data.url}</output>}
      {config && <form className="space-y-3" onSubmit={event => {
        event.preventDefault()
        mutation.mutate(async () => { await api.SaveMCPServer(config); setDraft(undefined) })
      }}>
        <Field orientation="horizontal"><FieldLabel htmlFor="mcp-serve">启用 MCP 服务</FieldLabel><Switch id="mcp-serve" checked={config.enabled} onCheckedChange={enabled => setDraft({ ...config, enabled })} /></Field>
        <Field orientation="horizontal"><FieldLabel htmlFor="mcp-external">允许本机外接入</FieldLabel><Switch id="mcp-external" checked={config.external} onCheckedChange={external => setDraft({ ...config, external })} /></Field>
        {config.external && <Field><FieldLabel htmlFor="mcp-address">监听 IP</FieldLabel><Input id="mcp-address" value={config.address} onChange={event => setDraft({ ...config, address: event.target.value })} /></Field>}
        <Field><FieldLabel htmlFor="mcp-port">端口</FieldLabel><Input id="mcp-port" type="number" min={1024} max={65535} required value={config.port || ''} onChange={event => setDraft({ ...config, port: Number(event.target.value) })} /></Field>
        <Field orientation="horizontal"><FieldLabel htmlFor="mcp-tls">启用 HTTPS</FieldLabel><Switch id="mcp-tls" checked={config.tls} onCheckedChange={tls => setDraft({ ...config, tls })} /></Field>
        {config.tls && <>
          <Field><FieldLabel htmlFor="mcp-cert">证书路径</FieldLabel><Input id="mcp-cert" required value={config.certificate} onChange={event => setDraft({ ...config, certificate: event.target.value })} /></Field>
          <Field><FieldLabel htmlFor="mcp-key">私钥路径</FieldLabel><Input id="mcp-key" required value={config.privateKey} onChange={event => setDraft({ ...config, privateKey: event.target.value })} /></Field>
        </>}
        <Field><FieldLabel htmlFor="mcp-tools">公开工具</FieldLabel>
          <SearchableSelect id="mcp-tools" label="公开工具" value={config.tools ?? []} placeholder={config.tools?.length ? `已选 ${config.tools.length} 项` : '选择工具'}
            options={(tools.data ?? []).map(tool => ({ value: tool.name, label: tool.name, keywords: [tool.description] }))}
            onValueChange={name => {
              const selected = config.tools ?? []
              setDraft({ ...config, tools: selected.includes(name) ? selected.filter(item => item !== name) : [...selected, name] })
            }} />
        </Field>
        <div className="flex gap-2">
          <Button size="sm" type="submit" disabled={busy || !draft}>保存服务端</Button>
          <Button size="sm" type="button" variant="outline" disabled={busy} onClick={() => mutation.mutate(async () => setToken(await api.RotateMCPToken()))}>生成或更换令牌</Button>
        </div>
        {token && <Field><FieldLabel htmlFor="mcp-token">访问令牌</FieldLabel><Input id="mcp-token" readOnly value={token} autoComplete="off" /></Field>}
      </form>}
      <div className="space-y-2">
        <h4>客户端授权</h4>
        {(grants.data ?? []).map(grant => <div key={grant.name} className="flex items-center gap-2">
          <span className="mr-auto">{grant.name} · {grant.tools?.length ?? 0} 项工具</span>
          <Button size="sm" variant="ghost" onClick={() => { setGrantName(grant.name); setGrantTools(grant.tools ?? []) }}>编辑</Button>
          <Button size="sm" variant="ghost" disabled={busy} onClick={() => mutation.mutate(async () => { await api.SaveGrant(grant, true); setToken('') })}>撤销</Button>
        </div>)}
        <form className="space-y-2" onSubmit={event => {
          event.preventDefault()
          mutation.mutate(async () => {
            setToken(await api.SaveGrant({ name: grantName, tools: grantTools }, false))
            setGrantName('')
            setGrantTools([])
          })
        }}>
          <Field><FieldLabel htmlFor="mcp-grant-name">客户端名称</FieldLabel>
            <Input id="mcp-grant-name" required maxLength={32} value={grantName} onChange={event => setGrantName(event.target.value)} />
          </Field>
          <Field><FieldLabel htmlFor="mcp-grant-tools">允许的工具</FieldLabel>
            <SearchableSelect id="mcp-grant-tools" label="允许的工具" placeholder={grantTools.length ? `已选 ${grantTools.length} 项` : '选择工具'}
              value={grantTools} options={(server.data?.config.tools ?? []).map(name => ({ value: name, label: name }))}
              onValueChange={name => setGrantTools(current => current.includes(name) ? current.filter(item => item !== name) : [...current, name])} />
          </Field>
          <Button type="submit" size="sm" variant="outline" disabled={busy || grantTools.length === 0}>生成客户端令牌</Button>
        </form>
      </div>
    </section>
    <Separator />
    <section className="space-y-3" aria-label="MCP 连接">
      <h3>MCP 连接</h3>
      {(clients.data ?? []).map(item => <div key={item.name} className="flex flex-wrap items-center gap-2">
        <span className="mr-auto">{item.name} · {item.connected ? `${item.tools?.length ?? 0} 项工具` : '已断开'}</span>
        {item.connected
          ? <Button size="sm" variant="ghost" disabled={busy} onClick={() => mutation.mutate(() => api.DisconnectMCP(item.name))}>断开</Button>
          : <Button size="sm" variant="ghost" disabled={busy} onClick={() => mutation.mutate(() => api.ConnectMCP(item.config))}>重连</Button>}
        <Button size="sm" variant="ghost" disabled={busy} onClick={() => { setClient(item.config); setKind(item.config.command ? 'stdio' : 'http'); setArgs(item.config.args?.join('\n') ?? '') }}>编辑</Button>
        <Button size="sm" variant="ghost" disabled={busy} onClick={() => mutation.mutate(() => api.ForgetMCP(item.name))}>移除</Button>
        {item.tools?.length ? <span className="w-full break-all text-xs text-muted-foreground">{item.tools.join(' · ')}</span> : null}
      </div>)}
      <form className="space-y-2" onSubmit={event => {
        event.preventDefault()
        mutation.mutate(async () => {
          await api.ConnectMCP({ ...client, command: kind === 'stdio' ? client.command : '', endpoint: kind === 'http' ? client.endpoint : '', bearer: kind === 'http' ? client.bearer : undefined, args: args.split('\n').filter(Boolean) })
          setClient(newClient)
          setArgs('')
        })
      }}>
        <Field><FieldLabel htmlFor="mcp-client-name">连接名称</FieldLabel><Input id="mcp-client-name" required value={client.name} onChange={event => setClient({ ...client, name: event.target.value })} /></Field>
        <Field><FieldLabel htmlFor="mcp-client-transport">传输方式</FieldLabel><SearchableSelect id="mcp-client-transport" label="传输方式" value={kind} onValueChange={setKind} options={[{ value: 'http', label: 'Streamable HTTP' }, { value: 'stdio', label: 'stdio' }]} /></Field>
        {kind === 'http'
          ? <Field><FieldLabel htmlFor="mcp-endpoint">MCP 地址</FieldLabel><Input id="mcp-endpoint" type="url" required value={client.endpoint} onChange={event => setClient({ ...client, endpoint: event.target.value })} /></Field>
          : <><Field><FieldLabel htmlFor="mcp-command">本地可执行文件</FieldLabel><Input id="mcp-command" required value={client.command} onChange={event => setClient({ ...client, command: event.target.value })} /></Field>
            <Field><FieldLabel htmlFor="mcp-args">启动参数（每行一项）</FieldLabel><Textarea id="mcp-args" value={args} onChange={event => setArgs(event.target.value)} /></Field></>}
        {kind === 'http' && <Field><FieldLabel htmlFor="mcp-client-token">Bearer 令牌（可选）</FieldLabel>
          <Input id="mcp-client-token" type="password" autoComplete="off" value={client.bearer ?? ''} onChange={event => setClient({ ...client, bearer: event.target.value })} />
        </Field>}
        <Button size="sm" type="submit" disabled={busy}>连接</Button>
      </form>
    </section>
    <Separator />
    <section className="space-y-3" aria-label="Skills">
      <div className="flex flex-wrap items-center gap-2"><h3 className="mr-auto">Skills</h3>
        <Button size="sm" variant="outline" disabled={busy} onClick={() => mutation.mutate(async () => { setSkill(await api.ImportSkill()) })}>导入 SKILL.md</Button>
        <Button size="sm" variant="outline" onClick={() => setSkill(newSkill)}>添加 Skill</Button></div>
      {(skills.data ?? []).map(item => <div key={item.name} className="flex flex-wrap items-center gap-2">
        <span className="mr-auto" title={item.description}>{item.name}</span>
        <Switch aria-label={`启用 ${item.name}`} checked={item.enabled} disabled={busy} onCheckedChange={enabled => mutation.mutate(() => api.SaveSkill(item.markdown, enabled))} />
        <Button size="sm" variant="ghost" onClick={() => setSkill(item)}>编辑</Button>
        <Button size="sm" variant="ghost" disabled={busy} onClick={() => mutation.mutate(() => api.DeleteSkill(item.name))}>删除</Button>
      </div>)}
      {skill && <form className="space-y-2" onSubmit={event => {
        event.preventDefault()
        mutation.mutate(async () => { await api.SaveSkill(skill.markdown, skill.enabled); setSkill(null) })
      }}>
        <Field><FieldLabel htmlFor="ai-skill-markdown">SKILL.md</FieldLabel><Textarea id="ai-skill-markdown" rows={8} value={skill.markdown} required onChange={event => setSkill({ ...skill, markdown: event.target.value })} /></Field>
        <div className="flex gap-2"><Button type="submit" size="sm" disabled={busy}>保存 Skill</Button><Button type="button" size="sm" variant="ghost" onClick={() => setSkill(null)}>取消</Button></div>
      </form>}
    </section>
    <Separator />
    <section className="space-y-2" aria-label="工具记录">
      <div className="flex items-center justify-between"><h3>工具记录</h3><Button size="sm" variant="outline" onClick={() => void audit.refetch()}>查看最近记录</Button></div>
      {audit.data?.length === 0 && <p className="text-sm text-muted-foreground">暂无工具记录</p>}
      {audit.data?.map(entry => <div key={`${entry.startedAt}${entry.tool}`} className="flex flex-wrap gap-2 text-sm">
        <span>{entry.tool}</span><span className="text-muted-foreground">{entry.principal}</span>
        <span className="ml-auto">{entry.status === 'complete' ? '已返回' : '未完成'}</span>
      </div>)}
    </section>
    {mutation.isError && <p role="alert" className="text-sm">AI 平台设置失败（{mutation.error instanceof Error ? mutation.error.message : '请检查配置'}）</p>}
  </>
}
