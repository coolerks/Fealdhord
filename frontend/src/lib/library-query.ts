import type { InfiniteData, QueryClient } from '@tanstack/react-query'
import type { ArticleQuery } from '../../bindings/github.com/coolerks/Fealdhord/internal/database/models'
import type { Article, LibraryPage } from './library'
import { articleQueryKey } from './library'

export type { ArticleQuery }
export type LibraryData = InfiniteData<LibraryPage>
export function libraryKey(query:ArticleQuery) {
  const {cursor: _cursor, ...key} = query
  void _cursor
  if((!query.limit || query.limit===50) && query.scope.kind==='all' && !query.search && query.sort==='published_desc' && query.favorite===undefined && (!query.read || query.read==='all')) return articleQueryKey
  return [...articleQueryKey,key] as const
}
export function libraryArticles(data:LibraryData|undefined):Article[] {
  return data?.pages.flatMap(page=>page.items) ?? []
}
export function findCachedArticle(client:QueryClient,id:string) {
  for(const [,data] of client.getQueriesData<LibraryData>({queryKey:articleQueryKey})) {
    const found=libraryArticles(data).find(article=>article.id===id)
    if(found) return found
  }
}
export function updateCachedArticle(client:QueryClient,id:string,patch:Partial<Article>) {
  client.setQueriesData<LibraryData>({queryKey:articleQueryKey},data=>data ? {...data,pages:data.pages.map(page=>({...page,items:page.items.map(article=>article.id===id ? {...article,...patch}:article)}))} : data)
}
export function visibleArticles(articles:Article[],query:ArticleQuery) {
  return articles.filter(article=>
    !((query.scope.kind==='unread' || query.read==='unread') && article.isRead) &&
    !(query.read==='read' && !article.isRead) &&
    !((query.scope.kind==='favorites' || query.favorite===true) && !article.isFavorite) &&
    !(query.favorite===false && article.isFavorite))
}
export function initialLibraryData(page:LibraryPage):LibraryData {return {pages:[page],pageParams:[undefined]}}
