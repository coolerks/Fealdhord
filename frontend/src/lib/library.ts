export type Article = {
  id: string
  title: string
  feedTitle: string
  publishedAt: string
  summary: string
  contentText: string
  isRead: boolean
  isFavorite: boolean
  aiLabel?: string
  cover?: { url: string; width?: number; height?: number; alt?: string }
}
export const articleQueryKey = ['library', 'articles'] as const
// Phase 1 仅提供真实空库入口；RSS 采集和持久化由后续阶段替换此边界。
export async function loadArticles(): Promise<Article[]> { return [] }

export function filterArticles(articles: Article[], scope: string, search: string, sort: 'newest' | 'oldest') {
  const term = search.trim().toLocaleLowerCase()
  return articles.filter(article => {
    if (scope === 'featured') return false
    if (scope === 'unread' && article.isRead) return false
    if (scope === 'favorites' && !article.isFavorite) return false
    return !term || `${article.title}\n${article.feedTitle}\n${article.summary}`.toLocaleLowerCase().includes(term)
  }).sort((a, b) => {
    const order = a.publishedAt.localeCompare(b.publishedAt) || a.id.localeCompare(b.id)
    return sort === 'newest' ? -order : order
  })
}
