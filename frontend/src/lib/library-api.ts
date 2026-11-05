import * as Service from '../../bindings/github.com/coolerks/Fealdhord/internal/library/service'
import { APIError,unwrap } from './api'
import type { ArticleQuery } from './library-query'
import type { LibraryPage } from './library'

export const libraryAPI={
 async list(request:ArticleQuery={scope:{kind:'all'},sort:'published_desc',limit:50},signal?:AbortSignal):Promise<LibraryPage>{
  const call=Service.Query(request)
  const cancel=()=>call.cancel()
  if(signal?.aborted) call.cancel()
  signal?.addEventListener('abort',cancel,{once:true})
  let result
  try { result=await call } finally { signal?.removeEventListener('abort',cancel) }
  const page=unwrap(result)
  if(!page.items)throw new Error('文章列表响应不完整。')
  return {items:page.items.map(article=>({id:article.id,feedID:article.feed_id,url:article.url,title:article.title,titleMatches:article.title_matches ?? [],feedTitle:article.feed_title,publishedAt:article.sort_at,summary:article.summary,contentText:'',isRead:article.is_read,isFavorite:article.is_favorite,...(article.cover?{cover:{url:article.cover.url,width:article.cover.width??undefined,height:article.cover.height??undefined,alt:article.cover.alt}}:{})})),hasMore:page.has_more,nextCursor:page.next_cursor,snapshotID:page.snapshot_id,searchLimited:page.search_limited}
 },
 async rebuildSearch(){const result=await Service.RebuildSearch();if(!result.ok)throw new APIError(result.error ?? {code:'DatabaseError',message:'重建索引失败。',retryable:true})},
 async text(articleID:string){return unwrap(await Service.Get({article_id:articleID})).content_text},
 async setRead(articleID:string,value:boolean){unwrap(await Service.SetRead({article_id:articleID,value}))},
 async setFavorite(articleID:string,value:boolean){unwrap(await Service.SetFavorite({article_id:articleID,value}))},
}
