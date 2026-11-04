import { lazy, Suspense, useCallback, useEffect, useLayoutEffect, useMemo, useState } from 'react'
import type { CSSProperties } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, MotionConfig, motion } from 'motion/react'
import { CircleAlert, Rss, Search, Sparkles } from 'lucide-react'
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { ApplicationMenu } from './ApplicationMenu'
import { native } from '@/lib/native'
import { isCommand,runCommand } from '@/lib/commands'
import { IconMotionContext } from '@/components/animated-glyph'
import { Dialog,DialogContent,DialogHeader,DialogTitle,DialogDescription } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Splitter } from '@/components/splitter'
import { ArticleCollection } from '@/features/articles/ArticleCollection'
import { Reader } from '@/features/reader/Reader'
import { useWorkspace } from '@/stores/workspace'
import { effectiveMotion, useMediaQuery } from '@/lib/environment'
import { articleQueryKey, filterArticles, loadArticles } from '@/lib/library'
import { Sidebar } from './Sidebar'
import { Toolbar } from './Toolbar'
import { cn } from '@/lib/utils'

const SettingsDialog = lazy(() => import('@/features/settings/SettingsDialog').then(module => ({ default: module.SettingsDialog })))
const CommandPalette = lazy(() => import('./CommandPalette').then(module => ({ default: module.CommandPalette })))

export function Shell() {
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
  const { data = [], isLoading } = useQuery({ queryKey: articleQueryKey, queryFn: loadArticles, staleTime: Infinity })
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
      if (event.isComposing || event.defaultPrevented || useWorkspace.getState().settingsOpen) return
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
  const featured = preferences.selected_scope === 'featured'
  return <IconMotionContext.Provider value={mode}><MotionConfig reducedMotion={mode === 'full' ? 'never' : 'always'} transition={{ duration: mode === 'off' ? 0 : mode === 'subtle' ? .1 : .22, ease: [.22, 1, .36, 1] }}>
    <div className="desktop-shell" data-platform={native.isMac()?'mac':'other'}>{!native.isMac()&&<ApplicationMenu/>}<div className="app-shell" style={{ '--sidebar-width': `${sidebarWidth}px`, '--list-width': `${listWidth}px` } as CSSProperties} data-view={preferences.article_view_mode} data-sidebar-collapsed={preferences.sidebar_collapsed}>
      <Sidebar articles={data} motionMode={mode} />
      {!preferences.sidebar_collapsed && <Splitter label="调整侧栏宽度" value={sidebarDraft ?? preferences.sidebar_width} min={220} max={280} onResize={setSidebarDraft} onCommit={value => { patch({ sidebar_width: value }); setSidebarDraft(null) }} />}
      <main className={cn('reading-workspace', !overlay && readerVisible && 'three-column')}>
        <section className="article-pane" aria-label="文章浏览"><Toolbar count={articles.length} overlay={overlay} hasSelection={Boolean(selected)} />
          {articles.length > 0 ? <ArticleCollection articles={articles} preferences={preferences} queryKey={queryKey} selectedID={preferences.selected_article_id} onOpen={open} onFavorite={favorite} onRead={read} onAnchor={saveAnchor} /> : <Empty className="library-empty"><EmptyHeader><EmptyMedia variant="icon">{featured ? <Sparkles /> : emptySearch ? <Search /> : <Rss />}</EmptyMedia><EmptyTitle>{isLoading ? '正在读取文章…' : featured ? '把值得读的，留给你' : emptySearch ? '没有找到匹配文章' : preferences.selected_scope === 'favorites' ? '值得重读的内容，会在这里' : preferences.selected_scope === 'unread' ? '暂时没有未读文章' : '你的阅读，从这里开始'}</EmptyTitle><EmptyDescription>{featured ? '精选需要配置 AI 评分，当前未启用。全部、未读和收藏可独立使用。' : emptySearch ? '试试其他关键词，或清除搜索条件。' : '还没有文章。订阅接入后，这里会呈现你的信息流。'}</EmptyDescription></EmptyHeader><EmptyContent>{emptySearch ? <Button variant="outline" onClick={() => useWorkspace.setState({ search: '' })}>清除搜索</Button> : <Button variant="outline" onClick={() => showSettings(featured ? 'ai' : 'feeds')}>{featured ? <Sparkles data-icon="inline-start" /> : <Rss data-icon="inline-start" />}{featured ? '了解 AI 精选' : '了解订阅管理'}</Button>}</EmptyContent><span className="empty-view-note">{preferences.article_view_mode === 'masonry' ? '瀑布流 · 图文浏览' : '列表 · 高效阅读'}</span></Empty>}
          <footer className="article-status"><span><span className="status-dot" />{saveStatus === 'error' ? '偏好尚未保存' : saveStatus === 'saving' ? '正在保存偏好' : '本地阅读空间'}</span><span><kbd>J</kbd><kbd>K</kbd> 切换</span></footer>
        </section>
        {!overlay && readerVisible && <><Splitter label="调整文章列表宽度" value={listDraft ?? preferences.list_width} min={300} max={420} onResize={setListDraft} onCommit={value => { patch({ list_width: value }); setListDraft(null) }} /><Reader article={selected} preferences={preferences} overlay={false} onClose={closeReader} onFavorite={favorite} onRead={read} onAnchor={saveAnchor} /></>}
        <AnimatePresence>{overlay && readerVisible && <motion.div className="reader-drawer" transition={mode === 'full' ? { type: 'spring', stiffness: 380, damping: 38 } : { duration: mode === 'off' ? 0 : .1 }} initial={mode === 'full' ? { x: 64, opacity: 0 } : { opacity: 0 }} animate={{ x: 0, opacity: 1 }} exit={mode === 'full' ? { x: 16, opacity: 0 } : { opacity: 0 }}><Reader article={selected} preferences={preferences} overlay onClose={closeReader} onFavorite={favorite} onRead={read} onAnchor={saveAnchor} /></motion.div>}</AnimatePresence>
      </main>
      <Suspense fallback={null}>{settingsOpen && <SettingsDialog />}{paletteOpen && <CommandPalette />}</Suspense>
      <div className="sr-only" aria-live="polite">{preferences.article_view_mode === 'masonry' ? '已切换到瀑布流视图' : '已切换到列表视图'}</div>
      {saveStatus === 'error' && <div className="save-warning" role="status"><CircleAlert /><span>偏好保存失败</span><Button size="sm" variant="ghost" onClick={() => showSettings()}>查看</Button></div>}
      <Dialog open={helpOpen} onOpenChange={open=>useWorkspace.setState({helpOpen:open})}><DialogContent><DialogHeader><DialogTitle>键盘快捷键</DialogTitle><DialogDescription>让阅读跟上你的节奏。</DialogDescription></DialogHeader><dl className="shortcut-list"><dt>下一篇／上一篇</dt><dd>J ／ K</dd><dt>收藏／切换已读</dt><dd>F ／ M</dd><dt>搜索文章</dt><dd>/</dd><dt>搜索与命令</dt><dd>⌘ ／ Ctrl + K</dd><dt>设置</dt><dd>⌘ ／ Ctrl + ,</dd><dt>返回浏览</dt><dd>Esc</dd><dt>Windows 菜单栏</dt><dd>F10</dd></dl></DialogContent></Dialog>
      {commandError&&<div role="alert" className="command-error">{commandError}<Button variant="ghost" onClick={()=>useWorkspace.setState({commandError:null})}>关闭</Button></div>}
    </div></div>
  </MotionConfig></IconMotionContext.Provider>
}
