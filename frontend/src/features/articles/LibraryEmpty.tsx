import { useArticleAISettings } from '@/features/article-ai/settings-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { CircleAlert, Rss, Search, Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Empty, EmptyContent, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'

type Props = {
  scope: string
  searching: boolean
  loading: boolean
  failed: boolean
  incomplete?:boolean
  onRetry: () => void
  onClearSearch: () => void
  onSettings: (section: 'ai' | 'feeds') => void
}

export function LibraryEmpty({ scope, searching, loading, failed, incomplete=false, onRetry, onClearSearch, onSettings }: Props) {
  const featured = scope === 'featured'
  const settings = useArticleAISettings(featured)
  let title = '暂无文章'
  if (scope === 'favorites') title = '暂无收藏'
  if (scope === 'unread') title = '没有未读文章'
  if (featured) title = settings.data?.featured ? '暂无符合条件的精选文章' : '精选尚未启用'
  if (searching) title = incomplete ? '当前页没有匹配文章' : '没有找到匹配文章'
  if (loading) title = '正在读取文章…'
  if (failed) title = '暂时无法读取文章'
  const Icon = failed ? CircleAlert : searching ? Search : featured ? Sparkles : Rss

  let action = null
  if (failed) action = <Button variant="outline" onClick={onRetry}>重新读取文章</Button>
  else if (!loading && searching) action = <Button variant="outline" onClick={onClearSearch}>清除搜索</Button>
  else if (!loading && (featured || scope === 'all')) {
    action = <Button variant="outline" onClick={() => onSettings(featured ? 'ai' : 'feeds')}>{featured ? 'AI 设置' : '订阅管理'}</Button>
  }

  return <Empty className="library-empty">
    <EmptyHeader><EmptyMedia variant="icon"><AnimatedGlyph icon={Icon} meaning={failed?"badge-alert":searching?"search":featured?"ai":"radio-tower"}/></EmptyMedia><EmptyTitle>{title}</EmptyTitle></EmptyHeader>
    {action && <EmptyContent>{action}</EmptyContent>}
  </Empty>
}
