import { useChatTarget } from '@/features/chat/chat-scope'
import { lazy, Suspense, useCallback, useEffect, useLayoutEffect, useState } from 'react'
import type { CSSProperties } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, MotionConfig, motion } from 'motion/react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { CircleAlert, PanelLeftClose, PanelLeftOpen } from 'lucide-react'
import { LibraryEmpty } from '@/features/articles/LibraryEmpty'
import { ApplicationMenu } from './ApplicationMenu'
import { native } from '@/lib/native'
import { captureTextTarget, editText } from '@/lib/text-actions'
import { isCommand,runCommand } from '@/lib/commands'
import { Dialog,DialogContent,DialogHeader,DialogTitle } from '@/components/ui/dialog'
import { SemanticButton } from '@/components/semantic-button'
import { Button } from '@/components/ui/button'
import { Splitter } from '@/components/splitter'
import { ArticleCollection } from '@/features/articles/ArticleCollection'
import { Reader } from '@/features/reader/Reader'
import { useWorkspace } from '@/stores/workspace'
import { effectiveMotion, useMediaQuery } from '@/lib/environment'
import { useLibraryQuery } from '@/features/articles/use-library-query'
import { findCachedArticle,libraryAnchorKey } from '@/lib/library-query'
import { Sidebar } from './Sidebar'
import { Toolbar } from './Toolbar'
import { cn } from '@/lib/utils'

const ChatDialog = lazy(() => import('@/features/chat/ChatDialog').then(module => ({ default: module.ChatDialog })))
const SettingsDialog = lazy(() => import('@/features/settings/SettingsDialog').then(module => ({ default: module.SettingsDialog })))
const CommandPalette = lazy(() => import('./CommandPalette').then(module => ({ default: module.CommandPalette })))

import { feedsQueryKey, rssAPI } from '@/lib/rss-api'
import type { Page } from '@/lib/rss-api'
import { useRSSSync } from './use-rss-sync'
import { RefreshStatus } from '@/features/feeds/RefreshStatus'


const readFeedIcons = (page: Page) => new Map(page.items?.map(feed => [feed.id, feed.icon_data]))

