import { lazy, Suspense, useCallback, useEffect, useLayoutEffect, useMemo, useState } from 'react'
import type { CSSProperties } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, MotionConfig, motion } from 'motion/react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { CircleAlert } from 'lucide-react'
import { LibraryEmpty } from '@/features/articles/LibraryEmpty'
import { ApplicationMenu } from './ApplicationMenu'
import { native } from '@/lib/native'
import { isCommand,runCommand } from '@/lib/commands'
import { Dialog,DialogContent,DialogHeader,DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Splitter } from '@/components/splitter'
import { ArticleCollection } from '@/features/articles/ArticleCollection'
import { Reader } from '@/features/reader/Reader'
import { useWorkspace } from '@/stores/workspace'
import { effectiveMotion, useMediaQuery } from '@/lib/environment'
import { libraryAPI } from '@/lib/library-api'
import type { Article } from '@/lib/library'
import { articleQueryKey, filterArticles } from '@/lib/library'
import { Sidebar } from './Sidebar'
import { Toolbar } from './Toolbar'
import { cn } from '@/lib/utils'

const SettingsDialog = lazy(() => import('@/features/settings/SettingsDialog').then(module => ({ default: module.SettingsDialog })))
const CommandPalette = lazy(() => import('./CommandPalette').then(module => ({ default: module.CommandPalette })))

import { useRSSSync } from './use-rss-sync'
import { RefreshStatus } from '@/features/feeds/RefreshStatus'

const EMPTY_ARTICLES:Article[]=[]

