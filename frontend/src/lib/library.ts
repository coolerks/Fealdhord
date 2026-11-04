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
export type LibraryPage={items:Article[];hasMore:boolean}

export function filterArticles(articles: Article[], scope: string, search: string, sort: 'newest' | 'oldest') {
  const term = search.trim().toLocaleLowerCase()
  return articles.filter(article => {
    if (scope === 'featured') return false
    if (scope === 'unread' && article.isRead) return false
    if (scope === 'favorites' && !article.isFavorite) return false
    return !term || `${article.title}\n${article.feedTitle}\n${article.summary}`.toLocaleLowerCase().includes(term)
  }).sort((a, b) => {
    const left=Date.parse(a.publishedAt),right=Date.parse(b.publishedAt)
    const order = (Number.isFinite(left)&&Number.isFinite(right)?left-right:0) || a.id.localeCompare(b.id)
    return sort === 'newest' ? -order : order
  })
}
