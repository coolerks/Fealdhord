import * as Service from '../../bindings/github.com/coolerks/Fealdhord/internal/library/service'
import { unwrap } from './api'
import type { LibraryPage } from './library'

export const libraryAPI={
 async list():Promise<LibraryPage>{
  const page=unwrap(await Service.List({limit:500}))
  if(!page.items)throw new Error('文章列表响应不完整。')
  return {items:page.items.map(article=>({id:article.id,title:article.title,feedTitle:article.feed_title,publishedAt:article.sort_at,summary:article.summary,contentText:'',isRead:article.is_read,isFavorite:article.is_favorite,...(article.cover?{cover:{url:article.cover.url,width:article.cover.width??undefined,height:article.cover.height??undefined,alt:article.cover.alt}}:{})})),hasMore:page.has_more}
 },
 async text(articleID:string){return unwrap(await Service.Get({article_id:articleID})).content_text},
 async setRead(articleID:string,value:boolean){unwrap(await Service.SetRead({article_id:articleID,value}))},
 async setFavorite(articleID:string,value:boolean){unwrap(await Service.SetFavorite({article_id:articleID,value}))},
}
