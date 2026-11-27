import * as API from '../../bindings/github.com/coolerks/Fealdhord/internal/translation/api'
import type {
  LanguageOption,
  ProviderConfig,
  ProviderMeta,
  Record as TranslationRecord,
  SaveProviderInput,
  TestResult,
  TranslateRequest,
} from '../../bindings/github.com/coolerks/Fealdhord/internal/translation/models'

export type {
  LanguageOption,
  ProviderConfig,
  ProviderMeta,
  SaveProviderInput,
  TestResult,
  TranslationRecord,
  TranslateRequest,
}

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

export const defaultTranslationProviders: ProviderMeta[] = [
  {
    id: 'ai', label: 'AI 模型', group: 'ai',
    aliases: ['ai', 'llm', 'model', '大模型', '模型'], requiresCredentials: false, unofficialFree: false,
    supportedLanguages: ['zh-CN', 'zh-TW', 'en', 'ja', 'ko', 'fr', 'de', 'es', 'ru'],
  },
  {
    id: 'microsoft_edge', label: 'Microsoft Edge（免费）', group: 'free',
    aliases: ['edge', 'microsoft', 'bing', '微软', '免费'], requiresCredentials: false, unofficialFree: true,
    supportedLanguages: ['zh-CN', 'zh-TW', 'en', 'ja', 'ko', 'fr', 'de', 'es', 'ru'],
  },
  {
    id: 'google_free', label: 'Google 翻译（免费）', group: 'free',
    aliases: ['google', 'gtx', 'te_lib', '谷歌', '免费'], requiresCredentials: false, unofficialFree: true,
    supportedLanguages: ['zh-CN', 'zh-TW', 'en', 'ja', 'ko', 'fr', 'de', 'es', 'ru'],
  },
  {
    id: 'microsoft_azure', label: 'Microsoft Azure', group: 'byok',
    aliases: ['azure', 'microsoft', '微软', 'translator'], requiresCredentials: true, unofficialFree: false,
    supportedLanguages: ['zh-CN', 'zh-TW', 'en', 'ja', 'ko', 'fr', 'de', 'es', 'ru'],
  },
  {
    id: 'google_byok', label: 'Google Cloud', group: 'byok',
    aliases: ['google', 'gcp', 'cloud', '谷歌'], requiresCredentials: true, unofficialFree: false,
    supportedLanguages: ['zh-CN', 'zh-TW', 'en', 'ja', 'ko', 'fr', 'de', 'es', 'ru'],
  },
  {
    id: 'alibaba', label: '阿里云机器翻译', group: 'byok',
    aliases: ['alibaba', 'aliyun', 'ali', '阿里', '阿里云'], requiresCredentials: true, unofficialFree: false,
    supportedLanguages: ['zh-CN', 'zh-TW', 'en', 'ja', 'ko', 'fr', 'de', 'es', 'ru'],
  },
  {
    id: 'caiyun', label: '彩云小译', group: 'byok',
    aliases: ['caiyun', 'lingocloud', '彩云', '彩云小译'], requiresCredentials: true, unofficialFree: false,
    supportedLanguages: ['zh-CN', 'zh-TW', 'en', 'ja', 'ko'],
  },
  {
    id: 'baidu', label: '百度翻译', group: 'byok',
    aliases: ['baidu', 'fanyi', '百度'], requiresCredentials: true, unofficialFree: false,
    supportedLanguages: ['zh-CN', 'zh-TW', 'en', 'ja', 'ko', 'fr', 'de', 'es', 'ru'],
  },
  {
    id: 'youdao', label: '有道智云', group: 'byok',
    aliases: ['youdao', 'netease', '有道', '网易有道'], requiresCredentials: true, unofficialFree: false,
    supportedLanguages: ['zh-CN', 'zh-TW', 'en', 'ja', 'ko', 'fr', 'de', 'es', 'ru'],
  },
]

const errorMessages: Record<string, string> = {
  Busy: '该文章正在翻译中，请稍候。',
  BudgetExceeded: '已达到每小时文章 AI 预算上限。',
  ConfigurationError: '请先在「设置 → 文章 AI」完成当前翻译方式的凭据配置（使用 AI 模型时需在「服务与模型」分配翻译模型）。',
  UnsupportedLanguage: '当前翻译方式不支持该源语言与目标语言组合，请切换目标语言或翻译方式。',
  InvalidModelOutput: '仍有内容待译，可继续翻译。',
  TranslationRefused: '翻译服务未返回这部分译文。',
  ProviderConnectionError: '翻译服务连接中断，请检查网络或代理设置后重试。',
  ProviderProtocolError: '翻译服务响应异常，可继续翻译。',
  ProviderHTTPError: '翻译服务请求失败，请检查配额、区域或服务状态后重试。',
  AuthenticationError: '翻译服务认证失败，请检查 API Key、密钥或开通状态。',
  CredentialError: '无法读取翻译服务凭据。',
  StorageError: '保存译文失败，可继续翻译。',
  NotFound: '文章不存在或所属订阅已归档。',
  RateLimitError: '翻译服务触发频率或额度限流，请稍后重试。',
  ReaderError: '无法读取或提取文章正文。',
  ContextWindowExceeded: '翻译内容超出所选模型的上下文预算。',
  Conflict: '正文已更新，请重新打开正文后继续翻译。',
  Timeout: '翻译请求超时，已翻段落已保存，可重试续翻。',
  Cancelled: '翻译已取消，已翻段落已保留。',
}

export function translationErrorMessage(error: unknown): string {
  const raw = error instanceof Error ? error.message : String(error ?? '')
  return Object.entries(errorMessages).find(([code]) => raw.includes(code))?.[1] ?? '翻译失败，请检查翻译设置后重试。'
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
  providers() {
    return API.Providers()
  },
  getProviderConfig(id: string) {
    return API.GetProviderConfig(id)
  },
  saveProviderConfig(input: SaveProviderInput) {
    return API.SaveProviderConfig(input)
  },
  testProvider(id: string, draft: SaveProviderInput | null) {
    return API.TestProvider(id, draft)
  },
}
