import * as API from '../../bindings/github.com/coolerks/Fealdhord/internal/translation/api'
import type { LanguageOption, Record as TranslationRecord, TranslateRequest } from '../../bindings/github.com/coolerks/Fealdhord/internal/translation/models'

export type { LanguageOption, TranslationRecord, TranslateRequest }

export type TranslationDisplayMode = 'original' | 'bilingual' | 'translated'

export const translationProgressEvent = 'translation:progress'

export const defaultTranslationLanguages: LanguageOption[] = [
  { code: 'zh-CN', label: '简体中文', promptName: 'Simplified Chinese (简体中文)' },
  { code: 'zh-TW', label: '繁體中文', promptName: 'Traditional Chinese (繁體中文)' },
  { code: 'en', label: 'English', promptName: 'English' },
  { code: 'ja', label: '日本語', promptName: 'Japanese (日本語)' },
  { code: 'ko', label: '한국어', promptName: 'Korean (한국어)' },
  { code: 'fr', label: 'Français', promptName: 'French' },
  { code: 'de', label: 'Deutsch', promptName: 'German' },
  { code: 'es', label: 'Español', promptName: 'Spanish' },
  { code: 'ru', label: 'Русский', promptName: 'Russian' },
]

const errorMessages: Record<string, string> = {
  Busy: '该文章正在翻译中，请稍候。',
  BudgetExceeded: '已达到每小时文章 AI 预算上限。',
  ConfigurationError: '请先在「设置 → 服务与模型」的用途分配中为「翻译」选择已启用的模型；若不想看到此提示，可在「设置 → 文章 AI」关闭「打开文章时自动翻译正文」。',
  InvalidModelOutput: '模型返回的翻译格式无效，请重试。',
  NotFound: '文章不存在或所属订阅已归档。',
  RateLimitError: '模型服务限流，请稍后重试。',
  ReaderError: '无法读取或提取文章正文。',
  ContextWindowExceeded: '翻译内容超出所选模型的上下文预算。',
  Conflict: '正文已更新，请重新打开正文后继续翻译。',
  Timeout: '翻译请求超时，已翻段落已保存，可重试续翻。',
  Cancelled: '翻译已取消，已翻段落已保留。',
}

export function translationErrorMessage(error: unknown): string {
  const raw = error instanceof Error ? error.message : String(error ?? '')
  return Object.entries(errorMessages).find(([code]) => raw.includes(code))?.[1] ?? '翻译失败，请检查翻译模型设置后重试。'
}

export const translationAPI = {
  get(articleID: string, targetLang: string, sourceMode: string) {
    return API.Get(articleID, targetLang, sourceMode)
  },
  translate(input: TranslateRequest) {
    return API.Translate(input)
  },
  cancel(articleID: string, targetLang: string, sourceMode: string) {
    return API.Cancel(articleID, targetLang, sourceMode)
  },
  clear(articleID: string, targetLang: string) {
    return API.Clear(articleID, targetLang)
  },
  languages() {
    return API.Languages()
  },
}
