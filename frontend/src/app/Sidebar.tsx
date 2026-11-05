import type { KeyboardEvent } from 'react'
import { motion } from 'motion/react'
import { Circle, Command, Folder, LibraryBig, PanelLeftClose, PanelLeftOpen, Plus, Rss, Search, Settings, Sparkles, Star } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { runCommand } from '@/lib/commands'
import { native } from '@/lib/native'
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
function Section({ title, onAdd }: { title: string; onAdd: () => void }) {
  return <section className="nav-section"><div className="nav-section-heading"><h2>{title}</h2><SemanticButton icon={Plus} label={`管理${title}`} meaning="add" onClick={onAdd} /></div></section>
}
export function Sidebar({ articles, motionMode }: { articles: Article[]; motionMode: 'full' | 'subtle' | 'off' }) {
  const client = useQueryClient()
  const openCommands = () => runCommand('palette', client)
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
    <div className="native-title-space"/><header className="sidebar-brand">{!collapsed && <div className="brand-copy"><strong>折栈</strong></div>}<SemanticButton icon={collapsed ? PanelLeftOpen : PanelLeftClose} label={collapsed ? '展开侧栏' : '折叠侧栏'} meaning={collapsed ? 'expand' : 'collapse'} onClick={() => patch({ sidebar_collapsed: !collapsed })} /></header>
    {!collapsed && <Button id="open-command" variant="outline" className="sidebar-search" aria-label="命令面板" aria-keyshortcuts={native.isMac() ? "Meta+K" : "Control+K"} onClick={openCommands}><AnimatedGlyph icon={Search} meaning="search"/><span>命令面板</span><kbd aria-hidden="true">{native.isMac() ? '⌘ K' : 'Ctrl K'}</kbd></Button>}
    <nav className="system-nav" aria-label="阅读视图" onKeyDown={keyboard}>{scopes.map(({ id, title, short, icon: Icon }) => {
      const count = id === 'all' || id === 'unread' ? articles.filter(article => !article.isRead).length : id === 'favorites' ? articles.filter(article => article.isFavorite).length : 0
      return <Button key={id} data-navigation={id} variant={preferences.selected_scope === id ? 'secondary' : 'ghost'} className={cn('nav-item', preferences.selected_scope === id && 'is-current')} aria-current={preferences.selected_scope === id ? 'page' : undefined} aria-label={title} title={collapsed ? title : undefined} onClick={() => patch({ selected_scope: id })}>{preferences.selected_scope===id&&<motion.span className="nav-selected-surface" layoutId="active-navigation" transition={motionMode==='full'?{type:'spring',stiffness:420,damping:35}:{duration:0}}/>}<AnimatedGlyph icon={Icon} meaning={id==='all'?'book':id==='featured'?'ai':id==='unread'?'unread':'favorite'} />{!collapsed && <><span>{short}</span>{count > 0 && <span className="nav-count">{count}</span>}</>}</Button>
    })}</nav>
    <Separator />
    <div className="sidebar-organize">{collapsed ? <div className="collapsed-sections"><SemanticButton icon={Folder} label="分类管理" onClick={() => showSettings('feeds')} /><SemanticButton icon={Rss} label="订阅管理" onClick={() => showSettings('feeds')} /></div> : <><Section title="分类" onAdd={() => showSettings('feeds')} /><Section title="订阅" onAdd={() => showSettings('feeds')} /></>}</div>
    <footer className="sidebar-footer"><div className="sidebar-bottom"><Button variant="ghost" size={collapsed ? 'icon' : 'default'} className="sidebar-ai" aria-label="AI 助手" onClick={() => showSettings('ai')}><AnimatedGlyph icon={Sparkles} meaning="ai"/>{!collapsed && 'AI 助手'}</Button><SemanticButton id="open-settings" icon={Settings} label="打开设置" meaning="settings" onClick={() => showSettings()} />{collapsed && <SemanticButton icon={Command} label="命令面板" onClick={openCommands} />}</div></footer>
  </aside>
}
