import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { SettingRow, StatusBadge } from '@/features/settings/settings-primitives'
import { native } from '@/lib/native'
import {
  translationAPI,
  translationErrorMessage,
  type ProviderConfig,
  type SaveProviderInput,
  type TestResult,
} from '@/lib/translation-api'

interface ProviderGuide {
  title: string
  docsURL: string
  steps: string
  appIDLabel?: string
  appIDPlaceholder?: string
  secretLabel: string
  secretPlaceholder?: string
  regionLabel?: string
  regionPlaceholder?: string
  regionHint?: string
  endpointHint?: string
  editions?: { value: string; label: string }[]
  editionLabel?: string
}

const providerGuides: Record<string, ProviderGuide> = {
  google_byok: {
    title: 'Google Cloud Translation（v2 Basic）',
    docsURL: 'https://cloud.google.com/translate/docs/setup',
    steps: '先打开 [Google Cloud 控制台](https://console.cloud.google.com/apis/library/translate.googleapis.com) 并登录账号，选择或新建一个项目，点击**启用 Cloud Translation API**（需绑定结算账号，每月自带 50 万字符免费额度）；再到左侧 **API 和服务 → 凭据** 点击**创建凭据 → API 密钥**，复制生成的 **API Key** 粘贴到下方即可。',
    secretLabel: 'API Key',
    secretPlaceholder: 'AIzaSy...',
    endpointHint: '可选：留空使用官方 v2 地址；国内若配置了反向代理可填入完整接口 URL。',
  },
  microsoft_azure: {
    title: 'Microsoft Azure AI Translator',
    docsURL: 'https://learn.microsoft.com/azure/ai-services/translator/create-translator-resource',
    steps: '打开 [Azure 门户](https://portal.azure.com/#create/Microsoft.CognitiveServicesTextTranslation) 创建 **Translator** 资源，定价层选择 **F0（免费，每月 200 万字符）** 或 **S1（按量付费）**；创建完成后进入资源的 **密钥和终结点** 页面，复制 **密钥 1** 填入下方，若资源不是“全球（Global）”而是指定区域（如 `eastasia`）或 Azure AI 多服务资源，还需把页面上的 **位置/区域** 填入区域框。',
    secretLabel: '订阅密钥（Subscription Key）',
    secretPlaceholder: '32 位密钥',
    regionLabel: '资源区域（Region）',
    regionPlaceholder: '例如 eastasia（全球单服务可留空）',
    regionHint: '区域 Translator 资源与 Azure AI 多服务资源必填；全球单服务资源可留空。',
    endpointHint: '可选：留空使用官方 Translator v3.0 地址。',
    editionLabel: '订阅定价层',
    editions: [
      { value: 'f0', label: 'F0 免费层（按 30,000 字/分钟限流防 429）' },
      { value: 's1', label: 'S1+ 付费层（高并发）' },
    ],
  },
  alibaba: {
    title: '阿里云机器翻译（通用版）',
    docsURL: 'https://help.aliyun.com/zh/machine-translation/developer-reference/api-alimt-2018-10-12-getbatchtranslate',
    steps: '打开 [阿里云机器翻译控制台](https://translate.console.aliyun.com/) 开通 **机器翻译通用版**（新用户每月通常有 100 万字符免费额度）；再前往 [RAM 访问控制](https://ram.console.aliyun.com/users) 创建一个子用户，授予 **AliyunMTFullAccess** 权限并创建 **AccessKey**，将生成的 **AccessKey ID** 与 **AccessKey Secret** 分别填入下方。',
    appIDLabel: 'AccessKey ID',
    appIDPlaceholder: 'LTAI5t...',
    secretLabel: 'AccessKey Secret',
    secretPlaceholder: 'AccessKey Secret',
    endpointHint: '可选：留空默认使用华东 1（杭州）通用接入点。',
  },
  caiyun: {
    title: '彩云小译',
    docsURL: 'https://docs.caiyunapp.com/lingocloud-api/',
    steps: '打开 [彩云科技开放平台](https://platform.caiyunapp.com/) 登录并完成实名认证，进入 **应用管理** 新建一个翻译应用，在应用详情页找到 **令牌（Token）** 并复制到下方即可。注意：彩云小译支持**中↔英/日/韩**与**德/法/西/俄→中**，暂不支持非中文语种之间直译。',
    secretLabel: 'API Token',
    secretPlaceholder: '彩云开放平台 Token',
  },
  baidu: {
    title: '百度翻译开放平台',
    docsURL: 'https://fanyi-api.baidu.com/doc/21',
    steps: '打开 [百度翻译开放平台](https://fanyi-api.baidu.com/manage/developer) 登录并开通 **通用文本翻译 API**，在 **开发者信息** 页面复制 **APP ID** 与 **密钥** 填入下方；完成个人实名认证可从标准版（1 QPS）升级到高级版（10 QPS），升级后请在下方同步切换**账户版本**以提升速度。',
    appIDLabel: 'APP ID',
    appIDPlaceholder: '例如 20240101000000000',
    secretLabel: '密钥（Secret Key）',
    secretPlaceholder: '开发者密钥',
    editionLabel: '账户版本（决定并发速率）',
    editions: [
      { value: 'standard', label: '标准版（1 QPS）' },
      { value: 'senior', label: '高级版（10 QPS）' },
      { value: 'premium', label: '企业尊享版（100 QPS）' },
    ],
  },
  youdao: {
    title: '有道智云',
    docsURL: 'https://ai.youdao.com/DOCSIRMA/html/trans/api/plwbfy/index.html',
    steps: '打开 [有道智云控制台](https://ai.youdao.com/console/) 登录后进入 **自然语言翻译服务** 创建实例并勾选 **文本翻译**；再到 **应用总览** 创建应用并绑定刚建的文本翻译实例，复制应用详情中的 **应用 ID（AppKey）** 和 **应用密钥（AppSecret）** 填入下方。若需开启网页 HTML 模式，需在控制台额外开通并绑定 **网页翻译** 服务。',
    appIDLabel: '应用 ID（AppKey）',
    appIDPlaceholder: '应用 ID',
    secretLabel: '应用密钥（AppSecret）',
    secretPlaceholder: '应用密钥',
    editionLabel: '接口模式',
    editions: [
      { value: 'text', label: '批量文本翻译（默认，仅需开通文本翻译）' },
      { value: 'html', label: '网页 HTML 翻译（需额外开通网页翻译服务）' },
    ],
  },
}

