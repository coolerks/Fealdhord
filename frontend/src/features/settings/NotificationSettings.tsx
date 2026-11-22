import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Textarea } from '@/components/ui/textarea'
import { defaultZone, FlagField, Selection, TextField, time, toggle, zones } from '@/features/notifications/notification-fields'
import { navigationAPI, navigationQueryKey } from '@/lib/navigation-api'
import { ConfirmDialog, SettingCard, SettingEmpty, SettingRow, SettingSection, StatusBadge } from './settings-primitives'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/api'
import type { Channel, ChannelInput, PairingStatus, Rule, WeChatBinding } from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/models'

const key = ['notifications']
const pairingInstructions: Record<string, string> = {
  telegram: '在 Telegram 里给机器人发送「配对」。要绑定群聊，请在机器人能接收消息的群里发送。',
  qq: '私聊发送「配对」，或在普通 QQ 群、QQ 频道的文字子频道中 @机器人并发送「配对」。',
  discord: '在 Discord 里私信机器人「配对」。',
  slack: '在 Slack 应用中启用 Socket Mode 并订阅 message.im，再私聊机器人发送「配对」。',
  dingtalk: '启用机器人的 Stream 模式。在钉钉里私聊机器人发送「配对」。',
  feishu: '为应用启用长连接并订阅接收消息事件，然后私聊机器人发送「配对」。',
  lark: '为应用启用长连接并订阅接收消息事件，然后私聊机器人发送「配对」。',
  wechat: '在「微信 ClawBot」私聊里发送「配对」。',
}
const terminalWeChatStatuses = ['connected', 'expired', 'verify_code_blocked', 'binded_redirect']
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
  const [channelStep, setChannelStep] = useState(1)
  const [pairing, setPairing] = useState<PairingStatus>()
  const [slackAppToken, setSlackAppToken] = useState('')
  const pairedTargets = recipientText.split(/[\s,，]+/u).filter(Boolean)
  const pairingID = pairing?.id
  const pairingPoll = useQuery({
    queryKey: [...key, 'pairing', pairingID],
    queryFn: () => api.RecipientPairingStatus(pairingID!),
    enabled: Boolean(pairingID) && channelStep === 2,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    refetchInterval: query => query.state.status === 'error' ||
      ['failed', 'expired'].includes(query.state.data?.state ?? '') ? false : 750,
  })
  useEffect(() => {
    if (!pairingID) return
    return () => { void api.CancelRecipientPairing(pairingID) }
  }, [pairingID])
  const [rule, setRule] = useState<Rule>()
  const preview = useQuery({
    queryKey: [...key, 'rule-preview', rule?.timezone, rule?.send_at, rule?.window_start, rule?.window_end],
    queryFn: () => api.PreviewRule(rule!),
    enabled: rule?.mode === 'daily',
    retry: false,
  })
  const [wechat, setWeChat] = useState<WeChatBinding>()
  const [pin, setPin] = useState('')
  const [verifiedPin, setVerifiedPin] = useState<{ value: string; attempt: number }>()
  const wechatPoll = useQuery({
    queryKey: [...key, 'wechat', wechat?.id, verifiedPin?.attempt, channelStep],
    queryFn: async ({ signal }) => {
      const request = api.PollWeChatBinding(wechat!.id, verifiedPin?.value ?? '', channelStep === 2)
      const cancel = () => { void request.cancel() }
      if (signal.aborted) cancel()
      else signal.addEventListener('abort', cancel, { once: true })
      try { return await request } finally { signal.removeEventListener('abort', cancel) }
    },
    enabled: Boolean(wechat?.id) && channel?.platform === 'wechat' && channelStep < 3,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    refetchInterval: query => query.state.status === 'error' || terminalWeChatStatuses.includes(query.state.data?.status ?? '') ||
      query.state.data?.status.includes('need_verifycode') ||
      (channelStep === 1 && query.state.data?.status === 'await_message') ? false : 1200,
  })
  const wechatID = wechat?.id
  useEffect(() => {
    if (!wechatID) return
    return () => { void api.CancelWeChatBinding(wechatID) }
  }, [wechatID])

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
  const currentWeChat = wechatPoll.data ?? wechat
  const bindingStatus = currentWeChat?.status
  const wechatMode = channel?.platform === 'wechat'
  const bindingMode = wechatMode && !channel.id
  const pairStatus = pairingPoll.data ?? pairing
  const appMode = channel?.mode === 'app'
  const webhookMode = Boolean(channel && appPlatforms.includes(channel.platform) && !appMode)
  const savedRecipients = channels.data?.find(value => value.id === channel?.id)?.recipients ?? []
  const wechatOptions = channel?.platform === 'wechat' ? [
    ...(channel.target ? [{ value: channel.target, label: '当前绑定私聊' }] : []),
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
    setChannelStep(3)
  }

  function closeChannelDialog() {
    void client.cancelQueries({ queryKey: [...key, 'wechat'] })
    setPairing(undefined)
    setSlackAppToken('')
    setWeChat(undefined)
    setPin('')
    setVerifiedPin(undefined)
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
          setChannelStep(1)
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
          {!webhookMode && (
            <ol aria-label="设置进度" className="grid grid-cols-3 gap-2 text-xs text-muted-foreground">
              {['填写凭据', '配对收件目标', '确认配置'].map((label, index) => (
                <li key={label} aria-current={channelStep === index + 1 ? 'step' : undefined}
                  className={channelStep === index + 1 ? 'font-semibold text-foreground' : undefined}>
                  {index + 1}. {label}
                </li>
              ))}
            </ol>
          )}
          <FieldGroup>
            {channelStep !== 2 && <>
              <TextField id="channel-name" label="渠道名称" value={channel.name} disabled={busy} onChange={value => setChannelField('name', value)} />
              <Selection id="channel-platform" label="平台" options={platforms} value={channel.platform} disabled={Boolean(channel.id || wechat) || busy || channelStep === 3} onChange={value => {
                setChannel({ ...blankChannel, platform: value, name: channel.name })
                setSlackAppToken('')
                setRecipientText('')
              }} />
              {appPlatforms.includes(channel.platform) && (
                <Selection
                  id="channel-mode"
                  label="接入方式"
                  options={[{ value: 'webhook', label: '群或频道 Webhook' }, { value: 'app', label: '应用机器人私发' }]}
                  value={appMode ? 'app' : 'webhook'}
                  disabled={Boolean(channel.id) || channelStep === 3}
                  onChange={value => {
                    setChannel({ ...blankChannel, platform: channel.platform, name: channel.name, mode: value === 'app' ? 'app' : '' })
                    setRecipientText('')
                  }}
                />
              )}
              {channel.platform === 'qq' && (
                <Selection
                  id="qq-environment"
                  label="QQ 接入环境"
                  options={[{ value: 'production', label: '正式环境' }, { value: 'sandbox', label: '沙箱（未上线测试）' }]}
                  value={channel.mode === 'sandbox' ? 'sandbox' : 'production'}
                  disabled={busy || Boolean(channel.id) || channelStep === 3}
                  onChange={value => {
                    setChannelField('mode', value === 'sandbox' ? value : '')
                    setRecipientText('')
                  }}
                />
              )}
              {channel.id && !wechatMode && (
                <p className="text-xs text-muted-foreground">凭据留空时沿用已保存的值。</p>
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
              {channelStep === 1 && appMode && channel.platform === 'slack' && (
                <TextField id="slack-pairing-app-token" label="配对用 App-Level Token（xapp-）" type="password"
                  value={slackAppToken} onChange={setSlackAppToken} />
              )}
            </>}
            {channelStep === 3 && (
              <fieldset className="min-w-0 space-y-2.5 rounded-lg border border-border p-3">
                <legend className="px-1 text-xs font-medium text-muted-foreground">收件白名单</legend>
                {webhookMode ? (
                  <p className="text-[13px] text-muted-foreground">已绑定的整个群或频道</p>
                ) : bindingMode && !channel.targets?.length ? (
                  <p className="text-[13px] text-muted-foreground">尚未配对。可保存后再配对。</p>
                ) : wechatMode ? (
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
            {wechatMode && channelStep < 3 && (
              <div className="flex flex-col items-center gap-3 rounded-lg border border-border bg-muted/25 p-4 text-center">
                {wechat?.image && !wechatPoll.isError && !terminalWeChatStatuses.includes(bindingStatus ?? '') && (
                  <img src={wechat.image} width={192} height={192} alt="微信绑定二维码" className="rounded-lg border border-border bg-white p-2" />
                )}
                {!wechatPoll.isError && bindingStatus === 'wait' && <p className="text-[13px] text-muted-foreground">用微信扫描二维码</p>}
                {!wechatPoll.isError && bindingStatus === 'scaned' && <StatusBadge tone="active">请在微信确认绑定</StatusBadge>}
                {!wechatPoll.isError && bindingStatus === 'await_message' && <p className="text-[13px] text-muted-foreground">{channelStep === 1 ? '微信扫码已确认' : pairingInstructions.wechat}</p>}
                {bindingStatus === 'connected' && <p role="status" className="text-[13px] font-medium">已收到配对消息</p>}
                {['expired', 'verify_code_blocked'].includes(bindingStatus ?? '') && <FieldError>绑定已失效，请重新生成二维码</FieldError>}
                {bindingStatus === 'binded_redirect' && <FieldError>微信报告此机器人已绑定，没有发放新的凭据</FieldError>}
                {!wechatPoll.isError && bindingStatus?.includes('need_verifycode') && (
                  <div className="flex w-full items-end gap-2 text-left">
                    <TextField id="wechat-pin" label="微信验证码" value={pin} onChange={setPin} />
                    <Button size="sm" disabled={!/^\d{1,12}$/.test(pin.trim())} onClick={() => {
                      setVerifiedPin(previous => ({ value: pin.trim(), attempt: (previous?.attempt ?? 0) + 1 }))
                    }}>确认验证码</Button>
                  </div>
                )}
                {(channelStep === 1 && !channel.id && !['await_message', 'connected'].includes(bindingStatus ?? '')) && (
                  <Button variant="outline" size="sm" disabled={busy} onClick={() => mutation.mutate(async () => {
                    if (wechatID) {
                      await client.cancelQueries({ queryKey: [...key, 'wechat', wechatID] })
                      await api.CancelWeChatBinding(wechatID)
                    }
                    setPin('')
                    setVerifiedPin(undefined)
                    setWeChat(await api.StartWeChatBinding())
                  })}>{wechat ? '重新生成二维码' : '生成微信二维码'}</Button>
                )}
                {channelStep === 2 && channel.id && (!wechat || wechatPoll.isError || terminalWeChatStatuses.includes(bindingStatus ?? '') && bindingStatus !== 'connected') && (
                  <Button variant="outline" size="sm" disabled={busy} onClick={() => mutation.mutate(async () => {
                    setWeChat(await api.ResumeWeChatBinding(channel))
                  })}>{wechat ? '重新配对' : '开始配对'}</Button>
                )}
                {wechatPoll.isError && <FieldError>{wechatPoll.error.message}</FieldError>}
              </div>
            )}
            {channelStep === 2 && !wechatMode && (
              <div className="space-y-3 rounded-lg border border-border p-3">
                {channel.platform === 'wecom' ? (
                  <p className="text-sm text-muted-foreground">企业微信自建应用需要配置消息回调。可跳过配对，直接填写成员 UserID。</p>
                ) : (
                  <>
                    <p className="text-sm text-muted-foreground">{pairingInstructions[channel.platform]}</p>
                    {pairStatus?.state === 'connecting' && <p role="status" className="text-sm">正在连接机器人…</p>}
                    {channel.platform === 'qq' && pairStatus?.bot_name && (
                      <p className="text-xs text-muted-foreground">QQ 机器人 · {pairStatus.bot_name}</p>
                    )}
                    {pairStatus?.state === 'listening' && (
                      <p role="status" className="text-sm">
                        {pairStatus.targets?.length ? '已收到配对消息，请勾选收件目标' :
                          channel.platform === 'qq' ? (pairStatus.received_message
                            ? '已收到 QQ 消息。请单独发送「配对」。'
                            : 'QQ 网关已连接，尚未收到聊天消息') : '正在等待配对消息'}
                      </p>
                    )}
                    {channel.platform === 'qq' && pairStatus?.state === 'listening' && !pairStatus.received_message && !pairStatus.targets?.length && (
                      <p className="text-xs text-muted-foreground">
                        {channel.mode === 'sandbox'
                          ? '请从 QQ 开放平台配置的沙箱账号和测试会话发送。'
                          : '如果手机提示「未连接灵魂」，请点手机上的「查看指引」，核对当前聊天是否对应上方机器人。'}
                      </p>
                    )}
                    {pairStatus?.state === 'expired' && (
                      <FieldError>{channel.platform === 'qq' && !pairStatus.targets?.length ? (pairStatus.received_message
                        ? '已收到 QQ 消息，但未识别到「配对」。本次配对已结束。'
                        : 'QQ 网关未送达聊天消息，本次配对已结束。') : '配对已超时'}</FieldError>
                    )}
                    {pairStatus?.state === 'failed' && <FieldError>{pairStatus.detail}</FieldError>}
                    {pairingPoll.isError && <FieldError>暂时无法读取配对状态</FieldError>}
                    {(!pairStatus || ['failed', 'expired'].includes(pairStatus.state) || pairingPoll.isError) && (
                      <Button variant="outline" size="sm" disabled={busy} onClick={() => mutation.mutate(async () => {
                        if (pairingID) await api.CancelRecipientPairing(pairingID)
                        setPairing(await api.StartRecipientPairing(channel, slackAppToken))
                      })}>{pairStatus ? '重新配对' : '开始配对'}</Button>
                    )}
                    {(pairStatus?.targets ?? []).map(target => (
                      <label key={target} className="flex items-start gap-2 break-all text-sm">
                        <Checkbox checked={pairedTargets.includes(target)} onCheckedChange={checked => {
                          setRecipientText((checked ? [...pairedTargets, target] : pairedTargets.filter(value => value !== target)).join('\n'))
                        }} />
                        <span>{target}</span>
                      </label>
                    ))}
                  </>
                )}
              </div>
            )}
            {channelStep === 3 && <FlagField id="channel-enable" label="启用渠道" value={channel.enabled} onChange={value => setChannelField('enabled', value)} />}
            {mutation.error && <FieldError>{mutation.error.message}</FieldError>}
            <DialogFooter>
              <Button variant="outline" size="sm" disabled={busy} onClick={closeChannelDialog}>取消</Button>
              {channelStep > 1 && (
                <Button variant="outline" size="sm" disabled={busy} onClick={() => {
                  mutation.reset()
                  setPairing(undefined)
                  setChannelStep(channelStep === 3 && !webhookMode ? 2 : 1)
                }}>上一步</Button>
              )}
              {channelStep < 3 ? (
                <Button size="sm" disabled={busy || (channelStep === 1 && bindingMode && !['await_message', 'connected'].includes(bindingStatus ?? ''))}
                  onClick={() => {
                    mutation.reset()
                    if (channelStep === 1) {
                      if (currentWeChat?.target) setWeChat(currentWeChat)
                      setChannelStep(webhookMode ? 3 : 2)
                      return
                    }
                    if (currentWeChat?.target) {
                      const target = currentWeChat.target
                      setChannel({ ...channel, target, targets: bindingStatus === 'connected' ? [...new Set([...(channel.targets ?? []), target])] : channel.targets })
                      setWeChat(currentWeChat)
                    }
                    setPairing(undefined)
                    setSlackAppToken('')
                    setChannelStep(3)
                  }}>
                  {channelStep === 1 ? '下一步' : (bindingStatus === 'connected' || (pairStatus?.targets ?? []).some(target => pairedTargets.includes(target))) ? '下一步' : '跳过配对'}
                </Button>
              ) : (
                <Button size="sm" disabled={busy} onClick={() => mutation.mutate(async () => {
                  const targets = webhookMode ? [] : wechatMode ? channel.targets : pairedTargets
                  const config = { ...channel, targets }
                  if (wechatID && (bindingMode || bindingStatus === 'connected')) await api.CompleteWeChatBinding(wechatID, config)
                  else await api.SaveChannel(config)
                  closeChannelDialog()
                })}>保存渠道</Button>
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
