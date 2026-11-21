import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Textarea } from '@/components/ui/textarea'
import { defaultZone, FlagField, Selection, TextField, time, toggle, zones } from '@/features/notifications/notification-fields'
import { navigationAPI, navigationQueryKey } from '@/lib/navigation-api'
import { ConfirmDialog, SettingCard, SettingEmpty, SettingRow, SettingSection, StatusBadge } from './settings-primitives'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/api'
import type { Channel, ChannelInput, Rule, WeChatBinding } from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/models'

const key = ['notifications']
const platforms = [
  { value: 'telegram', label: 'Telegram' }, { value: 'discord', label: 'Discord' }, { value: 'slack', label: 'Slack' },
  { value: 'dingtalk', label: '钉钉' }, { value: 'feishu', label: '飞书' }, { value: 'lark', label: 'Lark' },
  { value: 'wechat', label: '微信' }, { value: 'wecom', label: '企业微信' }, { value: 'qq', label: 'QQ' },
]
const appPlatforms = ['discord', 'slack', 'dingtalk', 'feishu', 'lark', 'wecom']
const recipientFields: Record<string, { label: string; example: string }> = {
  telegram: { label: 'Telegram 用户 ID 或会话 ID', example: '123456789\n-100123456789' },
  discord: { label: 'Discord 用户 ID', example: '123456789012345678' },
  slack: { label: 'Slack 工作区成员 ID', example: 'U012ABCDEF' },
  dingtalk: { label: '钉钉企业成员 UserID', example: 'user123' },
  feishu: { label: '飞书成员 ID', example: 'ou_xxxxx\nuser_id:employee123' },
  lark: { label: 'Lark 成员 ID', example: 'ou_xxxxx\nuser_id:employee123' },
  wecom: { label: '企业微信成员 UserID', example: 'zhangsan' },
  qq: { label: 'QQ 私聊 OpenID、群 OpenID 或子频道 ID', example: 'c2c:xxxxx\ngroup:xxxxx\nchannel:12345' },
}
const statusLabels: Record<string, { label: string; tone: 'active' | 'muted' | 'warning' | 'error' }> = {
  accepted: { label: '已接收', tone: 'active' },
  unknown: { label: '结果未知', tone: 'warning' },
  pending: { label: '待发送', tone: 'muted' },
  sending: { label: '发送中', tone: 'muted' },
  failed: { label: '失败', tone: 'error' },
  cancelled: { label: '已取消', tone: 'muted' },
}
const blankChannel: ChannelInput = {
  id: '', name: '', platform: 'telegram', target: '', mode: '', targets: [], recipients: [], address: '', signing: '', context: '', media_app_id: '', media_secret: '', enabled: true, verified: false, revision: 0,
}
const blankRule: Rule = {
  id: '', name: '', mode: 'instant', categories: [], channels: [], body: false, image: false,
  enabled: true, revision: 0, next_at: 0, timezone: defaultZone, send_at: '20:00', window_start: '00:00', window_end: '20:00',
}

