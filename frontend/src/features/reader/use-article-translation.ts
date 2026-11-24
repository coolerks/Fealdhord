import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Events } from '@wailsio/runtime'
import { useArticleAISettings } from '@/features/article-ai/settings-query'
import { translationAPI, translationErrorMessage, translationProgressEvent } from '@/lib/translation-api'
import type { TranslationDisplayMode, TranslationRecord } from '@/lib/translation-api'

function normalizeDisplayMode(value?: string): TranslationDisplayMode {
  return value === 'translated' || value === 'original' ? value : 'bilingual'
}

export function useArticleTranslation({
  articleID,
  articleRevision,
  sourceMode,
  hasContent,
  extracting,
  defaultDisplay,
  feedAutoTranslateMode,
}: {
  articleID?: string
  articleRevision?: number
  sourceMode: 'feed' | 'reader'
  hasContent: boolean
  extracting: boolean
  defaultDisplay?: string
  feedAutoTranslateMode?: string
}) {
  const client = useQueryClient()
  const aiSettings = useArticleAISettings(Boolean(articleID))
  const defaultTargetLang = aiSettings.data?.translationTargetLang || 'zh-CN'
  const translateOnOpen = aiSettings.data?.translateOnOpen ?? false

  const [overrideLang, setOverrideLang] = useState<string>()
  const [displayMode, setDisplayMode] = useState<TranslationDisplayMode>(() => normalizeDisplayMode(defaultDisplay))
  const [translating, setTranslating] = useState(false)
  const [error, setError] = useState('')
  const autoTriggered = useRef('')

  const targetLang = overrideLang || defaultTargetLang
  const queryKey = useMemo(
    () => ['reader', 'translation', articleID, articleRevision ?? 0, targetLang, sourceMode] as const,
    [articleID, articleRevision, targetLang, sourceMode],
  )

  const query = useQuery({
    queryKey,
    queryFn: () => translationAPI.get(articleID!, targetLang, sourceMode),
    enabled: Boolean(articleID) && hasContent && !extracting,
    staleTime: 60000,
    retry: false,
  })

  const record = query.data

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
        client.setQueryData(queryKey, next)
        if (next.status === 'translating') {
          setTranslating(true)
          setError('')
        } else {
          setTranslating(false)
          if (next.status === 'failed' && next.errorCode) {
            setError(translationErrorMessage(next.errorCode))
          }
        }
      })
    } catch {
      // 浏览器预览或单测环境中 Wails Events 不可用时静默忽略。
    }
    return () => {
      disposed = true
      try { unsubscribe?.() } catch { /* ignore */ }
    }
  }, [articleID, client, queryKey, sourceMode, targetLang])

  const translate = useCallback(async (force = false, langOverride?: string) => {
    if (!articleID) return
    const activeLang = langOverride || targetLang
    setTranslating(true)
    setError('')
    if (displayMode === 'original') {
      setDisplayMode(normalizeDisplayMode(defaultDisplay) === 'original' ? 'bilingual' : normalizeDisplayMode(defaultDisplay))
    }
    try {
      const result = await translationAPI.translate({
        articleId: articleID,
        targetLang: activeLang,
        sourceMode,
        force,
      })
      client.setQueryData(['reader', 'translation', articleID, articleRevision ?? 0, activeLang, sourceMode], result)
      if (result.status === 'failed' && result.errorCode) {
        setError(translationErrorMessage(result.errorCode))
      }
    } catch (cause) {
      setError(translationErrorMessage(cause))
    } finally {
      setTranslating(false)
    }
  }, [articleID, articleRevision, client, defaultDisplay, displayMode, sourceMode, targetLang])

  const cancel = useCallback(() => {
    if (!articleID) return
    void translationAPI.cancel(articleID, targetLang, sourceMode)
    setTranslating(false)
  }, [articleID, sourceMode, targetLang])

  const changeTargetLang = useCallback((nextLang: string) => {
    setOverrideLang(nextLang)
    setError('')
    if (articleID && hasContent && !extracting) {
      void translate(false, nextLang)
    }
  }, [articleID, extracting, hasContent, translate])

  // 打开文章时自动翻译正文（尊重 translateOnOpen 设置与单源 off 覆盖）。
  useEffect(() => {
    if (!articleID || !hasContent || extracting || !translateOnOpen) return
    if (feedAutoTranslateMode === 'off') return
    if (!record || record.status !== 'none') return
    const key = `${articleID}:${articleRevision ?? 0}:${targetLang}:${sourceMode}`
    if (autoTriggered.current === key) return
    autoTriggered.current = key
    void translate(false)
  }, [articleID, articleRevision, extracting, feedAutoTranslateMode, hasContent, record, sourceMode, targetLang, translate, translateOnOpen])

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

  return {
    record,
    targetLang,
    displayMode,
    effectiveDisplayMode,
    blocksMap,
    hasTranslation,
    translating: translating || record?.status === 'translating',
    error,
    translate,
    cancel,
    setDisplayMode,
    setTargetLang: changeTargetLang,
  }
}
