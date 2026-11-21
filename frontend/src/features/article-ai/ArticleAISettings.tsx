import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { SettingCard, SettingEmpty, SettingRow, SettingSection, StatusBadge } from '@/features/settings/settings-primitives'
import { libraryAPI } from '@/lib/library-api'
import { invalidateRSS } from '@/lib/refresh'
import { Evaluate, Save } from '../../../bindings/github.com/coolerks/Fealdhord/internal/articleai/api'
import type { Result, Settings } from '../../../bindings/github.com/coolerks/Fealdhord/internal/articleai/models'
import { articleAISettingsKey, useArticleAISettings } from './settings-query'

const messages: Record<string, string> = {
  Busy: '文章 AI 正在运行，请稍后重试。',
  BudgetExceeded: '已达到一小时的请求预算。',
  ConfigurationError: '请先在用途分配中选择启用的模型。',
  Conflict: '配置或文章已变化，请重新载入后再试。',
  Disabled: '请先启用并保存对应功能。',
  InvalidModelOutput: '模型返回的内容格式无效。',
  NoCategories: '请先创建至少一个分类。',
  NotFound: '文章已删除或所属订阅已归档。',
  RateLimitError: '模型服务已限流，请稍后再试。',
  Timeout: '模型服务响应超时。',
  TooManyCategories: '自动分类目前最多支持 128 个分类。',
  ValidationError: '请检查预算、评分阈值和提示词长度。',
}

const budgetSettings = [
  { key: 'requestsPerHour', label: '每小时请求上限', min: 1, max: 1000, step: 1 },
  { key: 'tokensPerHour', label: '每小时估算输入 token 上限', min: 1000, max: 2000000, step: 1 },
] as const

const thresholdSettings = [
  { key: 'minimumImportance', label: '精选最低重要性' },
  { key: 'minimumRelevance', label: '精选最低相关性' },
] as const

function message(error: unknown) {
  const raw = error instanceof Error ? error.message : ''
  return Object.entries(messages).find(([code]) => raw.includes(code))?.[1] ?? '文章 AI 请求失败，请检查设置后重试。'
}

