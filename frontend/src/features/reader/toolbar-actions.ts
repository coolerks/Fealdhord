import { BookText, CheckCheck, ExternalLink, Folder, Languages, RefreshCw, ScanText, Sparkles, Star } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { IconMeaning } from '@/components/animated-glyph'

export const readerToolbarOptions = [
  { value: 'refresh', label: '重新提取正文', icon: RefreshCw, meaning: 'refresh' },
  { value: 'favorite', label: '收藏当前文章', icon: Star, meaning: 'favorite' },
  { value: 'translate', label: '翻译（暂未开放）', icon: Languages, meaning: 'languages' },
  { value: 'ai', label: '文章 AI 助手', icon: Sparkles, meaning: 'ai' },
  { value: 'categories', label: '文章分类', icon: Folder, meaning: 'folder' },
  { value: 'read', label: '切换当前文章已读状态', icon: CheckCheck, meaning: 'read' },
  { value: 'view', label: '文章显示方式', icon: BookText, meaning: 'book-text' },
  { value: 'original', label: '在浏览器打开原文', icon: ExternalLink, meaning: 'external-link' },
] as const satisfies ReadonlyArray<{ value: string; label: string; icon: LucideIcon; meaning: IconMeaning }>

export const readerViews = [
  { value: 'feed', label: '订阅预览', icon: BookText, meaning: 'book-text' },
  { value: 'reader', label: '阅读模式', icon: ScanText, meaning: 'scan-text' },
] as const

export type ReaderToolbarAction = typeof readerToolbarOptions[number]['value']
export type ArticleView = typeof readerViews[number]['value']
