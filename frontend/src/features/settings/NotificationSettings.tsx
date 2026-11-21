import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { navigationAPI, navigationQueryKey } from '@/lib/navigation-api'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/api'
import type { Channel, ChannelInput, Rule, Task, WeChatBinding } from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/models'

const key = ['notifications']
const platforms = [
  { value: 'telegram', label: 'Telegram' }, { value: 'discord', label: 'Discord' }, { value: 'slack', label: 'Slack' },
  { value: 'dingtalk', label: '钉钉' }, { value: 'feishu', label: '飞书' }, { value: 'lark', label: 'Lark' },
  { value: 'wechat', label: '微信' }, { value: 'wecom', label: '企业微信' }, { value: 'qq', label: 'QQ' },
]
const appPlatforms = ['discord', 'slack', 'dingtalk', 'feishu', 'lark', 'wecom']
const recipientFields: Record<string, { label: string; example: string }> = {
  telegram: { label: 'Telegram Chat ID', example: '123456789\\n-100123456789' },
  discord: { label: 'Discord 用户 ID', example: '123456789012345678' },
  slack: { label: 'Slack 工作区成员 ID', example: 'U012ABCDEF' },
  dingtalk: { label: '钉钉企业成员 UserID', example: 'user123' },
  feishu: { label: '飞书成员 ID', example: 'ou_xxxxx\nuser_id:employee123' },
  lark: { label: 'Lark 成员 ID', example: 'ou_xxxxx\nuser_id:employee123' },
  wecom: { label: '企业微信成员 UserID', example: 'zhangsan' },
  qq: { label: 'QQ 私聊 OpenID、群 OpenID 或子频道 ID', example: 'c2c:xxxxx\\ngroup:xxxxx\\nchannel:12345' },
}
const actions = [{ value: 'reminder', label: '提醒' }, { value: 'articles', label: '文章列表' }, { value: 'summary', label: 'AI 文章摘要' }]
const frequencies = [{ value: 'once', label: '一次' }, { value: 'daily', label: '每天' }, { value: 'weekly', label: '指定星期' }]
const weekdays = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'].map((label, index) => ({ value: String(index), label }))
const defaultZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
const zones = [...new Set([defaultZone, 'UTC', ...(Intl.supportedValuesOf?.('timeZone') ?? [])])]
  .map(value => ({ value, label: value }))
const time = (value: number) => value > 0 ? new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'short' }).format(value) : ''
const statusLabels: Record<string, string> = {
  accepted: '已接收', unknown: '结果未知', pending: '待发送', sending: '发送中', failed: '失败', cancelled: '已取消',
  complete: '已完成', running: '运行中', interrupted: '中断',
}
const blankChannel: ChannelInput = {
  id: '', name: '', platform: 'telegram', target: '', mode: '', targets: [], recipients: [], address: '', signing: '', context: '', media_app_id: '', media_secret: '', enabled: true, verified: false, revision: 0,
}
const blankRule: Rule = {
  id: '', name: '', mode: 'instant', categories: [], channels: [], body: false, image: false,
  enabled: true, revision: 0, next_at: 0, timezone: defaultZone, send_at: '20:00', window_start: '00:00', window_end: '20:00',
}
const blankTask: Task = {
  id: '', name: '', action: 'reminder', lookback_days: 1, text: '', categories: [], channels: [], enabled: true, revision: 0, next_at: 0,
  calendar: { kind: 'daily', timezone: defaultZone, local_time: '20:00' },
}

function toggle<T>(items: T[] | null | undefined, item: T) {
  return items?.includes(item) ? items.filter(value => value !== item) : [...(items ?? []), item]
}

function TextField({ id, label, value, onChange, type = 'text', disabled }: {
  id: string; label: string; value: string; onChange(value: string): void; type?: string; disabled?: boolean
}) {
  return <Field>
    <FieldLabel htmlFor={id}>{label}</FieldLabel>
    <Input id={id} value={value} type={type} disabled={disabled} onChange={event => onChange(event.target.value)} />
  </Field>
}

