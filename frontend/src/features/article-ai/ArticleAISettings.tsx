import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
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
  RateLimitError: '模型服务已限流，请稍后重试。',
  Timeout: '模型服务响应超时。',
  TooManyCategories: '自动分类目前最多支持 128 个分类。',
  ValidationError: '请检查预算、评分阈值和提示词长度。',
}

const numericSettings = [
  { key: 'requestsPerHour', label: '每小时请求上限', min: 1, max: 1000, step: 1 },
  { key: 'tokensPerHour', label: '每小时估算输入 token 上限', min: 1000, max: 2000000, step: 1 },
  { key: 'minimumImportance', label: '精选最低重要性', min: 0, max: 1, step: 0.01 },
  { key: 'minimumRelevance', label: '精选最低相关性', min: 0, max: 1, step: 0.01 },
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
  const [showSamples, setShowSamples] = useState(false)
  const [result, setResult] = useState<Result>()
  const [error, setError] = useState('')
  const active = useRef<ReturnType<typeof Evaluate> | null>(null)
  const samples = useQuery({
    queryKey: ['article-ai', 'samples'],
    queryFn: ({ signal }) => libraryAPI.list({ scope: { kind: 'all' }, sort: 'published_desc', limit: 20 }, signal),
    enabled: showSamples,
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
  if (!value) return settings.isError
    ? <Button variant="outline" onClick={() => void settings.refetch()}>重新读取文章 AI 设置</Button>
    : <p role="status">正在读取文章 AI 设置…</p>
  const busy = save.isPending || testing
  const enabled = task === 'classification' ? settings.data?.classification : settings.data?.featured
  return <section aria-label="文章 AI" className="flex flex-col gap-4">
    <h3>文章 AI</h3>
    <form onSubmit={event => { event.preventDefault(); if (!busy && draft) save.mutate(draft) }}>
      <FieldGroup>
        <Field orientation="horizontal">
          <FieldLabel htmlFor="automatic-classification">自动分类</FieldLabel>
          <Switch id="automatic-classification" disabled={busy} checked={value.classification} onCheckedChange={classification => setDraft({ ...value, classification })} />
        </Field>
        <Field orientation="horizontal">
          <FieldLabel htmlFor="automatic-featured">精选评分</FieldLabel>
          <Switch id="automatic-featured" disabled={busy} checked={value.featured} onCheckedChange={featured => setDraft({ ...value, featured })} />
        </Field>
        <Field>
          <FieldLabel htmlFor="classification-prompt">分类提示词</FieldLabel>
          <Textarea id="classification-prompt" disabled={busy} value={value.prompt} maxLength={4096} onChange={event => setDraft({ ...value, prompt: event.target.value })} />
        </Field>
        <p className="text-muted-foreground">启用后会向所选服务发送近期文章的标题、摘要和部分正文。分类还会发送分类名称、描述及提示词。</p>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {numericSettings.map(({ key, label, min, max, step }) => <Field key={key}>
            <FieldLabel htmlFor={`article-ai-${key}`}>{label}</FieldLabel>
            <Input id={`article-ai-${key}`} type="number" required min={min} max={max} step={step}
              disabled={busy} value={Number.isFinite(value[key]) ? value[key] : ''}
              onChange={event => setDraft({ ...value, [key]: event.target.valueAsNumber })} />
          </Field>)}
        </div>
        <div><Button type="submit" size="sm" disabled={busy || !draft}>保存文章 AI 设置</Button></div>
        {save.isError && <p role="alert">{message(save.error)}</p>}
      </FieldGroup>
    </form>
    {!showSamples ? <div><Button size="sm" variant="outline" onClick={() => setShowSamples(true)}>试跑一篇文章</Button></div> : <FieldGroup>
      <Field>
        <FieldLabel htmlFor="article-ai-sample">近期样本</FieldLabel>
        <Select value={sample} onValueChange={value => { setSample(value); setResult(undefined); setError('') }} disabled={busy || !samples.data?.items.length}>
          <SelectTrigger id="article-ai-sample" className="w-full"><SelectValue placeholder="选择文章" /></SelectTrigger>
          <SelectContent position="popper"><SelectGroup>
            {samples.data?.items.map(article => <SelectItem key={article.id} value={article.id}>{article.title || '无标题'}</SelectItem>)}
          </SelectGroup></SelectContent>
        </Select>
      </Field>
      {samples.isError && <Button variant="ghost" onClick={() => void samples.refetch()}>重新读取样本</Button>}
      <Field>
        <FieldLabel htmlFor="article-ai-task">评估用途</FieldLabel>
        <Select value={task} onValueChange={value => { setTask(value); setResult(undefined); setError('') }} disabled={busy}>
          <SelectTrigger id="article-ai-task"><SelectValue /></SelectTrigger>
          <SelectContent position="popper"><SelectGroup>
            <SelectItem value="classification">分类</SelectItem>
            <SelectItem value="featured">精选</SelectItem>
          </SelectGroup></SelectContent>
        </Select>
      </Field>
      <p className="text-muted-foreground">试跑使用已保存设置，发送同样的文章资料，不修改分类或精选。</p>
      <div className="flex flex-wrap gap-2">
        <Button size="sm" variant="outline" disabled={busy || !sample || !!draft} onClick={() => void evaluate(true)}>发送试跑</Button>
        <Button size="sm" disabled={busy || !sample || !enabled || !!draft} onClick={() => void evaluate(false)}>评估并应用</Button>
        {testing && <Button size="sm" variant="ghost" onClick={cancel}>取消评估</Button>}
      </div>
      {error && <p role="alert">{error}</p>}
      {result && <div role="status">
        {result.cached ? <p>当前文章已评估。</p> : task === 'featured'
          ? <p>重要性 {result.scores.importance.toFixed(2)} · 相关性 {result.scores.relevance.toFixed(2)} · {result.scores.reason}</p>
          : result.matches?.length ? <ul>{result.matches.map(item => <li key={item.id}>{item.name}（{Math.round(item.confidence * 100)}%） · {item.reason}</li>)}</ul> : <p>无匹配分类。</p>}
      </div>}
    </FieldGroup>}
  </section>
}
