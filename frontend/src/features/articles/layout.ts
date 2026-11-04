import type { Article } from '@/lib/library'

export function columnCount(width: number, mode: string) {
  return mode === 'list' ? 1 : Math.max(1, Math.floor((width + 12) / 272))
}
export function coverHeight(article: Article, width: number, imageMode: string) {
  if (!article.cover?.url.startsWith('/media/')) return 0
  const { width: sourceWidth, height: sourceHeight } = article.cover
  const ratio = imageMode === 'cropped' ? 1.6 : sourceWidth && sourceHeight ? Math.max(.65, Math.min(2, sourceWidth / sourceHeight)) : 1.6
  return Math.min(320, width / ratio)
}
export function findAnchor(items: { index: number; start: number; end: number }[], articles: Article[], scrollTop: number) {
  const first = items.find(item => item.end > scrollTop)
  return first && articles[first.index] ? { article_id: articles[first.index].id, offset: scrollTop - first.start } : null
}