export function Shell() {
  useRSSSync()
  const chatOpen = useChatTarget(state => Boolean(state.target))
  const preferences = useWorkspace(state => state.preferences)!
  const patch = useWorkspace(state => state.patch)
  const search = useWorkspace(state => state.search)
  const showSettings = useWorkspace(state => state.showSettings)
  const helpOpen=useWorkspace(state=>state.helpOpen)
  const commandError=useWorkspace(state=>state.commandError)
  const settingsOpen = useWorkspace(state => state.settingsOpen)
  const paletteOpen = useWorkspace(state => state.paletteOpen)
  const saveStatus = useWorkspace(state => state.saveStatus)
  const client = useQueryClient()
  const library=useLibraryQuery()
  const {articles,data,isLoading,isError,refetch}=library
  const selected = data.find(article=>article.id===preferences.selected_article_id) ?? findCachedArticle(client,preferences.selected_article_id)
  const { data: feedIcons } = useQuery({
    queryKey: feedsQueryKey, queryFn: rssAPI.list, enabled: preferences.remote_images !== 'never',
    retry: false, select: readFeedIcons,
  })
  const selectedFeedIcon = selected?.feedID ? feedIcons?.get(selected.feedID) : undefined
  const systemDark = useMediaQuery('(prefers-color-scheme: dark)')
  const systemReduced = useMediaQuery('(prefers-reduced-motion: reduce)')
  const mode = effectiveMotion(preferences, systemReduced)
  const overlay = preferences.article_view_mode === 'masonry'
  const fillReader = overlay && preferences.reader_layout === 'fill'
  const slideReader = mode === 'full' && !fillReader
  const readerVisible = preferences.reader_open && Boolean(selected)
  const [sidebarDraft, setSidebarDraft] = useState<number | null>(null)
  const [listDraft, setListDraft] = useState<number | null>(null)
  const sidebarCollapsed = preferences.sidebar_collapsed
  const sidebarWidth = sidebarDraft ?? preferences.sidebar_width
  const listWidth = listDraft ?? preferences.list_width
  const queryKey = libraryAnchorKey(library.key)

  useLayoutEffect(() => {
    const dark = preferences.theme === 'dark' || (preferences.theme === 'system' && systemDark)
    document.documentElement.classList.toggle('dark', dark)
    document.documentElement.dataset.theme = preferences.theme
    document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
    document.documentElement.dataset.motion = mode
  }, [preferences.theme, systemDark, mode])

  const favorite = useCallback((id:string)=>runCommand('favorite',client,id),[client])
  const read = useCallback((id:string)=>runCommand('read',client,id),[client])
  const open = useCallback((id: string) => patch({ selected_article_id: id, reader_open: true }), [patch])
  const saveAnchor = useCallback((key: string, anchor: { article_id: string; offset: number }) => {
    const prefs = useWorkspace.getState().preferences
    if (!prefs) return
    const anchors = { ...prefs.scroll_anchors }
    delete anchors[key]
    anchors[key] = anchor
    for (const old of Object.keys(anchors).slice(0, Math.max(0, Object.keys(anchors).length - 32))) delete anchors[old]
    patch({ scroll_anchors: anchors })
  }, [patch])
  const closeReader = useCallback(() => {
    patch({ reader_open: false })
    const id = useWorkspace.getState().preferences?.selected_article_id
    requestAnimationFrame(() => {
      const trigger = id ? document.querySelector<HTMLElement>(`[data-article-trigger="${CSS.escape(id)}"]`) : null
      ;(trigger ?? document.getElementById('reader-toggle'))?.focus()
    })
  }, [patch])

  useEffect(() => {
    const keyboard = (event: KeyboardEvent) => {
      if (event.isComposing || event.defaultPrevented) return
      if (!native.isMac() && (event.metaKey || event.ctrlKey) && event.key.toLowerCase()==='r') { event.preventDefault();runCommand('refresh',client);return }
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'a' &&
        !(event.target instanceof Element && event.target.closest('input, textarea, [contenteditable=true]'))) {
        event.preventDefault()
        if (readerVisible && !useWorkspace.getState().settingsOpen && !useWorkspace.getState().paletteOpen) {
          void editText('selectAll', captureTextTarget())
        }
        return
      }
      if (useWorkspace.getState().settingsOpen) return
      if (!native.isMac() && event.key==='F10') {event.preventDefault();document.querySelector<HTMLElement>('[data-slot=menubar-trigger]')?.focus();return}
      if ((event.metaKey||event.ctrlKey) && !native.isMac()) {
        const id=event.key===','?'settings':event.key.toLowerCase()==='f'?'search':event.key==='1'?'all':event.key==='2'?'unread':event.key==='3'?'favorites':null
        if(id){event.preventDefault();runCommand(id,client);return}
      }
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault(); useWorkspace.setState({ paletteOpen: !useWorkspace.getState().paletteOpen }); return
      }
      if (event.metaKey || event.ctrlKey || event.altKey) return
      const target = event.target
      if (target instanceof Element && target.closest('input,textarea,select,[contenteditable=true],[role=dialog],[role=menu]')) return
      const key = event.key.toLowerCase()
      if (key === '/') { event.preventDefault(); if (overlay && readerVisible) patch({ reader_open: false }); useWorkspace.setState({ searchOpen: true }); requestAnimationFrame(() => document.getElementById('article-search')?.focus()) }
      if ((key === 'j' || key === 'k') && articles.length) {
        event.preventDefault()
        const current = articles.findIndex(article => article.id === preferences.selected_article_id)
        const index = Math.max(0, Math.min(articles.length - 1, current < 0 ? 0 : current + (key === 'j' ? 1 : -1)))
        open(articles[index].id)
        if (!overlay) requestAnimationFrame(() => document.querySelector<HTMLElement>(`[data-article-trigger="${CSS.escape(articles[index].id)}"]`)?.focus({ preventScroll: true }))
      }
      if (key === 'f' && selected) { event.preventDefault(); favorite(selected.id) }
      if (key === 'm' && selected) { event.preventDefault(); read(selected.id) }
      if (key === 'escape' && readerVisible) closeReader()
    }
    window.addEventListener('keydown', keyboard)
    return () => window.removeEventListener('keydown', keyboard)
  }, [articles, preferences.selected_article_id, open, selected, favorite, read, readerVisible, closeReader, overlay, patch, client])

  useEffect(()=>native.onCommand(id=>{if(isCommand(id))runCommand(id,client)}),[client])

  const emptySearch = search.trim() !== ''
  const platform = native.isMac() ? 'mac' : native.isWindows() ? 'windows' : 'other'
  const sidebarToggle = <SemanticButton id="sidebar-toggle" icon={sidebarCollapsed ? PanelLeftOpen : PanelLeftClose}
    label={sidebarCollapsed ? '展开侧栏' : '折叠侧栏'} meaning={sidebarCollapsed ? 'expand' : 'collapse'}
    aria-expanded={!sidebarCollapsed} aria-controls="workspace-sidebar"
    onClick={() => runCommand('sidebar', client)} />
  return <MotionConfig reducedMotion={mode === 'full' ? 'never' : 'always'} transition={{ duration: mode === 'off' ? 0 : mode === 'subtle' ? .1 : .22, ease: [.22, 1, .36, 1] }}>
    <div className="desktop-shell" data-platform={platform}>
      {platform !== 'mac' && <ApplicationMenu windowsSidebarToggle={platform === 'windows' ? sidebarToggle : undefined} />}
      <div className="app-shell" style={{ '--sidebar-width': `${sidebarWidth}px`, '--list-width': `${listWidth}px` } as CSSProperties} data-view={preferences.article_view_mode} data-sidebar-collapsed={sidebarCollapsed} data-sidebar-resizing={sidebarDraft !== null}>
      {platform !== 'windows' && <div className="sidebar-toggle">{sidebarToggle}</div>}
      <div className="sidebar-track"><Sidebar motionMode={mode} /></div>
      {!preferences.sidebar_collapsed && <Splitter label="调整侧栏宽度" value={sidebarDraft ?? preferences.sidebar_width} min={220} max={360} onResize={setSidebarDraft}
        onCollapse={() => {
          patch({ sidebar_collapsed: true })
          document.getElementById('sidebar-toggle')?.focus()
        }} onCommit={value => { patch({ sidebar_width: value }); setSidebarDraft(null) }} />}
      <main className={cn('reading-workspace', !overlay && 'reader-columns')}>
        <section className="article-pane" aria-label="文章浏览" inert={fillReader && readerVisible} aria-hidden={fillReader && readerVisible || undefined}><Toolbar count={articles.length} overlay={overlay} hasSelection={Boolean(selected)} />{!settingsOpen && <RefreshStatus />}
          {articles.length > 0 ? <ArticleCollection feedIcons={feedIcons} articles={articles} preferences={preferences} queryKey={queryKey} selectedID={preferences.selected_article_id} onOpen={open} onFavorite={favorite} onRead={read} onAnchor={saveAnchor} /> : <LibraryEmpty scope={preferences.selected_scope} searching={emptySearch} loading={isLoading} failed={isError} incomplete={library.hasNextPage} onRetry={() => { void refetch() }} onClearSearch={() => useWorkspace.setState({ search: '' })} onSettings={showSettings} />}
          {library.isError && search.trim() && <div className="library-limit"><Button variant="ghost" disabled={library.repairing} onClick={()=>{void library.repairSearch()}}>{library.repairing?'正在重建索引…':'重建搜索索引'}</Button>{library.repairError && <p role="alert">{library.repairError}</p>}</div>}
          {library.searchLimited && <p className="library-limit" role="status">搜索候选较多，可继续载入或缩小范围。</p>}
          {library.searchPending && <p className="library-limit" role="status">正在准备搜索…</p>}
          {library.isFetchNextPageError && <p role="alert">{library.expired?'列表已变化，请重新载入。':'继续读取失败。'}<Button variant="ghost" onClick={()=>{void (library.expired?refetch():library.fetchNextPage())}}>重试</Button></p>}
          {library.hasNextPage && <div className="library-limit"><Button variant="ghost" disabled={library.isFetching} onClick={()=>{void library.fetchNextPage()}}>{library.isFetchingNextPage?'正在载入…':'载入更多文章'}</Button></div>}
        </section>
        {!overlay && <><Splitter label="调整文章列表宽度" value={listDraft ?? preferences.list_width} min={300} max={420} onResize={setListDraft} onCommit={value => { patch({ list_width: value }); setListDraft(null) }} /><Reader feedIcon={selectedFeedIcon} article={readerVisible ? selected : undefined} preferences={preferences} overlay={false} onClose={closeReader} onFavorite={favorite} onRead={read} onAnchor={saveAnchor} /></>}
        <AnimatePresence>{overlay && readerVisible && <motion.div className={cn('reader-overlay', fillReader ? 'reader-fill' : 'reader-drawer')} transition={slideReader ? { type: 'spring', stiffness: 380, damping: 38 } : { duration: mode === 'off' ? 0 : .1 }} initial={slideReader ? { x: 64, opacity: 0 } : { opacity: 0 }} animate={{ x: 0, opacity: 1 }} exit={slideReader ? { x: 16, opacity: 0 } : { opacity: 0 }}><Reader feedIcon={selectedFeedIcon} key={selected?.id} article={selected} preferences={preferences} overlay onClose={closeReader} onFavorite={favorite} onRead={read} onAnchor={saveAnchor} /></motion.div>}</AnimatePresence>
      </main>
      <Suspense fallback={null}>{settingsOpen && <SettingsDialog />}{paletteOpen && <CommandPalette />}{chatOpen && <ChatDialog />}</Suspense>
      <div className="sr-only" aria-live="polite">{preferences.article_view_mode === 'masonry' ? '已切换到瀑布流视图' : '已切换到列表视图'}</div>
      {saveStatus === 'error' && <div className="save-warning" role="status"><AnimatedGlyph icon={CircleAlert} meaning="badge-alert"/><span>偏好保存失败</span><Button size="sm" variant="ghost" onClick={() => showSettings()}>查看</Button></div>}
      <Dialog open={helpOpen} onOpenChange={open=>useWorkspace.setState({helpOpen:open})}><DialogContent aria-describedby={undefined}><DialogHeader><DialogTitle>键盘快捷键</DialogTitle></DialogHeader><dl className="shortcut-list"><dt>下一篇／上一篇</dt><dd>J ／ K</dd><dt>收藏／切换已读</dt><dd>F ／ M</dd><dt>搜索文章</dt><dd>/</dd><dt>搜索与命令</dt><dd>⌘ ／ Ctrl + K</dd><dt>设置</dt><dd>⌘ ／ Ctrl + ,</dd><dt>刷新订阅</dt><dd>⌘ ／ Ctrl + R</dd><dt>返回浏览</dt><dd>Esc</dd><dt>Windows 菜单栏</dt><dd>F10</dd></dl></DialogContent></Dialog>
      {commandError&&<div role="alert" className="command-error">{commandError}<Button variant="ghost" onClick={()=>useWorkspace.setState({commandError:null})}>关闭</Button></div>}
    </div></div>
  </MotionConfig>
}