function FlagField({ id, label, value, onChange }: { id: string; label: string; value: boolean; onChange(value: boolean): void }) {
  return <Field orientation="horizontal"><FieldLabel htmlFor={id}>{label}</FieldLabel><Switch id={id} checked={value} onCheckedChange={onChange} /></Field>
}

function Selection({ id, label, options, value, onChange, placeholder, disabled }: {
  id: string; label: string; options: { value: string; label: string }[]; value: string | string[]
  onChange(value: string): void; placeholder?: string; disabled?: boolean
}) {
  return <Field>
    <FieldLabel htmlFor={id}>{label}</FieldLabel>
    <SearchableSelect id={id} label={label} options={options} value={value} onValueChange={onChange}
      className="w-full" placeholder={placeholder} disabled={disabled} />
  </Field>
}

export function NotificationSettings() {
  const client = useQueryClient()
  const channels = useQuery({ queryKey: [...key, 'channels'], queryFn: () => api.ListChannels(), retry: false })
  const rules = useQuery({ queryKey: [...key, 'rules'], queryFn: () => api.ListRules(), retry: false })
  const tasks = useQuery({ queryKey: [...key, 'tasks'], queryFn: () => api.ListTasks(), retry: false })
  const navigation = useQuery({ queryKey: navigationQueryKey, queryFn: () => navigationAPI.list() })
  const [channel, setChannel] = useState<ChannelInput>()
  const [recipientText, setRecipientText] = useState('')
  const [rule, setRule] = useState<Rule>()
  const [task, setTask] = useState<Task>()
  const preview = useQuery({ queryKey: [...key, 'rule-preview', rule?.timezone, rule?.send_at, rule?.window_start, rule?.window_end],
    queryFn: () => api.PreviewRule(rule!), enabled: rule?.mode === 'daily', retry: false })
  const [wechat, setWeChat] = useState<WeChatBinding>()
  const [pin, setPin] = useState('')
  const [verifiedPin, setVerifiedPin] = useState('')
  const wechatPoll = useQuery({
    queryKey: [...key, 'wechat', wechat?.id, verifiedPin],
    queryFn: () => api.PollWeChatBinding(wechat!.id, verifiedPin, channel!.name),
    enabled: Boolean(wechat?.id) && channel?.platform === 'wechat' && !channel.id,
    retry: false,
    refetchInterval: query => ['connected', 'expired', 'verify_code_blocked', 'binded_redirect'].includes(query.state.data?.status ?? '') ||
      query.state.data?.status.includes('need_verifycode') ? false : 1200,
  })
  useEffect(() => {
    if (wechatPoll.data?.status === 'connected') void client.invalidateQueries({ queryKey: [...key, 'channels'] })
  }, [client, wechatPoll.data?.status])

  const [showHistory, setShowHistory] = useState(false)
  const [openedTask, setOpenedTask] = useState('')
  const [confirmation, setConfirmation] = useState<{ title: string; description: string; label: string; run: () => Promise<unknown> }>()
  const deliveries = useQuery({ queryKey: [...key, 'deliveries'], queryFn: () => api.ListDeliveries(), enabled: showHistory, refetchInterval: showHistory ? 5000 : false })
  const runs = useQuery({ queryKey: [...key, 'runs', openedTask], queryFn: () => api.ListRuns(openedTask), enabled: Boolean(openedTask) })
  const mutation = useMutation({ mutationFn: (work: () => Promise<unknown>) => work(),
    onSettled: () => client.invalidateQueries({ queryKey: key, predicate: query => query.queryKey[1] !== 'wechat' }) })
  const channelOptions = (channels.data ?? []).filter(value => value.enabled && value.verified).map(value => ({ label: value.name, value: value.id }))
  const categoryOptions = (navigation.data?.categories ?? []).map(value => ({ label: value.name, value: value.id }))
  function askDelete(name: string, run: () => Promise<unknown>) {
    setConfirmation({ title: `删除 ${name}？`, description: '未开始的投递将取消，已发送的消息保留。', label: '删除', run })
  }
  const busy = mutation.isPending
  const bindingStatus = wechatPoll.data?.status ?? wechat?.status
  const bindingMode = channel?.platform === 'wechat' && !channel.id
  const appMode = channel?.mode === 'app'
  const webhookMode = Boolean(channel && appPlatforms.includes(channel.platform) && !appMode)
  const savedRecipients = channels.data?.find(value => value.id === channel?.id)?.recipients ?? []
  const wechatOptions = channel?.platform === 'wechat' ? [
    { value: channel.target, label: '当前绑定私聊' },
    ...(channels.data ?? []).filter(value => value.id !== channel.id && value.platform === 'wechat' && value.enabled)
      .map(value => ({ value: `binding:${value.id}`, label: value.name })),
  ] : []
  function recipientLabel(value: string) {
    if (!value) return '已绑定的群或频道'
    return wechatOptions.find(item => item.value === value)?.label ?? value
  }

  const setChannelField = <K extends keyof ChannelInput,>(field: K, value: ChannelInput[K]) => setChannel(previous => previous && { ...previous, [field]: value })
  const setRuleField = <K extends keyof Rule,>(field: K, value: Rule[K]) => setRule(previous => previous && { ...previous, [field]: value })
  const setTaskField = <K extends keyof Task,>(field: K, value: Task[K]) => setTask(previous => previous && { ...previous, [field]: value })
  const setCalendar = (fields: Partial<Task['calendar']>) => setTask(previous => previous && { ...previous, calendar: { ...previous.calendar, ...fields } })
  function editChannel(value: Channel) {
    const targets = value.recipients?.map(recipient => recipient.target) ?? (value.target ? [value.target] : [])
    setChannel({ ...value, targets, address: '', signing: '', context: '', media_app_id: '', media_secret: '' })
    setRecipientText(targets.join('\n'))
  }
  function saveTask() {
    if (!task) return
    const onceAt = task.calendar.once_at
    mutation.mutate(async () => {
      await api.SaveTask({ ...task, calendar: { ...task.calendar, once_at: onceAt } })
      setTask(undefined)
    })
  }

  return <FieldGroup>
    <Separator />
    <section className="space-y-3" aria-label="通知渠道">
      <div className="flex items-center justify-between gap-2"><h2 className="font-medium">通知渠道</h2><Button size="sm" variant="outline" disabled={busy} onClick={() => { setRecipientText(''); setChannel(blankChannel) }}>添加渠道</Button></div>
      {(channels.data ?? []).map(value => <div className="flex flex-wrap items-center justify-between gap-2" key={value.id}>
        <div><p>{value.name}</p><p className="text-xs text-muted-foreground">{platforms.find(item => item.value === value.platform)?.label} · {value.verified ? value.enabled ? '已启用' : '已停用' : '待测试'}</p></div>
        <div className="flex gap-1"><Button size="sm" variant="ghost" disabled={busy} onClick={() => editChannel(value)}>编辑</Button>
          <Button size="sm" variant="ghost" disabled={busy} onClick={() => mutation.mutate(() => api.TestChannel(value.id))}>{(value.recipients?.length ?? 0) > 1 ? '测试全部收件目标' : '发送测试'}</Button>
          <Button size="sm" variant="ghost" disabled={busy} onClick={() => askDelete(value.name, () => api.DeleteChannel(value.id, value.revision))}>删除</Button></div>
      </div>)}
      {channel && <FieldGroup className="rounded-lg border p-3">
        <TextField id="channel-name" label="渠道名称" value={channel.name} onChange={value => setChannelField('name', value)} />
        <Selection id="channel-platform" label="平台" options={platforms} value={channel.platform} disabled={Boolean(channel.id)} onChange={value => {
          setChannel({ ...blankChannel, platform: value, name: channel.name }); setRecipientText('')
        }} />
        {appPlatforms.includes(channel.platform) && <Selection id="channel-mode" label="接入方式"
          options={[{ value: 'webhook', label: '群或频道 Webhook' }, { value: 'app', label: '应用机器人私发' }]}
          value={appMode ? 'app' : 'webhook'} disabled={Boolean(channel.id)} onChange={value => {
            setChannel({ ...blankChannel, platform: channel.platform, name: channel.name, mode: value === 'app' ? 'app' : '' })
            setRecipientText('')
          }} />}
        {!bindingMode && <fieldset className="min-w-0 space-y-2">
          <legend className="font-medium">收件白名单</legend>
          {webhookMode ? <p className="text-sm text-muted-foreground">已绑定的整个群或频道</p>
            : channel.platform === 'wechat'
              ? <Selection id="wechat-recipients" label="已绑定的微信私聊" options={wechatOptions}
                value={channel.targets ?? []} onChange={value => setChannelField('targets', toggle(channel.targets, value))} />
              : <Field><FieldLabel htmlFor="channel-recipients">{recipientFields[channel.platform]?.label}</FieldLabel>
                <Textarea id="channel-recipients" value={recipientText} onChange={event => setRecipientText(event.target.value)}
                  placeholder={recipientFields[channel.platform]?.example} rows={3} /></Field>}
          {!webhookMode && savedRecipients.map(recipient => <div className="flex items-center justify-between gap-2 text-sm" key={recipient.id}>
            <span className="min-w-0 break-all">{recipientLabel(recipient.target)} · {recipient.verified ? '已测试' : '待测试'}</span>
            <Button variant="ghost" size="sm" disabled={busy}
              onClick={() => mutation.mutate(() => api.TestRecipient(channel.id, recipient.id))}>发送测试</Button>
          </div>)}
        </fieldset>}
        {channel.platform !== 'wechat' && <TextField id="channel-secret"
          label={webhookMode ? 'Webhook URL' : channel.platform === 'telegram' || channel.platform === 'discord' ? 'Bot Token'
            : channel.platform === 'slack' ? 'Slack Bot Token' : channel.platform === 'wecom' ? 'CorpID' : channel.platform === 'dingtalk' ? 'AppKey' : 'AppID'}
          value={channel.address} type="password" onChange={value => setChannelField('address', value)} />}
        {(['qq', 'dingtalk', 'feishu', 'lark', 'wecom'].includes(channel.platform) && (channel.platform !== 'wecom' || appMode)) &&
          <TextField id="channel-signing" label={webhookMode ? '加签密钥（可选）' : channel.platform === 'wecom' ? '应用 Secret' : 'AppSecret'}
            type="password" value={channel.signing} onChange={value => setChannelField('signing', value)} />}
        {appMode && ['dingtalk', 'wecom'].includes(channel.platform) &&
          <TextField id="channel-context" label={channel.platform === 'wecom' ? '应用 AgentID' : 'RobotCode（可选）'}
            value={channel.context} onChange={value => setChannelField('context', value)} />}
        {webhookMode && ['feishu', 'lark'].includes(channel.platform) && <>
          <TextField id="channel-media-app" label="图片上传 AppID（可选）" value={channel.media_app_id} onChange={value => setChannelField('media_app_id', value)} />
          <TextField id="channel-media-secret" label="图片上传 AppSecret" type="password" value={channel.media_secret} onChange={value => setChannelField('media_secret', value)} />
        </>}
        {bindingMode && <div className="space-y-2">
          {wechat?.image && !['connected', 'expired', 'verify_code_blocked', 'binded_redirect'].includes(bindingStatus ?? '') &&
            <img src={wechat.image} width={192} height={192} alt="微信绑定二维码" />}
          {bindingStatus === 'wait' && <p>用微信扫描二维码</p>}
          {bindingStatus === 'scaned' && <p>请在微信确认绑定</p>}
          {bindingStatus === 'await_message' && <p>绑定后，在微信私聊里给机器人发一条消息</p>}
          {bindingStatus === 'connected' && <p role="status">微信已绑定，可以发送测试</p>}
          {['expired', 'verify_code_blocked', 'binded_redirect'].includes(bindingStatus ?? '') && <p role="alert">绑定未完成，请重新扫码</p>}
          {bindingStatus?.includes('need_verifycode') && <div className="flex items-end gap-2">
            <TextField id="wechat-pin" label="微信验证码" value={pin} onChange={setPin} />
            <Button onClick={() => setVerifiedPin(pin)}>确认验证码</Button>
          </div>}
          {wechatPoll.isError && <p role="alert">无法完成微信绑定，请重试</p>}
        </div>}

        <FlagField id="channel-enable" label="启用渠道" value={channel.enabled} onChange={value => setChannelField('enabled', value)} />
        <div className="flex justify-end gap-2"><Button variant="ghost" onClick={() => {
          if (wechat?.id && bindingStatus !== 'connected') void api.CancelWeChatBinding(wechat.id)
          setWeChat(undefined); setPin(''); setVerifiedPin(''); setChannel(undefined)
        }}>{bindingStatus === 'connected' ? '完成' : '取消'}</Button>
          {bindingMode ? <Button disabled={busy || !channel.name.trim() || bindingStatus === 'connected'} onClick={() => mutation.mutate(async () => {
            setPin(''); setVerifiedPin(''); setWeChat(await api.StartWeChatBinding())
          })}>{wechat ? '重新生成二维码' : '生成微信二维码'}</Button>
            : <Button disabled={busy} onClick={() => mutation.mutate(async () => { const targets = webhookMode ? [] : channel.platform === 'wechat' ? channel.targets : recipientText.split(/[\s,，]+/u).filter(Boolean)
              await api.SaveChannel({ ...channel, targets }); setChannel(undefined) })}>保存渠道</Button>}</div>
      </FieldGroup>}
    </section>
    <Separator />
    <section className="space-y-3" aria-label="文章推送规则">
      <div className="flex items-center justify-between gap-2"><h2 className="font-medium">文章推送规则</h2><Button size="sm" variant="outline" disabled={!channelOptions.length || busy} onClick={() => setRule(blankRule)}>添加规则</Button></div>
      {(rules.data ?? []).map(value => <div className="flex items-center justify-between gap-2" key={value.id}>
        <div><p>{value.name}</p><p className="text-xs text-muted-foreground">{value.enabled ? value.mode === 'daily' ? time(value.next_at) : '逐条推送' : '已暂停'}</p></div>
        <div className="flex gap-1"><Button size="sm" variant="ghost" onClick={() => setRule(value)}>编辑</Button>
          <Button size="sm" variant="ghost" disabled={busy} onClick={() => askDelete(value.name, () => api.DeleteRule(value.id, value.revision))}>删除</Button></div>
      </div>)}
      {rule && <FieldGroup className="rounded-lg border p-3">
        <TextField id="notification-rule-name" label="规则名称" value={rule.name} onChange={value => setRuleField('name', value)} />
        <Selection id="notification-rule-channels" label="接收渠道" options={channelOptions} value={rule.channels ?? []} onChange={value => setRuleField('channels', toggle(rule.channels, value))} />
        <Selection id="notification-rule-categories" label="分类" options={categoryOptions} value={rule.categories ?? []} placeholder="全部分类" onChange={value => setRuleField('categories', toggle(rule.categories, value))} />
        <Selection id="notification-rule-mode" label="推送方式" options={[{ value: 'instant', label: '逐条推送' }, { value: 'daily', label: '每日汇总' }]} value={rule.mode} onChange={value => setRuleField('mode', value)} />
        {rule.mode === 'daily' && <>
          <Selection id="notification-rule-timezone" label="时区" value={rule.timezone} options={zones} onChange={value => setRuleField('timezone', value)} />
          <div className="grid grid-cols-3 gap-2">
            <TextField id="notification-rule-start" label="收录开始" type="time" value={rule.window_start} onChange={value => setRuleField('window_start', value)} />
            <TextField id="notification-rule-end" label="收录结束" type="time" value={rule.window_end} onChange={value => setRuleField('window_end', value)} />
            <TextField id="notification-rule-at" label="发送时间" type="time" value={rule.send_at} onChange={value => setRuleField('send_at', value)} />
          </div>
          {preview.data && <p className="text-sm text-muted-foreground">下次发送 {time(preview.data.next_at)} · 收录 {time(preview.data.start)} 至 {time(preview.data.end)}</p>}
        </>}
        <FlagField id="notification-rule-body" label="附带正文" value={rule.body} onChange={value => setRuleField('body', value)} />
        <FlagField id="notification-rule-image" label="附带首图" value={rule.image} onChange={value => setRuleField('image', value)} />
        <FlagField id="notification-rule-enabled" label="启用规则" value={rule.enabled} onChange={value => setRuleField('enabled', value)} />
        <div className="flex justify-end gap-2"><Button variant="ghost" onClick={() => setRule(undefined)}>取消</Button>
          <Button disabled={busy} onClick={() => mutation.mutate(async () => { await api.SaveRule(rule); setRule(undefined) })}>保存规则</Button></div>
      </FieldGroup>}
    </section>
    <Separator />
    <section className="space-y-3" aria-label="定时任务">
      <div className="flex items-center justify-between gap-2"><h2 className="font-medium">定时任务</h2><Button size="sm" variant="outline" disabled={busy} onClick={() => setTask(blankTask)}>添加任务</Button></div>
      {(tasks.data ?? []).map(value => <div key={value.id} className="space-y-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div><p>{value.name}</p><p className="text-xs text-muted-foreground">{value.enabled ? time(value.next_at) || '已运行' : '已暂停'}</p></div>
          <div className="flex gap-1"><Button size="sm" variant="ghost" onClick={() => setTask(value)}>编辑</Button>
            <Button size="sm" variant="ghost" aria-expanded={openedTask === value.id} onClick={() => setOpenedTask(openedTask === value.id ? '' : value.id)}>运行记录</Button>
            <Button size="sm" variant="ghost" disabled={busy} onClick={() => askDelete(value.name, () => api.DeleteTask(value.id, value.revision))}>删除</Button></div>
        </div>
        {openedTask === value.id && <div className="space-y-2 rounded-lg border p-3 text-sm">
          {runs.isError && <p role="alert">暂时无法读取运行记录</p>}
          {runs.data?.length === 0 && <p className="text-muted-foreground">还没有运行记录</p>}
          {runs.data?.map(run => <div key={run.occurrence}><p>{time(run.occurrence)} · {statusLabels[run.state] ?? run.state}</p>
            {run.result && <p className="whitespace-pre-wrap break-words text-muted-foreground">{run.result}</p>}</div>)}
        </div>}
      </div>)}
      {task && <FieldGroup className="rounded-lg border p-3">
        <TextField id="notification-task-name" label="任务名称" value={task.name} onChange={value => setTaskField('name', value)} />
        <Selection id="notification-task-action" label="任务类型" value={task.action} options={actions} onChange={value => setTaskField('action', value)} />
        <Selection id="notification-task-frequency" label="重复方式" value={task.calendar.kind} options={frequencies} onChange={value => setCalendar({ kind: value })} />
        <Selection id="notification-task-timezone" label="时区" value={task.calendar.timezone} options={zones} onChange={value => setCalendar({ timezone: value })} />
        {task.calendar.kind === 'once'
          ? <TextField id="notification-task-once" label="执行时间" type="datetime-local" value={task.calendar.once_at?.slice(0, 16) ?? ''} onChange={value => setCalendar({ once_at: value })} />
          : <TextField id="notification-task-at" label="执行时间" type="time" value={task.calendar.local_time} onChange={value => setCalendar({ local_time: value })} />}
        {task.calendar.kind === 'weekly' && <Selection id="notification-task-days" label="星期" value={(task.calendar.weekdays ?? []).map(String)} options={weekdays} onChange={value => setCalendar({ weekdays: toggle(task.calendar.weekdays, Number(value)) })} />}
        {task.action !== 'reminder' && <>
          <Selection id="notification-task-categories" label="分类" value={task.categories ?? []} options={categoryOptions} placeholder="全部分类" onChange={value => setTaskField('categories', toggle(task.categories, value))} />
          <TextField id="notification-task-lookback" label="回看天数（1 至 30 天）" type="number" value={String(task.lookback_days || 1)} onChange={value => setTaskField('lookback_days', Number(value))} />
        </>}
        <Selection id="notification-task-channels" label="接收渠道" value={task.channels ?? []} options={channelOptions} placeholder="仅应用内" onChange={value => setTaskField('channels', toggle(task.channels, value))} />
        <Field><FieldLabel htmlFor="notification-task-text">{task.action === 'summary' ? '摘要要求' : '任务正文'}</FieldLabel>
          <Textarea id="notification-task-text" value={task.text} onChange={event => setTaskField('text', event.target.value)} /></Field>
        <FlagField id="notification-task-enabled" label="启用任务" value={task.enabled} onChange={value => setTaskField('enabled', value)} />
        <div className="flex justify-end gap-2"><Button variant="ghost" onClick={() => setTask(undefined)}>取消</Button>
          <Button disabled={busy} onClick={saveTask}>保存任务</Button></div>
      </FieldGroup>}
    </section>
    <Separator />
    <section aria-label="投递历史" className="space-y-2">
      <Button size="sm" variant="outline" aria-expanded={showHistory} onClick={() => setShowHistory(value => !value)}>投递历史</Button>
      {showHistory && deliveries.isError && <p role="alert">暂时无法读取投递历史</p>}
      {showHistory && deliveries.data?.length === 0 && <p className="text-sm text-muted-foreground">还没有投递记录</p>}
      {showHistory && (deliveries.data ?? []).map(value => <div className="flex justify-between gap-2 text-sm" key={value.id}>
        <span>{channels.data?.find(item => item.id === value.channel_id)?.name ?? '通知渠道'}{value.recipient_target && (' · ' + value.recipient_target)}</span>
        <span className="flex items-center gap-2 text-muted-foreground">{time(value.created_at)} · {statusLabels[value.state] ?? value.state}
          {['failed', 'unknown'].includes(value.state) && <Button size="sm" variant="ghost" onClick={() => setConfirmation({
            title: '重新投递？', label: '重新投递', run: () => api.RetryDelivery(value.id),
            description: value.state === 'unknown' ? '平台可能已接收原消息，重发可能产生重复。' : '使用原接收渠道重新投递。',
          })}>重发</Button>}</span>
      </div>)}
    </section>
    <Dialog open={Boolean(confirmation)} onOpenChange={open => { if (!open) setConfirmation(undefined) }}>
      {confirmation && <DialogContent>
        <DialogHeader><DialogTitle>{confirmation.title}</DialogTitle><DialogDescription>{confirmation.description}</DialogDescription></DialogHeader>
        <DialogFooter><Button variant="outline" onClick={() => setConfirmation(undefined)}>取消</Button>
          <Button variant={confirmation.label === '删除' ? 'destructive' : 'default'} disabled={busy}
            onClick={() => mutation.mutate(async () => { await confirmation.run(); setConfirmation(undefined) })}>{confirmation.label}</Button></DialogFooter>
        {mutation.error && <p role="alert" className="text-destructive">{mutation.error.message}</p>}
      </DialogContent>}
    </Dialog>
    {mutation.error && !confirmation && <p role="alert" className="text-destructive">{mutation.error.message}</p>}
    {(channels.isError || rules.isError || tasks.isError) && <p role="alert" className="text-destructive">暂时无法读取通知配置</p>}
  </FieldGroup>
}
