import type { QueryClient } from '@tanstack/react-query'
import { articleQueryKey, filterArticles } from './library'
import type { Article } from './library'
import { useWorkspace } from '@/stores/workspace'
import { native } from './native'

export type CommandID = 'settings'|'about'|'help'|'feeds'|'ai'|'search'|'palette'|'all'|'unread'|'favorites'|'featured'|'list'|'masonry'|'sidebar'|'reader'|'theme-system'|'theme-light'|'theme-dark'|'next'|'previous'|'favorite'|'read'|'close'|'minimize'|'maximize'|'fullscreen'|'quit'
export const commandLabels: Record<CommandID, string> = {
 settings:'设置…',about:'关于折栈',help:'键盘快捷键',feeds:'订阅管理…',ai:'AI 与扩展…',search:'搜索文章',palette:'搜索与命令…',all:'全部文章',unread:'未读文章',favorites:'收藏文章',featured:'精选',list:'列表视图',masonry:'瀑布流视图',sidebar:'显示／隐藏侧栏',reader:'显示／隐藏阅读面板','theme-system':'跟随系统外观','theme-light':'浅色外观','theme-dark':'深色外观',next:'下一篇',previous:'上一篇',favorite:'收藏／取消收藏',read:'切换已读状态',close:'关闭窗口',minimize:'最小化',maximize:'缩放窗口',fullscreen:'切换全屏',quit:'退出折栈',
}
export function isCommand(id: string): id is CommandID { return Object.hasOwn(commandLabels,id) }
export function runCommand(id: CommandID, client: QueryClient, articleID?: string) {
 const state=useWorkspace.getState(), prefs=state.preferences
 if (!prefs) return
 const patch=state.patch
 switch(id) {
 case 'settings':state.showSettings();break
 case 'about':state.showSettings('about');break
 case 'help':useWorkspace.setState({ helpOpen:true });break
 case 'feeds':state.showSettings('feeds');break
 case 'ai':state.showSettings('ai');break
 case 'palette':useWorkspace.setState({ paletteOpen:true });break
 case 'search':patch({reader_open:false});useWorkspace.setState({searchOpen:true});requestAnimationFrame(()=>document.getElementById('article-search')?.focus());break
 case 'all':case 'unread':case 'favorites':case 'featured':patch({selected_scope:id});break
 case 'list':case 'masonry':patch({article_view_mode:id});break
 case 'sidebar':patch({sidebar_collapsed:!prefs.sidebar_collapsed});break
 case 'reader':patch({reader_open:!prefs.reader_open});break
 case 'theme-system':patch({theme:'system'});break
 case 'theme-light':patch({theme:'light'});break
 case 'theme-dark':patch({theme:'dark'});break
 case 'next':case 'previous': {
  const all=client.getQueryData<Article[]>(articleQueryKey)??[]
  const articles=filterArticles(all,prefs.selected_scope,state.search,state.sort)
  const current=articles.findIndex(article=>article.id===prefs.selected_article_id)
  const article=articles[Math.max(0,Math.min(articles.length-1,current<0?0:current+(id==='next'?1:-1)))]
  if(article)patch({selected_article_id:article.id,reader_open:true})
  break
 }
 case 'favorite':case 'read': {
  const target=articleID??prefs.selected_article_id
  client.setQueryData<Article[]>(articleQueryKey,current=>current?.map(article=>article.id===target?{...article,...(id==='favorite'?{isFavorite:!article.isFavorite}:{isRead:!article.isRead})}:article))
  break
 }
 case 'close':case 'minimize':case 'maximize':case 'fullscreen':case 'quit':
  void native[id]().catch(()=>useWorkspace.setState({commandError:'窗口操作失败，请重试。'}));break
 }
}