export function Shell() {
  useRSSSync()
  const preferences = useWorkspace(state => state.preferences)!
  const patch = useWorkspace(state => state.patch)
  const search = useWorkspace(state => state.search)
  const sort = useWorkspace(state => state.sort)
  const showSettings = useWorkspace(state => state.showSettings)
  const helpOpen=useWorkspace(state=>state.helpOpen)
  const commandError=useWorkspace(state=>state.commandError)
  const settingsOpen = useWorkspace(state => state.settingsOpen)
  const paletteOpen = useWorkspace(state => state.paletteOpen)
  const saveStatus = useWorkspace(state => state.saveStatus)
  const client = useQueryClient()
  const { data:page, isLoading, isError,refetch } = useQuery({ queryKey: articleQueryKey, queryFn: libraryAPI.list, staleTime: 30000,retry:false })
  const data=page?.items??EMPTY_ARTICLES
  const articles = useMemo(() => filterArticles(data, preferences.selected_scope, search, sort), [data, preferences.selected_scope, search, sort])
  const selected = data.find(article => article.id === preferences.selected_article_id)
  const compact = useMediaQuery('(max-width: 1080px)')
  const systemDark = useMediaQuery('(prefers-color-scheme: dark)')
  const systemReduced = useMediaQuery('(prefers-reduced-motion: reduce)')
  const mode = effectiveMotion(preferences, systemReduced)
  const overlay = !preferences.reader_pinned || compact || preferences.article_view_mode === 'masonry'
  const readerVisible = preferences.reader_open && Boolean(selected)
  const [sidebarDraft, setSidebarDraft] = useState<number | null>(null)
  const [listDraft, setListDraft] = useState<number | null>(null)
  const sidebarWidth = preferences.sidebar_collapsed ? 64 : sidebarDraft ?? preferences.sidebar_width
  const listWidth = listDraft ?? preferences.list_width
  const queryKey = `${preferences.selected_scope}|${search}|${sort}`

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
  return <MotionConfig reducedMotion={mode === 'full' ? 'never' : 'always'} transition={{ duration: mode === 'off' ? 0 : mode === 'subtle' ? .1 : .22, ease: [.22, 1, .36, 1] }}>
    <div className="desktop-shell" data-platform={native.isMac()?'mac':'other'}>{!native.isMac()&&<ApplicationMenu/>}<div className="app-shell" style={{ '--sidebar-width': `${sidebarWidth}px`, '--list-width': `${listWidth}px` } as CSSProperties} data-view={preferences.article_view_mode} data-sidebar-collapsed={preferences.sidebar_collapsed}>
      <Sidebar articles={data} motionMode={mode} />
      {!preferences.sidebar_collapsed && <Splitter label="调整侧栏宽度" value={sidebarDraft ?? preferences.sidebar_width} min={220} max={280} onResize={setSidebarDraft} onCommit={value => { patch({ sidebar_width: value }); setSidebarDraft(null) }} />}
      <main className={cn('reading-workspace', !overlay && readerVisible && 'three-column')}>
        <section className="article-pane" aria-label="文章浏览"><Toolbar count={articles.length} overlay={overlay} hasSelection={Boolean(selected)} />{!settingsOpen && <RefreshStatus />}
          {articles.length > 0 && !isError ? <ArticleCollection articles={articles} preferences={preferences} queryKey={queryKey} selectedID={preferences.selected_article_id} onOpen={open} onFavorite={favorite} onRead={read} onAnchor={saveAnchor} /> : <LibraryEmpty scope={preferences.selected_scope} searching={emptySearch} loading={isLoading} failed={isError} onRetry={() => { void refetch() }} onClearSearch={() => useWorkspace.setState({ search: '' })} onSettings={showSettings} />}
          {page?.hasMore && <p className="library-limit" role="status">仅显示最新 {data.length} 篇</p>}
        </section>
        {!overlay && readerVisible && <><Splitter label="调整文章列表宽度" value={listDraft ?? preferences.list_width} min={300} max={420} onResize={setListDraft} onCommit={value => { patch({ list_width: value }); setListDraft(null) }} /><Reader article={selected} preferences={preferences} overlay={false} onClose={closeReader} onFavorite={favorite} onRead={read} onAnchor={saveAnchor} /></>}
        <AnimatePresence>{overlay && readerVisible && <motion.div className="reader-drawer" transition={mode === 'full' ? { type: 'spring', stiffness: 380, damping: 38 } : { duration: mode === 'off' ? 0 : .1 }} initial={mode === 'full' ? { x: 64, opacity: 0 } : { opacity: 0 }} animate={{ x: 0, opacity: 1 }} exit={mode === 'full' ? { x: 16, opacity: 0 } : { opacity: 0 }}><Reader article={selected} preferences={preferences} overlay onClose={closeReader} onFavorite={favorite} onRead={read} onAnchor={saveAnchor} /></motion.div>}</AnimatePresence>
      </main>
      <Suspense fallback={null}>{settingsOpen && <SettingsDialog />}{paletteOpen && <CommandPalette />}</Suspense>
      <div className="sr-only" aria-live="polite">{preferences.article_view_mode === 'masonry' ? '已切换到瀑布流视图' : '已切换到列表视图'}</div>
      {saveStatus === 'error' && <div className="save-warning" role="status"><AnimatedGlyph icon={CircleAlert} meaning="badge-alert"/><span>偏好保存失败</span><Button size="sm" variant="ghost" onClick={() => showSettings()}>查看</Button></div>}
      <Dialog open={helpOpen} onOpenChange={open=>useWorkspace.setState({helpOpen:open})}><DialogContent aria-describedby={undefined}><DialogHeader><DialogTitle>键盘快捷键</DialogTitle></DialogHeader><dl className="shortcut-list"><dt>下一篇／上一篇</dt><dd>J ／ K</dd><dt>收藏／切换已读</dt><dd>F ／ M</dd><dt>搜索文章</dt><dd>/</dd><dt>搜索与命令</dt><dd>⌘ ／ Ctrl + K</dd><dt>设置</dt><dd>⌘ ／ Ctrl + ,</dd><dt>刷新订阅</dt><dd>⌘ ／ Ctrl + R</dd><dt>返回浏览</dt><dd>Esc</dd><dt>Windows 菜单栏</dt><dd>F10</dd></dl></DialogContent></Dialog>
      {commandError&&<div role="alert" className="command-error">{commandError}<Button variant="ghost" onClick={()=>useWorkspace.setState({commandError:null})}>关闭</Button></div>}
    </div></div>
  </MotionConfig>
}