function renderHighlightedSteps(text: string) {
  return text.split(/(\*\*[^*]+\*\*|`[^`]+`|\[[^\]]+\]\(https:\/\/[^)]+\))/gu).map((part, index) => {
    if (part.startsWith('**') && part.endsWith('**')) {
      return <strong key={index} className="font-semibold text-blue-600 dark:text-blue-400">{part.slice(2, -2)}</strong>
    }
    if (part.startsWith('`') && part.endsWith('`')) {
      return <code key={index} className="rounded bg-muted px-1 py-0.5 font-mono text-xs text-foreground">{part.slice(1, -1)}</code>
    }
    const link = /^\[([^\]]+)\]\((https:\/\/[^)]+)\)$/u.exec(part)
    if (link) {
      const [, label, url] = link
      return (
        <a
          key={index}
          href={url}
          className="font-medium text-foreground underline decoration-muted-foreground/70 underline-offset-4 transition-colors hover:text-blue-600 hover:decoration-blue-600 dark:hover:text-blue-400 dark:hover:decoration-blue-400"
          onClick={event => {
            event.preventDefault()
            void native.openURL(url)
          }}
        >
          {label}
        </a>
      )
    }
    return part
  })
}

function defaultEdition(id: string) {
  if (id === 'microsoft_azure') return 'f0'
  if (id === 'baidu') return 'standard'
  if (id === 'youdao') return 'text'
  return ''
}

export function TranslationProviderRow({ engineId, disabled }: { engineId: string; disabled?: boolean }) {
  const cache = useQueryClient()
  const isFree = engineId === 'microsoft_edge' || engineId === 'google_free'
  const guide = providerGuides[engineId]
  const [open, setOpen] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<TestResult | null>(null)

  const configQuery = useQuery({
    queryKey: ['translation-provider', engineId],
    queryFn: () => translationAPI.getProviderConfig(engineId),
    enabled: Boolean(guide),
    retry: false,
  })

  useEffect(() => {
    setTestResult(null)
  }, [engineId])

  async function runQuickTest() {
    setTesting(true)
    setTestResult(null)
    try {
      const res = await translationAPI.testProvider(engineId, null)
      setTestResult(res)
    } catch (error) {
      setTestResult({ ok: false, latencyMs: 0, errorCode: 'ProviderHTTPError', errorDetail: translationErrorMessage(error) })
    } finally {
      setTesting(false)
    }
  }

  if (isFree) {
    return (
      <div className="space-y-2 px-3.5 py-3">
        <div className="flex items-center justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span className="text-[13px]">免费接口</span>
              <StatusBadge tone="warning">非官方</StatusBadge>
            </div>
            <p className="text-xs text-muted-foreground">无需配置密钥，可能受网络、代理或频率限制。</p>
          </div>
          <Button type="button" size="sm" variant="outline" disabled={disabled || testing} onClick={() => void runQuickTest()}>
            {testing ? '测试中…' : '测试连接'}
          </Button>
        </div>
        {testResult && <ProviderTestBanner result={testResult} />}
      </div>
    )
  }

  if (!guide) return null

  const cfg = configQuery.data
  const configured = Boolean(cfg?.hasSecret && (!guide.appIDLabel || cfg.appId))

  return (
    <SettingRow>
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-[13px]">接口凭据</span>
          <StatusBadge tone={configured ? 'active' : 'muted'}>
            {configured ? `已配置${cfg?.secretMasked ? ` · ${cfg.secretMasked}` : ''}` : '未配置'}
          </StatusBadge>
        </div>
        {testResult && <div className="mt-1.5"><ProviderTestBanner result={testResult} /></div>}
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <Button type="button" size="sm" variant="ghost" disabled={disabled || testing || !configured} onClick={() => void runQuickTest()}>
          {testing ? '测试中…' : '测试连接'}
        </Button>
        <Button type="button" size="sm" variant="outline" disabled={disabled} onClick={() => setOpen(true)}>
          {configured ? '修改凭据' : '配置凭据'}
        </Button>
      </div>
      {open && (
        <ProviderConfigDialog
          engineId={engineId}
          guide={guide}
          initial={cfg}
          open={open}
          onOpenChange={setOpen}
          onSaved={saved => {
            cache.setQueryData(['translation-provider', engineId], saved)
            setTestResult(null)
          }}
        />
      )}
    </SettingRow>
  )
}

function ProviderTestBanner({ result }: { result: TestResult }) {
  if (result.ok) {
    return (
      <p role="status" className="text-xs text-emerald-600 dark:text-emerald-400">
        连接成功（{result.latencyMs} ms）：{result.sampleOut}
      </p>
    )
  }
  const msg = result.errorCode ? translationErrorMessage(new Error(result.errorCode)) : '连接测试失败。'
  return <FieldError>{msg}</FieldError>
}

function ProviderConfigDialog({
  engineId,
  guide,
  initial,
  open,
  onOpenChange,
  onSaved,
}: {
  engineId: string
  guide: ProviderGuide
  initial?: ProviderConfig
  open: boolean
  onOpenChange(open: boolean): void
  onSaved(saved: ProviderConfig): void
}) {
  const [appId, setAppId] = useState(initial?.appId ?? '')
  const [secret, setSecret] = useState('')
  const [clearSecret, setClearSecret] = useState(false)
  const [region, setRegion] = useState(initial?.region ?? '')
  const [endpoint, setEndpoint] = useState(initial?.endpoint ?? '')
  const [edition, setEdition] = useState(initial?.edition || defaultEdition(engineId))
  const [testResult, setTestResult] = useState<TestResult | null>(null)
  const [testing, setTesting] = useState(false)

  useEffect(() => {
    if (!open) return
    setAppId(initial?.appId ?? '')
    setSecret('')
    setClearSecret(false)
    setRegion(initial?.region ?? '')
    setEndpoint(initial?.endpoint ?? '')
    setEdition(initial?.edition || defaultEdition(engineId))
    setTestResult(null)
  }, [open, initial, engineId])

  const draftInput: SaveProviderInput = {
    id: engineId,
    appId: appId.trim(),
    secret: secret.trim(),
    clearSecret,
    region: region.trim(),
    endpoint: endpoint.trim(),
    edition,
  }

  const save = useMutation({
    mutationFn: () => translationAPI.saveProviderConfig(draftInput),
    onSuccess: saved => {
      onSaved(saved)
      onOpenChange(false)
    },
  })

  async function handleTest() {
    setTesting(true)
    setTestResult(null)
    try {
      const res = await translationAPI.testProvider(engineId, draftInput)
      setTestResult(res)
    } catch (error) {
      setTestResult({ ok: false, latencyMs: 0, errorCode: 'ProviderHTTPError', errorDetail: translationErrorMessage(error) })
    } finally {
      setTesting(false)
    }
  }

  const busy = save.isPending || testing

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <div className="flex items-center justify-between gap-2 pr-6">
            <DialogTitle>配置 {guide.title}</DialogTitle>
            <div className="flex items-center gap-1">
              <Dialog>
                <DialogTrigger asChild>
                  <Button type="button" variant="ghost" size="xs" className="text-muted-foreground">操作步骤</Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>{guide.title} 获取与配置步骤</DialogTitle>
                    <DialogDescription className="leading-relaxed">
                      {renderHighlightedSteps(guide.steps)}
                    </DialogDescription>
                  </DialogHeader>
                </DialogContent>
              </Dialog>
              <Button asChild variant="ghost" size="xs" className="text-muted-foreground">
                <a
                  href={guide.docsURL}
                  onClick={event => {
                    event.preventDefault()
                    void native.openURL(guide.docsURL)
                  }}
                >
                  查看官方文档
                </a>
              </Button>
            </div>
          </div>
        </DialogHeader>

        <div className="rounded-lg border border-border/70 bg-muted/30 px-3 py-2.5 text-xs leading-relaxed text-muted-foreground">
          {renderHighlightedSteps(guide.steps)}
        </div>

        <FieldGroup>
          {guide.appIDLabel && (
            <Field>
              <FieldLabel htmlFor="tp-appid">{guide.appIDLabel}</FieldLabel>
              <Input
                id="tp-appid"
                disabled={busy}
                value={appId}
                placeholder={guide.appIDPlaceholder}
                onChange={event => setAppId(event.target.value)}
              />
            </Field>
          )}

          <Field>
            <div className="flex items-center justify-between gap-2">
              <FieldLabel htmlFor="tp-secret">{guide.secretLabel}</FieldLabel>
              {initial?.hasSecret && !clearSecret && (
                <Button
                  type="button"
                  size="xs"
                  variant="ghost"
                  className="text-muted-foreground"
                  disabled={busy}
                  onClick={() => { setClearSecret(true); setSecret('') }}
                >
                  清除已保存密钥
                </Button>
              )}
            </div>
            <Input
              id="tp-secret"
              type="password"
              autoComplete="off"
              disabled={busy}
              value={secret}
              placeholder={
                initial?.hasSecret && !clearSecret
                  ? `已保存（${initial.secretMasked || '****'}），留空保持不变`
                  : guide.secretPlaceholder
              }
              onChange={event => {
                setSecret(event.target.value)
                if (event.target.value) setClearSecret(false)
              }}
            />
          </Field>

          {guide.regionLabel && (
            <Field>
              <FieldLabel htmlFor="tp-region">{guide.regionLabel}</FieldLabel>
              <Input
                id="tp-region"
                disabled={busy}
                value={region}
                placeholder={guide.regionPlaceholder}
                onChange={event => setRegion(event.target.value)}
              />
              {guide.regionHint && <p className="text-xs text-muted-foreground">{guide.regionHint}</p>}
            </Field>
          )}

          {guide.editions && guide.editionLabel && (
            <Field>
              <FieldLabel htmlFor="tp-edition">{guide.editionLabel}</FieldLabel>
              <SearchableSelect
                id="tp-edition"
                label={guide.editionLabel}
                className="w-full"
                disabled={busy}
                value={edition}
                options={guide.editions}
                onValueChange={setEdition}
              />
            </Field>
          )}

          {guide.endpointHint && (
            <Field>
              <FieldLabel htmlFor="tp-endpoint">自定义接口地址（可选）</FieldLabel>
              <Input
                id="tp-endpoint"
                type="url"
                disabled={busy}
                value={endpoint}
                placeholder="留空使用官方默认地址"
                onChange={event => setEndpoint(event.target.value)}
              />
              <p className="text-xs text-muted-foreground">{guide.endpointHint} 仅支持 HTTPS。更换域名或端口时请重新输入密钥。</p>
            </Field>
          )}

          {testResult && <ProviderTestBanner result={testResult} />}
          {save.isError && <FieldError>{translationErrorMessage(save.error)}</FieldError>}

          <DialogFooter>
            <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void handleTest()}>
              {testing ? '测试中…' : '测试连接'}
            </Button>
            <Button type="button" size="sm" disabled={busy} onClick={() => save.mutate()}>
              {save.isPending ? '保存中…' : '保存凭据'}
            </Button>
          </DialogFooter>
        </FieldGroup>
      </DialogContent>
    </Dialog>
  )
}
