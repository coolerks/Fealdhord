import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useArticleAISettings } from '@/features/article-ai/settings-query'
import { Generate, Get } from '../../../bindings/github.com/coolerks/Fealdhord/internal/articlesummary/service'

const summaryErrors: Record<string, string> = {
  ConfigurationError: '请在「服务与模型」中配置摘要模型。',
  BodyUnavailable: '暂时无法取得完整正文。',
  ContextWindowExceeded: '正文超出当前摘要模型的处理范围。',
  InvalidModelOutput: '摘要结果不完整或重复正文，请重试。',
  Conflict: '正文或摘要设置已更新，请重试。',
  Timeout: '生成摘要超时，请重试。若持续超时，可在「服务与模型」检查当前摘要模型。',
  ProviderConnectionError: '无法连接摘要模型服务。请检查网络，或在「服务与模型」测试连接。',
  RateLimitError: '摘要模型暂时限流，请稍后重试。',
  StorageError: '无法保存或读取摘要，请重试。',
}

export function useArticleSummary(articleId: string | undefined, mode: 'feed' | 'reader', version: string | undefined) {
  const client = useQueryClient()
  const articleAI = useArticleAISettings(Boolean(articleId)).data
  const settings = articleAI?.summary
  const targetLang = articleAI?.translationTargetLang || 'zh-CN'
  const source = useMemo(() => articleId && version ? { articleId, mode, version } : undefined, [articleId, mode, version])
  const sourceKey = JSON.stringify([source ?? null, targetLang])
  const queryKey = useMemo(
    () => ['reader', 'article-summary', sourceKey, settings?.hanLimit, settings?.englishLimit],
    [sourceKey, settings?.hanLimit, settings?.englishLimit],
  )
  const query = useQuery({
    queryKey,
    queryFn: ({ signal }) => {
      const request = Get(source!)
      const cancel = () => { void request.cancel() }
      if (signal.aborted) cancel()
      signal.addEventListener('abort', cancel, { once: true })
      return Promise.resolve(request).finally(() => signal.removeEventListener('abort', cancel))
    },
    enabled: Boolean(source && settings),
    retry: false,
  })
  const attempted = useRef('')
  const active = useRef<{ key: string; request: ReturnType<typeof Generate> } | null>(null)
  const [activity, setActivity] = useState<{ key: string; pending: boolean; error?: string }>()

  const cancel = useCallback(() => {
    const previous = active.current
    if (!previous) return
    active.current = null
    void previous.request.cancel()
    setActivity(undefined)
  }, [])

  useEffect(() => () => {
    if (active.current?.key === sourceKey) {
      void active.current.request.cancel()
      active.current = null
    }
  }, [sourceKey])

  const generate = useCallback(async (automatic = false) => {
    if (!source || active.current) return
    const request = Generate({ source, automatic })
    const operation = { key: sourceKey, request }
    active.current = operation
    setActivity({ key: sourceKey, pending: true })
    try {
      const result = await request
      await client.cancelQueries({ queryKey, exact: true })
      if (active.current === operation) {
        client.setQueryData(queryKey, result)
        setActivity(undefined)
      }
    } catch (error) {
      if (active.current === operation) {
        const code = error instanceof Error ? error.message : String(error)
        setActivity(automatic ? undefined : { key: sourceKey, pending: false, error: code })
      }
    } finally {
      if (active.current === operation) active.current = null
    }
  }, [client, queryKey, source, sourceKey])

  useEffect(() => {
    if (!source || !settings?.onOpen || !query.data || attempted.current === sourceKey) return
    let current = true
    queueMicrotask(() => {
      if (!current) return
      attempted.current = sourceKey
      if (query.data.status === 'none') void generate(true)
    })
    return () => { current = false }
  }, [source, sourceKey, settings?.onOpen, query.data, generate])

  const current = activity?.key === sourceKey ? activity : undefined
  return {
    result: query.data,
    pending: Boolean(current?.pending),
    error: current?.error ? (summaryErrors[current.error] ?? '生成摘要失败，请重试。') : '',
    needsModel: current?.error === 'ConfigurationError',
    generate,
    cancel,
  }
}
