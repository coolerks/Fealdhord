import { AnimatedGlyph } from '@/components/animated-glyph'
import { memo, useState } from 'react'
import { Bookmark, Circle, CircleCheck, Sparkles, Star } from 'lucide-react'
import type { Article } from '@/lib/library'
import { SemanticButton } from '@/components/semantic-button'
import { coverHeight } from './layout'
import { cn } from '@/lib/utils'

type Props = {
  imagesAllowed?: boolean; article: Article; mode: string; selected: boolean; thumbnail: string; width: number; imageMode: string
  onOpen: (id: string) => void; onFavorite: (id: string) => void; onRead: (id: string) => void
}
export const ArticleItem = memo(function ArticleItem({ imagesAllowed=true, article, mode, selected, thumbnail, width, imageMode, onOpen, onFavorite, onRead }: Props) {
  const [failedURL, setFailedURL] = useState('')
  const imageHeight = coverHeight(article, width, imageMode)
  const showCover = imagesAllowed && imageHeight > 0 && article.cover?.url !== failedURL && (mode === 'masonry' || thumbnail === 'small')
  const time = new Date(article.publishedAt)
  const label = Number.isNaN(time.getTime()) ? '时间未知' : new Intl.DateTimeFormat('zh-CN', { month: 'short', day: 'numeric' }).format(time)
  return <div className={cn('article-item', mode === 'masonry' ? 'article-card' : 'article-row', selected && 'is-selected', article.isRead && 'is-read')} data-article-id={article.id}>
    {showCover && article.cover && <div className="article-cover" style={mode === 'masonry' ? { height: imageHeight } : undefined}><img src={article.cover.url} width={article.cover.width} height={article.cover.height} alt={article.cover.alt ?? ''} loading="lazy" decoding="async" onError={() => setFailedURL(article.cover?.url ?? '')} /></div>}
    <div className="article-body">
      <div className="article-meta"><span className="article-source">{article.feedTitle}</span><time dateTime={article.publishedAt}>{label}</time></div>
      <button type="button" className="article-title" data-article-trigger={article.id} aria-label={`打开文章：${article.title}`} aria-current={selected || undefined} onClick={() => onOpen(article.id)}><span className="unread-marker" aria-label={article.isRead ? '已读' : '未读'} />{article.title}</button>
      {mode === 'masonry' && <p className="article-summary">{article.summary}</p>}
      <div className="article-flags">{article.isFavorite && <span role="img" aria-label="已收藏" className="favorite-indicator"><AnimatedGlyph icon={Star} meaning="favorite"/></span>}{article.aiLabel && <span className="ai-indicator"><AnimatedGlyph icon={Sparkles} meaning="ai"/>{article.aiLabel}</span>}</div>
    </div>
    <div className="article-quick-actions"><SemanticButton icon={article.isFavorite ? Star : Bookmark} meaning="favorite" active={article.isFavorite} label={article.isFavorite ? '取消收藏' : '收藏文章'} onClick={() => onFavorite(article.id)} /><SemanticButton icon={article.isRead ? Circle : CircleCheck} meaning={article.isRead?"unread":"read"} label={article.isRead ? '标记未读' : '标记已读'} onClick={() => onRead(article.id)} /></div>
  </div>
})
