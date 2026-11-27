import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Events } from '@wailsio/runtime'
import { useArticleAISettings } from '@/features/article-ai/settings-query'
import { translationAPI, translationErrorCode, translationErrorMessage, translationProgressEvent } from '@/lib/translation-api'
import type { TranslationDisplayMode, TranslationRecord } from '@/lib/translation-api'

function normalizeDisplayMode(value?: string): TranslationDisplayMode {
  return value === 'translated' || value === 'original' ? value : 'bilingual'
}

function latestTranslation(current: TranslationRecord | undefined, next: TranslationRecord) {
  return current && (current.updatedAt > next.updatedAt ||
    (current.updatedAt === next.updatedAt && current.status !== 'translating' && next.status === 'translating'))
    ? current : next
}

export function useArticleTranslation({
  articleID,
  articleRevision,
  sourceMode,
  sourceVersion,
  readStartBlock,
  hasContent,
  extracting,
  autoReady,
  defaultDisplay,
  feedAutoTranslateMode,
}: {
  articleID?: string
  articleRevision?: number
  sourceMode: 'feed' | 'reader'
  sourceVersion?: string
  readStartBlock: () => string | undefined
  hasContent: boolean
  extracting: boolean
  autoReady: boolean
  defaultDisplay?: string
  feedAutoTranslateMode?: string
}) {
  const client = useQueryClient()
  const aiSettings = useArticleAISettings(Boolean(articleID))
  const defaultTargetLang = aiSettings.data?.translationTargetLang || 'zh-CN'
  const translateOnOpen = aiSettings.data?.translateOnOpen ?? false
  const usingAIModel = (aiSettings.data?.translationEngine || 'ai') === 'ai'

  const [overrideLang, setOverrideLang] = useState<string>()
  const [displayOverride, setDisplayMode] = useState<TranslationDisplayMode>()
  const [requests, setRequests] = useState<Record<string, { running: boolean; errorCode: string }>>({})
  const running = useRef(new Set<string>())
  const autoTriggered = useRef(new Set<string>())

  const displayMode = displayOverride ?? normalizeDisplayMode(defaultDisplay)
  const targetLang = overrideLang || defaultTargetLang
  const queryKey = useMemo(
    () => ['reader', 'translation', articleID, articleRevision ?? 0, targetLang, sourceMode, sourceVersion ?? ''] as const,
    [articleID, articleRevision, targetLang, sourceMode, sourceVersion],
  )
  const requestKey = JSON.stringify(queryKey)

  const query = useQuery({
    queryKey,
    queryFn: async () => {
      const next = await translationAPI.get(articleID!, targetLang, sourceMode)
      if (sourceVersion && next.sourceVersion !== sourceVersion) return null
      return latestTranslation(client.getQueryData<TranslationRecord | null>(queryKey) ?? undefined, next)
    },
    enabled: Boolean(articleID) && hasContent && !extracting,
    staleTime: 60000,
    retry: false,
  })

  const record = query.data
  useEffect(() => {
    if (record === null) void client.invalidateQueries({ queryKey: ['reader', 'content', articleID] })
  }, [articleID, client, record])

  // 订阅后端按批次推送的翻译进度事件，实时渲染已翻译段落。
  useEffect(() => {
    if (!articleID || typeof Events?.On !== 'function') return
    let disposed = false
    let unsubscribe: (() => void) | undefined
    try {
      unsubscribe = Events.On(translationProgressEvent, (event: { data?: { record?: TranslationRecord } | [{ record?: TranslationRecord }] }) => {
        if (disposed) return
        const payload = Array.isArray(event?.data) ? event.data[0] : event?.data
        const next = payload?.record
        if (!next || next.articleId !== articleID || next.targetLang !== targetLang || next.sourceMode !== sourceMode) return
        if (sourceVersion && next.sourceVersion !== sourceVersion) return
        client.setQueryData<TranslationRecord | null>(queryKey, current => latestTranslation(current ?? undefined, next))
      })
    } catch {
      // 浏览器预览或单测环境中 Wails Events 不可用时静默忽略。
    }
    return () => {
      disposed = true
      try { unsubscribe?.() } catch { /* ignore */ }
    }
  }, [articleID, client, queryKey, sourceMode, sourceVersion, targetLang])

  const canTranslate = Boolean(articleID) && hasContent && !extracting && !query.isPending && record !== null
  const translate = useCallback(async (force = false) => {
    if (!articleID || !canTranslate || running.current.has(requestKey)) return
    running.current.add(requestKey)
    setRequests(previous => ({ ...previous, [requestKey]: { running: true, errorCode: '' } }))
    let errorCode = ''
    try {
      const startBlockId = readStartBlock()
      const result = await translationAPI.translate({
        articleId: articleID,
        targetLang,
        sourceMode,
        force,
        ...(sourceVersion ? { sourceVersion } : {}),
        ...(startBlockId ? { startBlockId } : {}),
      })
      if (!sourceVersion || result.sourceVersion === sourceVersion) {
        client.setQueryData<TranslationRecord | null>(queryKey, current => latestTranslation(current ?? undefined, result))
      }
    } catch (cause) {
      errorCode = translationErrorCode(cause)
      // Wails can reject the RPC even if the backend saved a partial result.
      void client.invalidateQueries({ queryKey, exact: true })
    } finally {
      running.current.delete(requestKey)
      setRequests(previous => ({ ...previous, [requestKey]: { running: false, errorCode } }))
    }
  }, [articleID, canTranslate, client, queryKey, readStartBlock, requestKey, sourceMode, sourceVersion, targetLang])

  const cancel = useCallback(() => {
    if (!articleID) return
    void translationAPI.cancel(articleID, targetLang, sourceMode)
  }, [articleID, sourceMode, targetLang])

  // 订阅覆盖优先。手动选择临时目标语言只读取缓存。
  const autoOnOpen = feedAutoTranslateMode === 'on' || (feedAutoTranslateMode !== 'off' && translateOnOpen)
  useEffect(() => {
    if (!aiSettings.data || !canTranslate || !autoReady || !autoOnOpen || overrideLang || query.isFetching || record?.status !== 'none') return
    if (autoTriggered.current.has(requestKey)) return
    autoTriggered.current.add(requestKey)
    void translate(false)
  }, [aiSettings.data, autoOnOpen, autoReady, canTranslate, overrideLang, query.isFetching, record, requestKey, translate])

  const blocksMap = useMemo(() => {
    const map: Record<string, string> = {}
    for (const block of record?.blocks ?? []) {
      if (block.id && block.translatedHtml) {
        map[block.id] = block.translatedHtml
      }
    }
    return map
  }, [record?.blocks])

  const hasTranslation = Boolean(record && (Object.keys(blocksMap).length > 0 || record.translatedTitle))
  const effectiveDisplayMode: TranslationDisplayMode = hasTranslation ? displayMode : 'original'
  const request = requests[requestKey]
  const translating = Boolean(request?.running || record?.status === 'translating')
  const recordError = (record?.status === 'partial' || record?.status === 'failed') && record.errorCode
  const errorCode = request?.errorCode || recordError || ''

  return {
    record,
    targetLang,
    displayMode,
    effectiveDisplayMode,
    blocksMap,
    hasTranslation,
    bodyStatus: record?.status,
    canTranslate,
    translating,
    errorCode,
    error: errorCode ? translationErrorMessage(errorCode, usingAIModel) : '',
    usingAIModel,
    translate,
    cancel,
    setDisplayMode,
    setTargetLang: setOverrideLang,
  }
}
