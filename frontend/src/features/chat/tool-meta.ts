import type { LucideIcon } from 'lucide-react'
import {
  Bell,
  CalendarClock,
  FileText,
  FolderTree,
  List,
  Rss,
  ScanText,
  Search,
  Settings,
  Wrench,
} from 'lucide-react'

export type ToolMeta = {
  label: string
  icon: LucideIcon
}

const exactTools: Record<string, ToolMeta> = {
  article_list: { label: '列出文章', icon: List },
  'article.list': { label: '列出文章', icon: List },
  article_search: { label: '搜索文章', icon: Search },
  'article.search': { label: '搜索文章', icon: Search },
  article_get: { label: '读取文章正文', icon: FileText },
  'article.get': { label: '读取文章正文', icon: FileText },
  article_read: { label: '读取文章正文', icon: FileText },
  article_get_metadata: { label: '读取文章摘要', icon: FileText },
  article_extract: { label: '提取文章正文', icon: ScanText },
  'article.extract': { label: '提取文章正文', icon: ScanText },
  read_article: { label: '读取授权文章', icon: FileText },
  library_search: { label: '搜索本地文章', icon: Search },
  find_app_tools: { label: '查找可用工具', icon: Search },
  run_app_tool: { label: '调用工具', icon: Wrench },
  'schedule.create': { label: '创建定时任务', icon: CalendarClock },
  'schedule.update': { label: '更新定时任务', icon: CalendarClock },
  'schedule.delete': { label: '删除定时任务', icon: CalendarClock },
  'schedule.list': { label: '查看定时任务', icon: CalendarClock },
}

export function getToolMeta(name: string): ToolMeta {
  if (Object.hasOwn(exactTools, name)) return exactTools[name]
  const lower = name.toLowerCase()
  if (lower.includes('search') || lower.includes('find')) return { label: name, icon: Search }
  if (lower.startsWith('schedule.') || lower.includes('schedule')) return { label: name, icon: CalendarClock }
  if (lower.startsWith('notification.') || lower.includes('notify')) return { label: name, icon: Bell }
  if (lower.startsWith('category.') || lower.includes('categor')) return { label: name, icon: FolderTree }
  if (lower.startsWith('feed.') || lower.includes('rss')) return { label: name, icon: Rss }
  if (lower.startsWith('article.') || lower.includes('article')) return { label: name, icon: FileText }
  if (lower.startsWith('settings.') || lower.includes('config')) return { label: name, icon: Settings }
  return { label: name, icon: Wrench }
}
