import { useState } from 'react'
import type { KeyboardEvent } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import { ChevronDown, Circle, Command, Folder, LibraryBig, PanelLeftClose, PanelLeftOpen, Plus, Rss, Search, Settings, Sparkles, Star } from 'lucide-react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { SemanticButton } from '@/components/semantic-button'
import { Separator } from '@/components/ui/separator'
import { useWorkspace } from '@/stores/workspace'
import type { Article } from '@/lib/library'
import { cn } from '@/lib/utils'

export const scopes = [
  { id: 'all', title: '全部文章', short: '全部', icon: LibraryBig },
  { id: 'featured', title: '精选', short: '精选', icon: Sparkles },
  { id: 'unread', title: '未读', short: '未读', icon: Circle },
  { id: 'favorites', title: '收藏', short: '收藏', icon: Star },
]
function Section({ title, icon: Icon, children, onAdd, motionMode }: { title: string; icon: typeof Folder; children: React.ReactNode; onAdd: () => void; motionMode: 'full' | 'subtle' | 'off' }) {
  const [expanded, setExpanded] = useState(true)
  return <section className="nav-section"><div className="nav-section-heading"><Button size="sm" variant="ghost" className="nav-section-toggle" aria-expanded={expanded} onClick={() => setExpanded(value => !value)}><ChevronDown data-icon="inline-start" className={cn(!expanded && 'is-folded')} />{title}</Button><SemanticButton icon={Plus} label={`管理${title}`} meaning="add" onClick={onAdd} /></div><AnimatePresence initial={false}>{expanded && <motion.div transition={{ height: { duration: motionMode === 'full' ? .22 : 0 } }} initial={{ opacity: 0, height: motionMode === 'full' ? 0 : 'auto' }} animate={{ opacity: 1, height: 'auto' }} exit={{ opacity: 0, height: 0 }} className="nav-section-body"><div className="nav-empty"><Icon /><span>{children}</span></div></motion.div>}</AnimatePresence></section>
}
export function Sidebar({ articles, motionMode }: { articles: Article[]; motionMode: 'full' | 'subtle' | 'off' }) {
  const preferences = useWorkspace(state => state.preferences)!
  const patch = useWorkspace(state => state.patch)
  const showSettings = useWorkspace(state => state.showSettings)
  const collapsed = preferences.sidebar_collapsed
  function keyboard(event: KeyboardEvent<HTMLElement>) {
    const buttons = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('[data-navigation]'))
    const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
    if (index < 0) return
    let next: number
    switch (event.key) { case 'ArrowDown': next = (index + 1) % buttons.length; break; case 'ArrowUp': next = (index - 1 + buttons.length) % buttons.length; break; case 'Home': next = 0; break; case 'End': next = buttons.length - 1; break; default: return }
    event.preventDefault(); buttons[next]?.focus()
  }
  return <aside className={cn('glass-sidebar', collapsed && 'is-collapsed')} aria-label="主导航">
    <div className="native-title-space"/><header className="sidebar-brand">{!collapsed && <div className="brand-copy"><strong>折栈</strong><span>你的阅读空间</span></div>}<SemanticButton icon={collapsed ? PanelLeftOpen : PanelLeftClose} label={collapsed ? '展开侧栏' : '折叠侧栏'} meaning="collapse" onClick={() => patch({ sidebar_collapsed: !collapsed })} /></header>
    {!collapsed && <Button id="open-command" variant="outline" className="sidebar-search" onClick={() => useWorkspace.setState({ paletteOpen: true })}><AnimatedGlyph icon={Search} meaning="search"/><span>搜索</span><kbd>⌘ K</kbd></Button>}
    <nav className="system-nav" aria-label="阅读视图" onKeyDown={keyboard}>{scopes.map(({ id, title, short, icon: Icon }) => {
      const count = id === 'all' || id === 'unread' ? articles.filter(article => !article.isRead).length : id === 'favorites' ? articles.filter(article => article.isFavorite).length : 0
      return <Button key={id} data-navigation={id} variant={preferences.selected_scope === id ? 'secondary' : 'ghost'} className={cn('nav-item', preferences.selected_scope === id && 'is-current')} aria-current={preferences.selected_scope === id ? 'page' : undefined} aria-label={title} title={collapsed ? title : undefined} onClick={() => patch({ selected_scope: id })}>{preferences.selected_scope===id&&<motion.span className="nav-selected-surface" layoutId="active-navigation" transition={motionMode==='full'?{type:'spring',stiffness:420,damping:35}:{duration:0}}/>}<AnimatedGlyph icon={Icon} meaning={id==='all'?'book':id==='featured'?'ai':id==='unread'?'unread':'favorite'} />{!collapsed && <><span>{short}</span><span className="nav-count">{count || '—'}</span></>}</Button>
    })}</nav>
    <Separator />
    <div className="sidebar-organize">{collapsed ? <div className="collapsed-sections"><SemanticButton icon={Folder} label="分类管理" onClick={() => showSettings('feeds')} /><SemanticButton icon={Rss} label="订阅管理" onClick={() => showSettings('feeds')} /></div> : <><Section motionMode={motionMode} title="分类" icon={Folder} onAdd={() => showSettings('feeds')}>还没有分类</Section><Section motionMode={motionMode} title="订阅" icon={Rss} onAdd={() => showSettings('feeds')}>还没有订阅</Section></>}</div>
    <footer className="sidebar-footer">{!collapsed && <div className="local-note"><span className="status-dot" />本地优先，安心阅读</div>}<div className="sidebar-bottom"><Button variant="ghost" size={collapsed ? 'icon' : 'default'} className="sidebar-ai" aria-label="AI 助手" onClick={() => showSettings('ai')}><AnimatedGlyph icon={Sparkles} meaning="ai"/>{!collapsed && 'AI 助手'}</Button><SemanticButton id="open-settings" icon={Settings} label="打开设置" meaning="settings" onClick={() => showSettings()} />{collapsed && <SemanticButton icon={Command} label="命令面板" onClick={() => useWorkspace.setState({ paletteOpen: true })} />}</div></footer>
  </aside>
}