export function ArticleAISettings() {
  const cache = useQueryClient()
  const settings = useArticleAISettings()
  const [draft, setDraft] = useState<Settings>()
  const [sample, setSample] = useState('')
  const [task, setTask] = useState('classification')
  const [testing, setTesting] = useState(false)
  const [sampleOpen, setSampleOpen] = useState(false)
  const [result, setResult] = useState<Result>()
  const [error, setError] = useState('')
  const active = useRef<ReturnType<typeof Evaluate> | null>(null)
  const samples = useQuery({
    queryKey: ['article-ai', 'samples'],
    queryFn: ({ signal }) => libraryAPI.list({ scope: { kind: 'all' }, sort: 'published_desc', limit: 20 }, signal),
    enabled: sampleOpen,
    retry: false,
  })
  const save = useMutation({
    mutationFn: (value: Settings) => Save(value),
    onSuccess: value => {
      cache.setQueryData(articleAISettingsKey, value)
      setDraft(undefined)
      return invalidateRSS(cache)
    },
  })

  useEffect(() => () => {
    const request = active.current
    active.current = null
    if (request) void request.cancel()
  }, [])

  function cancel() {
    const request = active.current
    active.current = null
    if (request) void request.cancel()
    setTesting(false)
  }

  async function evaluate(preview: boolean) {
    if (active.current || !sample) return
    const request = Evaluate({ articleId: sample, task, preview })
    active.current = request
    setTesting(true)
    setResult(undefined)
    setError('')
    try {
      const response = await request
      if (active.current === request) setResult(response)
    } catch (cause) {
      if (active.current === request) setError(message(cause))
    } finally {
      if (active.current === request) {
        active.current = null
        setTesting(false)
      }
    }
  }

  const value = draft ?? settings.data
  if (!value) {
    return settings.isError ? (
      <SettingCard divided={false}>
        <SettingEmpty title="无法读取文章 AI 设置" action={<Button size="sm" variant="outline" onClick={() => void settings.refetch()}>重新读取文章 AI 设置</Button>} />
      </SettingCard>
    ) : <p role="status" className="text-[13px] text-muted-foreground">正在读取文章 AI 设置…</p>
  }

  const busy = save.isPending || testing
  const enabled = task === 'classification' ? settings.data?.classification : settings.data?.featured
  const showRules = value.classification || value.featured || Boolean(draft)

  return <div className="space-y-6">
    <form className="space-y-6" onSubmit={event => { event.preventDefault(); if (!busy && draft) save.mutate(draft) }}>
      <SettingSection
        title="自动评估"
        actions={draft && (
          <>
            <Button type="button" size="sm" variant="ghost" disabled={busy} onClick={() => setDraft(undefined)}>放弃更改</Button>
            <Button type="submit" size="sm" disabled={busy}>保存文章 AI 设置</Button>
          </>
        )}
      >
        <SettingCard>
          <SettingRow>
            <FieldLabel htmlFor="automatic-classification" className="font-normal">自动分类</FieldLabel>
            <Switch id="automatic-classification" disabled={busy} checked={value.classification}
              onCheckedChange={classification => setDraft({ ...value, classification })} />
          </SettingRow>
          <SettingRow>
            <FieldLabel htmlFor="automatic-featured" className="font-normal">精选评分</FieldLabel>
            <Switch id="automatic-featured" disabled={busy} checked={value.featured}
              onCheckedChange={featured => setDraft({ ...value, featured })} />
          </SettingRow>
        </SettingCard>
      </SettingSection>

      {showRules && (
        <SettingSection title="规则与预算">
          <SettingCard>
            {value.classification && (
              <div className="p-3.5">
                <Field>
                  <FieldLabel htmlFor="classification-prompt">分类提示词</FieldLabel>
                  <Textarea
                    id="classification-prompt"
                    rows={3}
                    disabled={busy}
                    value={value.prompt}
                    maxLength={4096}
                    onChange={event => setDraft({ ...value, prompt: event.target.value })}
                  />
                </Field>
              </div>
            )}
            {value.featured && thresholdSettings.map(({ key, label }) => {
              const current = Number.isFinite(value[key]) ? value[key] : 0
              return (
                <div key={key} className="space-y-2.5 px-3.5 py-2.5">
                  <div className="flex items-center justify-between gap-2">
                    <FieldLabel id={`article-ai-${key}`} className="font-normal">{label}</FieldLabel>
                    <span className="range-value text-xs">{current.toFixed(2)}</span>
                  </div>
                  <Slider
                    min={0}
                    max={1}
                    step={0.01}
                    disabled={busy}
                    value={[current]}
                    thumbProps={{ 'aria-labelledby': `article-ai-${key}`, 'aria-valuetext': current.toFixed(2) }}
                    onValueChange={([next]) => {
                      if (Number.isFinite(next)) setDraft({ ...value, [key]: Math.round(next * 100) / 100 })
                    }}
                  />
                </div>
              )
            })}
            <div className="grid grid-cols-1 gap-3 p-3.5 sm:grid-cols-2">
              {budgetSettings.map(({ key, label, min, max, step }) => (
                <Field key={key}>
                  <FieldLabel htmlFor={`article-ai-${key}`}>{label}</FieldLabel>
                  <Input
                    id={`article-ai-${key}`}
                    type="number"
                    required
                    min={min}
                    max={max}
                    step={step}
                    disabled={busy}
                    value={Number.isFinite(value[key]) ? value[key] : ''}
                    onChange={event => setDraft({ ...value, [key]: event.target.valueAsNumber })}
                  />
                </Field>
              ))}
            </div>
            <SettingRow className="bg-muted/25">
              <p className="min-w-0 flex-1 text-xs text-muted-foreground">
                启用后会向已分配模型发送近期文章标题、摘要和部分正文。
              </p>
              <Button type="submit" size="sm" disabled={busy || !draft}>保存文章 AI 设置</Button>
            </SettingRow>
          </SettingCard>
        </SettingSection>
      )}
      {save.isError && <FieldError>{message(save.error)}</FieldError>}
    </form>

    <SettingSection title="效果试跑">
      <SettingCard>
        <SettingRow>
          <span className="text-[13px]">选取近期文章测试当前分类或精选规则</span>
          <Button size="sm" variant="outline" onClick={() => setSampleOpen(true)}>
            <AnimatedGlyph meaning="flask" />试跑一篇文章
          </Button>
        </SettingRow>
      </SettingCard>
    </SettingSection>

    <Dialog open={sampleOpen} onOpenChange={open => { if (!open) { cancel(); setSampleOpen(false) } }}>
      {sampleOpen && (
        <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>试跑文章 AI</DialogTitle>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="article-ai-sample">近期样本</FieldLabel>
              <SearchableSelect
                id="article-ai-sample"
                label="近期样本"
                className="w-full"
                placeholder="选择文章"
                value={sample}
                onValueChange={next => { setSample(next); setResult(undefined); setError('') }}
                disabled={busy || !samples.data?.items.length}
                options={(samples.data?.items ?? []).map(article => ({ value: article.id, label: article.title || '无标题' }))}
              />
            </Field>
            {samples.isError && <Button size="sm" variant="ghost" onClick={() => void samples.refetch()}>重新读取样本</Button>}
            <Field>
              <FieldLabel htmlFor="article-ai-task">评估用途</FieldLabel>
              <SearchableSelect
                id="article-ai-task"
                label="评估用途"
                className="w-full"
                value={task}
                disabled={busy}
                onValueChange={next => { setTask(next); setResult(undefined); setError('') }}
                options={[{ value: 'classification', label: '分类' }, { value: 'featured', label: '精选' }]}
              />
            </Field>
            {draft && <p className="text-xs text-muted-foreground">当前有未保存修改，请先保存后再试跑。</p>}
            {error && <FieldError>{error}</FieldError>}
            {result && (
              <div role="status" className="space-y-2 rounded-lg border border-border bg-muted/35 p-3 text-[13px]">
                {result.cached ? (
                  <p>当前文章已评估。</p>
                ) : task === 'featured' ? (
                  <>
                    <div className="flex flex-wrap gap-2">
                      <StatusBadge tone="active">重要性 {result.scores.importance.toFixed(2)}</StatusBadge>
                      <StatusBadge tone="active">相关性 {result.scores.relevance.toFixed(2)}</StatusBadge>
                    </div>
                    {result.scores.reason && <p className="text-xs text-muted-foreground">{result.scores.reason}</p>}
                  </>
                ) : result.matches?.length ? (
                  <ul className="space-y-1.5">
                    {result.matches.map(item => (
                      <li key={item.id} className="flex flex-wrap items-baseline gap-2">
                        <StatusBadge tone="active">{item.name} · {Math.round(item.confidence * 100)}%</StatusBadge>
                        {item.reason && <span className="text-xs text-muted-foreground">{item.reason}</span>}
                      </li>
                    ))}
                  </ul>
                ) : <p className="text-muted-foreground">无匹配分类。</p>}
              </div>
            )}
            <DialogFooter>
              {testing && <Button type="button" size="sm" variant="ghost" onClick={cancel}>取消评估</Button>}
              <Button type="button" size="sm" variant="outline" disabled={busy || !sample || !!draft} onClick={() => void evaluate(true)}>发送试跑</Button>
              <Button type="button" size="sm" disabled={busy || !sample || !enabled || !!draft} onClick={() => void evaluate(false)}>评估并应用</Button>
            </DialogFooter>
          </FieldGroup>
        </DialogContent>
      )}
    </Dialog>
  </div>
}
