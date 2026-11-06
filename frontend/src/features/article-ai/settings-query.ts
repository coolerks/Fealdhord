import { useQuery } from '@tanstack/react-query'
import { Settings } from '../../../bindings/github.com/coolerks/Fealdhord/internal/articleai/api'

export const articleAISettingsKey = ['article-ai', 'settings'] as const

export function useArticleAISettings(enabled = true) {
  return useQuery({ queryKey: articleAISettingsKey, queryFn: () => Settings(), enabled, retry: false })
}
