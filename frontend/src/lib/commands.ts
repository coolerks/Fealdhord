import type { QueryClient } from '@tanstack/react-query'
import { articleQueryKey } from './library'
import { libraryArticles,visibleArticles } from './library-query'
import type { LibraryData } from './library-query'
import { toggleArticleState } from './article-state'
import { useWorkspace } from '@/stores/workspace'
import { startRefresh } from './refresh'
import { readerAPI } from './reader-api'
import { native } from './native'

export type CommandID = 'import-opml'|'export-opml'|'reader-mode'|'open-original'|'categories'|'refresh'|'settings'|'about'|'diagnostics'|'devtools'|'help'|'feeds'|'ai'|'search'|'palette'|'all'|'unread'|'favorites'|'featured'|'list'|'masonry'|'sidebar'|'reader'|'theme-system'|'theme-light'|'theme-dark'|'next'|'previous'|'favorite'|'read'|'close'|'minimize'|'maximize'|'fullscreen'|'quit'
export const commandLabels: Record<CommandID, string> = {
 'import-opml':'导入 OPML…','export-opml':'导出 OPML…','reader-mode':'切换 Reader 模式','open-original':'在浏览器打开原文',categories:'分类管理…',refresh:'刷新订阅',settings:'设置…',about:'关于折栈',diagnostics:'连接诊断…',devtools:'开发者工具',help:'键盘快捷键',feeds:'订阅管理…',ai:'AI 与扩展…',search:'搜索文章',palette:'命令面板…',all:'全部文章',unread:'未读文章',favorites:'收藏文章',featured:'精选',list:'列表视图',masonry:'瀑布流视图',sidebar:'显示／隐藏侧栏',reader:'显示／隐藏阅读面板','theme-system':'跟随系统外观','theme-light':'浅色外观','theme-dark':'深色外观',next:'下一篇',previous:'上一篇',favorite:'收藏／取消收藏',read:'切换已读状态',close:'关闭窗口',minimize:'最小化',maximize:'缩放窗口',fullscreen:'切换全屏',quit:'退出折栈',
}
export function isCommand(id: string): id is CommandID { return Object.hasOwn(commandLabels,id) }
export function runCommand(id: CommandID, client: QueryClient, articleID?: string) {
 const state=useWorkspace.getState(), prefs=state.preferences
 if (!prefs) return
 const patch=state.patch
 switch(id) {
 case 'import-opml':case 'export-opml':state.showSettings('data');useWorkspace.setState({transferIntent:id==='import-opml'?'import':'export'});break
 case 'categories':state.showSettings('categories');break
 case 'reader-mode':if(articleID||prefs.selected_article_id){patch({reader_open:true});useWorkspace.setState({readerModeRequestID:articleID??prefs.selected_article_id})}break
 case 'open-original':if(articleID||prefs.selected_article_id)void readerAPI.open(articleID??prefs.selected_article_id).catch(error=>useWorkspace.setState({commandError:error instanceof Error?error.message:'无法打开原文。'}));break
 case 'refresh':void startRefresh(client);break
 case 'settings':state.showSettings();break
 case 'about':state.showSettings('about');break
 case 'diagnostics':useWorkspace.setState({ diagnosticsOpen:true });break
 case 'devtools':
  if (prefs.developer_mode) void native.openDevTools().catch(() => useWorkspace.setState({commandError:'无法打开开发者工具。'}))
  break
 case 'help':useWorkspace.setState({ helpOpen:true });break
 case 'feeds':state.showSettings('feeds');break
 case 'ai':state.showSettings('ai');break
 case 'palette':useWorkspace.setState({ paletteOpen:true });break
 case 'search':patch({reader_open:false});useWorkspace.setState({searchOpen:true});requestAnimationFrame(()=>document.getElementById('article-search')?.focus());break
 case 'all':case 'unread':case 'favorites':case 'featured':patch({selected_scope:id,selected_scope_id:''});break
 case 'list':case 'masonry':patch({article_view_mode:id});break
 case 'sidebar':patch({sidebar_collapsed:!prefs.sidebar_collapsed});break
 case 'reader':patch({reader_open:!prefs.reader_open});break
 case 'theme-system':patch({theme:'system'});break
 case 'theme-light':patch({theme:'light'});break
 case 'theme-dark':patch({theme:'dark'});break
 case 'next':case 'previous': {
  const articles=visibleArticles(libraryArticles(client.getQueryData<LibraryData>(state.activeArticleQueryKey ?? articleQueryKey)),{scope:{kind:prefs.selected_scope,id:prefs.selected_scope_id},sort:'published_desc',read:state.readFilter,favorite:state.favoriteFilter==='all'?undefined:state.favoriteFilter==='favorites'})
  const current=articles.findIndex(article=>article.id===prefs.selected_article_id)
  const article=articles[Math.max(0,Math.min(articles.length-1,current<0?0:current+(id==='next'?1:-1)))]
  if(article)patch({selected_article_id:article.id,reader_open:true})
  break
 }
 case 'favorite':case 'read': {
  const target=articleID??prefs.selected_article_id
  toggleArticleState(client,target,id==='favorite'?'favorite':'read')
  break
 }
 case 'close':case 'minimize':case 'maximize':case 'fullscreen':case 'quit':
  void native[id]().catch(()=>useWorkspace.setState({commandError:'窗口操作失败，请重试。'}));break
 }
}