export function NotificationSettings() {
  const client = useQueryClient()
  const channels = useQuery({ queryKey: [...key, 'channels'], queryFn: () => api.ListChannels(), retry: false })
  const rules = useQuery({ queryKey: [...key, 'rules'], queryFn: () => api.ListRules(), retry: false })
  const navigation = useQuery({ queryKey: navigationQueryKey, queryFn: () => navigationAPI.list() })
  const [channel, setChannel] = useState<ChannelInput>()
  const [recipientText, setRecipientText] = useState('')
  const [rule, setRule] = useState<Rule>()
  const preview = useQuery({
    queryKey: [...key, 'rule-preview', rule?.timezone, rule?.send_at, rule?.window_start, rule?.window_end],
    queryFn: () => api.PreviewRule(rule!),
    enabled: rule?.mode === 'daily',
    retry: false,
  })
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
  const [confirmation, setConfirmation] = useState<{ title: string; description: string; label: string; destructive: boolean; run: () => Promise<unknown> }>()
  const deliveries = useQuery({ queryKey: [...key, 'deliveries'], queryFn: () => api.ListDeliveries(), enabled: showHistory, refetchInterval: showHistory ? 5000 : false })
  const mutation = useMutation({
    mutationFn: (work: () => Promise<unknown>) => work(),
    onSettled: () => client.invalidateQueries({ queryKey: key, predicate: query => query.queryKey[1] !== 'wechat' }),
  })
  const channelOptions = (channels.data ?? []).filter(value => value.enabled && value.verified).map(value => ({ label: value.name, value: value.id }))
  const categoryOptions = (navigation.data?.categories ?? []).map(value => ({ label: value.name, value: value.id }))

  function askDelete(name: string, run: () => Promise<unknown>) {
    mutation.reset()
    setConfirmation({ title: `删除“${name}”？`, description: '未开始的投递将取消，已发送的消息保留。', label: '删除', destructive: true, run })
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

  const setChannelField = <K extends keyof ChannelInput,>(field: K, value: ChannelInput[K]) =>
    setChannel(previous => previous && { ...previous, [field]: value })
  const setRuleField = <K extends keyof Rule,>(field: K, value: Rule[K]) =>
    setRule(previous => previous && { ...previous, [field]: value })

  function editChannel(value: Channel) {
    const targets = value.recipients?.map(recipient => recipient.target) ?? (value.target ? [value.target] : [])
    mutation.reset()
    setChannel({ ...value, targets, address: '', signing: '', context: '', media_app_id: '', media_secret: '' })
    setRecipientText(targets.join('\n'))
  }

  function closeChannelDialog() {
    if (wechat?.id && bindingStatus !== 'connected') void api.CancelWeChatBinding(wechat.id)
    setWeChat(undefined)
    setPin('')
    setVerifiedPin('')
    setChannel(undefined)
    mutation.reset()
  }

  return <div className="space-y-6">
    <SettingSection
      title="通知渠道"
      actions={
        <Button size="sm" variant="outline" disabled={busy} onClick={() => {
          mutation.reset()
          setRecipientText('')
          setChannel(blankChannel)
        }}>
          <AnimatedGlyph meaning="plus" />添加渠道
        </Button>
      }
    >
      <SettingCard>
        {(channels.data ?? []).length === 0 ? (
          <SettingEmpty icon={<AnimatedGlyph meaning="send" />} title="还没有配置外部通知渠道" />
        ) : (channels.data ?? []).map(value => (
          <SettingRow key={value.id}>
            <div className="min-w-0 flex-1 space-y-0.5">
              <div className="flex flex-wrap items-center gap-2">
                <span className="truncate font-medium">{value.name}</span>
                <StatusBadge tone={value.verified ? (value.enabled ? 'active' : 'muted') : 'warning'}>
                  {value.verified ? (value.enabled ? '已启用' : '已停用') : '待测试'}
                </StatusBadge>
              </div>
              <p className="text-xs text-muted-foreground">
                {platforms.find(item => item.value === value.platform)?.label ?? value.platform}
              </p>
            </div>
            <div className="flex shrink-0 flex-wrap items-center gap-1">
              <Button size="sm" variant="ghost" disabled={busy} onClick={() => editChannel(value)}>编辑</Button>
              <Button size="sm" variant="ghost" disabled={busy} onClick={() => mutation.mutate(() => api.TestChannel(value.id))}>
                {(value.recipients?.length ?? 0) > 1 ? '测试全部收件目标' : '发送测试'}
              </Button>
              <Button size="sm" variant="ghost" disabled={busy} onClick={() => askDelete(value.name, () => api.DeleteChannel(value.id, value.revision))}>删除</Button>
            </div>
          </SettingRow>
        ))}
      </SettingCard>
    </SettingSection>

    <SettingSection
      title="文章推送规则"
      actions={
        <Button size="sm" variant="outline" disabled={!channelOptions.length || busy} onClick={() => { mutation.reset(); setRule(blankRule) }}>
          <AnimatedGlyph meaning="plus" />添加规则
        </Button>
      }
    >
      <SettingCard>
        {(rules.data ?? []).length === 0 ? (
          <SettingEmpty icon={<AnimatedGlyph meaning="bell" />} title={channelOptions.length ? '还没有文章推送规则' : '先添加并测试通知渠道，再创建推送规则'} />
        ) : (rules.data ?? []).map(value => (
          <SettingRow key={value.id}>
            <div className="min-w-0 flex-1 space-y-0.5">
              <div className="flex flex-wrap items-center gap-2">
                <span className="truncate font-medium">{value.name}</span>
                <StatusBadge tone={value.enabled ? 'active' : 'muted'}>
                  {value.enabled ? (value.mode === 'daily' ? '每日汇总' : '逐条推送') : '已暂停'}
                </StatusBadge>
              </div>
              {value.enabled && value.mode === 'daily' && value.next_at > 0 && (
                <p className="text-xs text-muted-foreground tabular-nums">下次发送 {time(value.next_at)}</p>
              )}
            </div>
            <div className="flex shrink-0 items-center gap-1">
              <Button size="sm" variant="ghost" disabled={busy} onClick={() => { mutation.reset(); setRule(value) }}>编辑</Button>
              <Button size="sm" variant="ghost" disabled={busy} onClick={() => askDelete(value.name, () => api.DeleteRule(value.id, value.revision))}>删除</Button>
            </div>
          </SettingRow>
        ))}
      </SettingCard>
    </SettingSection>

    <SettingSection
      title="投递历史"
      actions={
        <Button size="sm" variant="outline" aria-expanded={showHistory} onClick={() => setShowHistory(value => !value)}>
          <AnimatedGlyph meaning="history" />投递历史
        </Button>
      }
    >
      {showHistory && (
        <SettingCard>
          {deliveries.isError && <div className="p-3.5"><FieldError>暂时无法读取投递历史</FieldError></div>}
          {deliveries.data?.length === 0 && <SettingEmpty icon={<AnimatedGlyph meaning="history" />} title="还没有投递记录" />}
          {(deliveries.data ?? []).map(value => {
            const badge = statusLabels[value.state] ?? { label: value.state, tone: 'muted' as const }
            return (
              <SettingRow key={value.id}>
                <div className="min-w-0 flex-1 space-y-0.5">
                  <p className="truncate font-medium">
                    {channels.data?.find(item => item.id === value.channel_id)?.name ?? '通知渠道'}
                    {value.recipient_target && ` · ${value.recipient_target}`}
                  </p>
                  <p className="text-xs text-muted-foreground tabular-nums">{time(value.created_at)}</p>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <StatusBadge tone={badge.tone}>{badge.label}</StatusBadge>
                  {['failed', 'unknown'].includes(value.state) && (
                    <Button size="sm" variant="ghost" onClick={() => {
                      mutation.reset()
                      setConfirmation({
                        title: '重新投递？',
                        label: '重新投递',
                        destructive: false,
                        run: () => api.RetryDelivery(value.id),
                        description: value.state === 'unknown' ? '平台可能已接收原消息，重发可能产生重复。' : '使用原接收渠道重新投递。',
                      })
                    }}>重发</Button>
                  )}
                </div>
              </SettingRow>
            )
          })}
        </SettingCard>
      )}
    </SettingSection>

    {mutation.error && !confirmation && !channel && !rule && <FieldError>{mutation.error.message}</FieldError>}
    {(channels.isError || rules.isError) && <FieldError>暂时无法读取通知配置</FieldError>}

    <Dialog open={Boolean(channel)} onOpenChange={open => { if (!open && !busy) closeChannelDialog() }}>
      {channel && (
        <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{channel.id ? '编辑渠道' : '添加渠道'}</DialogTitle>
          </DialogHeader>
          <FieldGroup>
            <TextField id="channel-name" label="渠道名称" value={channel.name} onChange={value => setChannelField('name', value)} />
            <Selection id="channel-platform" label="平台" options={platforms} value={channel.platform} disabled={Boolean(channel.id)} onChange={value => {
              setChannel({ ...blankChannel, platform: value, name: channel.name })
              setRecipientText('')
            }} />
            {appPlatforms.includes(channel.platform) && (
              <Selection
                id="channel-mode"
                label="接入方式"
                options={[{ value: 'webhook', label: '群或频道 Webhook' }, { value: 'app', label: '应用机器人私发' }]}
                value={appMode ? 'app' : 'webhook'}
                disabled={Boolean(channel.id)}
                onChange={value => {
                  setChannel({ ...blankChannel, platform: channel.platform, name: channel.name, mode: value === 'app' ? 'app' : '' })
                  setRecipientText('')
                }}
              />
            )}
            {!bindingMode && (
              <fieldset className="min-w-0 space-y-2.5 rounded-lg border border-border p-3">
                <legend className="px-1 text-xs font-medium text-muted-foreground">收件白名单</legend>
                {webhookMode ? (
                  <p className="text-[13px] text-muted-foreground">已绑定的整个群或频道</p>
                ) : channel.platform === 'wechat' ? (
                  <Selection
                    id="wechat-recipients"
                    label="已绑定的微信私聊"
                    options={wechatOptions}
                    value={channel.targets ?? []}
                    onChange={value => setChannelField('targets', toggle(channel.targets, value))}
                  />
                ) : (
                  <Field>
                    <FieldLabel htmlFor="channel-recipients">{recipientFields[channel.platform]?.label}</FieldLabel>
                    <Textarea
                      id="channel-recipients"
                      value={recipientText}
                      onChange={event => setRecipientText(event.target.value)}
                      placeholder={recipientFields[channel.platform]?.example}
                      rows={3}
                    />
                  </Field>
                )}
                {!webhookMode && savedRecipients.length > 0 && (
                  <div className="divide-y divide-border rounded-md border border-border">
                    {savedRecipients.map(recipient => (
                      <div className="flex items-center justify-between gap-2 px-2.5 py-1.5 text-xs" key={recipient.id}>
                        <span className="min-w-0 break-all">{recipientLabel(recipient.target)}</span>
                        <div className="flex shrink-0 items-center gap-1.5">
                          <StatusBadge tone={recipient.verified ? 'active' : 'warning'}>{recipient.verified ? '已测试' : '待测试'}</StatusBadge>
                          <Button variant="ghost" size="xs" disabled={busy}
                            onClick={() => mutation.mutate(() => api.TestRecipient(channel.id, recipient.id))}>发送测试</Button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </fieldset>
            )}
            {channel.platform !== 'wechat' && (
              <TextField
                id="channel-secret"
                label={webhookMode ? 'Webhook URL' : channel.platform === 'telegram' || channel.platform === 'discord' ? 'Bot Token'
                  : channel.platform === 'slack' ? 'Slack Bot Token' : channel.platform === 'wecom' ? 'CorpID' : channel.platform === 'dingtalk' ? 'AppKey' : 'AppID'}
                value={channel.address}
                type="password"
                onChange={value => setChannelField('address', value)}
              />
            )}
            {(['qq', 'dingtalk', 'feishu', 'lark', 'wecom'].includes(channel.platform) && (channel.platform !== 'wecom' || appMode)) && (
              <TextField
                id="channel-signing"
                label={webhookMode ? '加签密钥（可选）' : channel.platform === 'wecom' ? '应用 Secret' : 'AppSecret'}
                type="password"
                value={channel.signing}
                onChange={value => setChannelField('signing', value)}
              />
            )}
            {appMode && ['dingtalk', 'wecom'].includes(channel.platform) && (
              <TextField
                id="channel-context"
                label={channel.platform === 'wecom' ? '应用 AgentID' : 'RobotCode（可选）'}
                value={channel.context}
                onChange={value => setChannelField('context', value)}
              />
            )}
            {webhookMode && ['feishu', 'lark'].includes(channel.platform) && (
              <>
                <TextField id="channel-media-app" label="图片上传 AppID（可选）" value={channel.media_app_id} onChange={value => setChannelField('media_app_id', value)} />
                <TextField id="channel-media-secret" label="图片上传 AppSecret" type="password" value={channel.media_secret} onChange={value => setChannelField('media_secret', value)} />
              </>
            )}
            {bindingMode && (
              <div className="flex flex-col items-center gap-3 rounded-lg border border-border bg-muted/25 p-4 text-center">
                {wechat?.image && !['connected', 'expired', 'verify_code_blocked', 'binded_redirect'].includes(bindingStatus ?? '') && (
                  <img src={wechat.image} width={192} height={192} alt="微信绑定二维码" className="rounded-lg border border-border bg-white p-2" />
                )}
                {bindingStatus === 'wait' && <p className="text-[13px] text-muted-foreground">用微信扫描二维码</p>}
                {bindingStatus === 'scaned' && <StatusBadge tone="active">请在微信确认绑定</StatusBadge>}
                {bindingStatus === 'await_message' && <p className="text-[13px] text-muted-foreground">绑定后，在微信私聊里给机器人发一条消息</p>}
                {bindingStatus === 'connected' && <p role="status" className="text-[13px] font-medium">微信已绑定，可以发送测试</p>}
                {['expired', 'verify_code_blocked', 'binded_redirect'].includes(bindingStatus ?? '') && <FieldError>绑定未完成，请重新扫码</FieldError>}
                {bindingStatus?.includes('need_verifycode') && (
                  <div className="flex w-full items-end gap-2 text-left">
                    <TextField id="wechat-pin" label="微信验证码" value={pin} onChange={setPin} />
                    <Button size="sm" onClick={() => setVerifiedPin(pin)}>确认验证码</Button>
                  </div>
                )}
                {wechatPoll.isError && <FieldError>无法完成微信绑定，请重试</FieldError>}
              </div>
            )}
            <FlagField id="channel-enable" label="启用渠道" value={channel.enabled} onChange={value => setChannelField('enabled', value)} />
            {mutation.error && <FieldError>{mutation.error.message}</FieldError>}
            <DialogFooter>
              <Button variant="outline" size="sm" disabled={busy} onClick={closeChannelDialog}>
                {bindingStatus === 'connected' ? '完成' : '取消'}
              </Button>
              {bindingMode ? (
                <Button size="sm" disabled={busy || !channel.name.trim() || bindingStatus === 'connected'} onClick={() => mutation.mutate(async () => {
                  setPin('')
                  setVerifiedPin('')
                  setWeChat(await api.StartWeChatBinding())
                })}>
                  {wechat ? '重新生成二维码' : '生成微信二维码'}
                </Button>
              ) : (
                <Button size="sm" disabled={busy} onClick={() => mutation.mutate(async () => {
                  const targets = webhookMode ? [] : channel.platform === 'wechat' ? channel.targets : recipientText.split(/[\s,，]+/u).filter(Boolean)
                  await api.SaveChannel({ ...channel, targets })
                  setChannel(undefined)
                })}>
                  保存渠道
                </Button>
              )}
            </DialogFooter>
          </FieldGroup>
        </DialogContent>
      )}
    </Dialog>

    <Dialog open={Boolean(rule)} onOpenChange={open => { if (!open && !busy) { setRule(undefined); mutation.reset() } }}>
      {rule && (
        <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{rule.id ? '编辑推送规则' : '添加推送规则'}</DialogTitle>
          </DialogHeader>
          <FieldGroup>
            <TextField id="notification-rule-name" label="规则名称" value={rule.name} onChange={value => setRuleField('name', value)} />
            <Selection id="notification-rule-channels" label="接收渠道" options={channelOptions} value={rule.channels ?? []} onChange={value => setRuleField('channels', toggle(rule.channels, value))} />
            <Selection id="notification-rule-categories" label="分类" options={categoryOptions} value={rule.categories ?? []} placeholder="全部分类" onChange={value => setRuleField('categories', toggle(rule.categories, value))} />
            <Selection id="notification-rule-mode" label="推送方式" options={[{ value: 'instant', label: '逐条推送' }, { value: 'daily', label: '每日汇总' }]} value={rule.mode} onChange={value => setRuleField('mode', value)} />
            {rule.mode === 'daily' && (
              <>
                <Selection id="notification-rule-timezone" label="时区" value={rule.timezone} options={zones} onChange={value => setRuleField('timezone', value)} />
                <div className="grid grid-cols-3 gap-2">
                  <TextField id="notification-rule-start" label="收录开始" type="time" value={rule.window_start} onChange={value => setRuleField('window_start', value)} />
                  <TextField id="notification-rule-end" label="收录结束" type="time" value={rule.window_end} onChange={value => setRuleField('window_end', value)} />
                  <TextField id="notification-rule-at" label="发送时间" type="time" value={rule.send_at} onChange={value => setRuleField('send_at', value)} />
                </div>
                {preview.data && (
                  <p className="text-xs text-muted-foreground tabular-nums">
                    下次发送 {time(preview.data.next_at)} · 收录 {time(preview.data.start)} 至 {time(preview.data.end)}
                  </p>
                )}
              </>
            )}
            <FlagField id="notification-rule-body" label="附带正文" value={rule.body} onChange={value => setRuleField('body', value)} />
            <FlagField id="notification-rule-image" label="附带首图" value={rule.image} onChange={value => setRuleField('image', value)} />
            <FlagField id="notification-rule-enabled" label="启用规则" value={rule.enabled} onChange={value => setRuleField('enabled', value)} />
            {mutation.error && <FieldError>{mutation.error.message}</FieldError>}
            <DialogFooter>
              <Button variant="outline" size="sm" disabled={busy} onClick={() => { setRule(undefined); mutation.reset() }}>取消</Button>
              <Button size="sm" disabled={busy} onClick={() => mutation.mutate(async () => { await api.SaveRule(rule); setRule(undefined) })}>保存规则</Button>
            </DialogFooter>
          </FieldGroup>
        </DialogContent>
      )}
    </Dialog>

    <ConfirmDialog
      open={Boolean(confirmation)}
      title={confirmation?.title ?? ''}
      description={confirmation?.description}
      confirmLabel={confirmation?.label ?? '确认'}
      destructive={confirmation?.destructive ?? true}
      busy={busy}
      error={confirmation && mutation.error ? mutation.error.message : undefined}
      onOpenChange={open => { if (!open) { setConfirmation(undefined); mutation.reset() } }}
      onConfirm={() => {
        if (!confirmation) return
        mutation.mutate(async () => {
          await confirmation.run()
          setConfirmation(undefined)
        })
      }}
    />
  </div>
}
