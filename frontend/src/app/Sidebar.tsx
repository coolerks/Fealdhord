import { SidebarAction } from '@/components/sidebar-action'
import { openChat } from '@/features/chat/chat-scope'
import { useMemo,useState } from 'react'
import type { ReactNode } from 'react'
import { useNavigation } from '@/features/categories/use-navigation'
import { categoryPath } from '@/features/categories/category-path'
import { categoryIcons } from '@/features/categories/category-icons'
import type { CategoryInfo } from '@/lib/navigation-api'
import { AnimatePresence, motion } from 'motion/react'
import { Circle, LibraryBig, Plus, Settings, Sparkles, Star } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { runCommand } from '@/lib/commands'
import { native } from '@/lib/native'
import { FeedIcon } from '@/features/feeds/FeedIcon'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { SemanticButton } from '@/components/semantic-button'
import { useWorkspace } from '@/stores/workspace'
import { cn } from '@/lib/utils'
import { moveSidebarFocus } from '@/lib/sidebar-navigation'

export const scopes = [
  { id: 'all', title: '全部文章', short: '全部', icon: LibraryBig },
  { id: 'featured', title: '精选', short: '精选', icon: Sparkles },
  { id: 'unread', title: '未读', short: '未读', icon: Circle },
  { id: 'favorites', title: '收藏', short: '收藏', icon: Star },
]
function Section({ title, onAdd,children }: { title: string; onAdd: () => void;children?:ReactNode }) {
  return <section className="nav-section"><div className="nav-section-heading"><h2>{title}</h2><SemanticButton icon={Plus} label={`管理${title}`} meaning="add" onClick={onAdd} /></div>{children}</section>
}
export function Sidebar({ motionMode, inChat = false, suspended = false }: { motionMode: 'full' | 'subtle' | 'off'; inChat?: boolean; suspended?: boolean }) {
  const client = useQueryClient()
  const navigation=useNavigation()
  const [organizeScrolled, setOrganizeScrolled] = useState(false)
  const [expanded,setExpanded]=useState<Set<string>>(()=>new Set())
  const children=useMemo(()=>{
    const map=new Map<string,CategoryInfo[]>()
    for(const category of navigation.data?.categories ?? []) {const siblings=map.get(category.parent_id);if(siblings)siblings.push(category);else map.set(category.parent_id,[category])}
    return map
  },[navigation.data])
  const openCommands = () => runCommand('palette', client)
  const preferences = useWorkspace(state => state.preferences)!
  const selectScope = useWorkspace(state => state.selectScope)
  const showSettings = useWorkspace(state => state.showSettings)
  const collapsed = preferences.sidebar_collapsed
  return <aside id={inChat ? undefined : 'workspace-sidebar'} className={cn('glass-sidebar workspace-mode-reading', inChat && 'chat-obscured', suspended && 'settings-obscured')} hidden={suspended} aria-label="主导航" inert={collapsed || inChat} aria-hidden={collapsed || inChat || undefined}>
    <div className="sidebar-titlebar" />
    <nav className="system-nav" aria-label="阅读视图" onKeyDown={moveSidebarFocus}>
    <SidebarAction id="open-command" label="命令面板" meaning="search" shortcut={native.isMac() ? 'Meta+K' : 'Control+K'} aria-haspopup="dialog" onClick={openCommands} />
    {scopes.map(({ id, title, short, icon: Icon }) => {
      const count = id==='all'||id==='unread' ? navigation.data?.totals.unread ?? 0 : id==='favorites' ? navigation.data?.totals.favorites ?? 0 : 0
      return <Button key={id} data-navigation={id} variant={preferences.selected_scope === id ? 'secondary' : 'ghost'} className={cn('nav-item', preferences.selected_scope === id && 'is-current')} aria-current={preferences.selected_scope === id ? 'page' : undefined} aria-label={title} onClick={() => selectScope(id)}>{preferences.selected_scope===id&&<motion.span className="nav-selected-surface" layoutId="active-navigation" transition={motionMode==='full'?{type:'spring',stiffness:420,damping:35}:{duration:0}}/>}<AnimatedGlyph icon={Icon} meaning={id==='all'?'book':id==='featured'?'ai':id==='unread'?'unread':'favorite'} /><span>{short}</span>{count > 0 && <span className="nav-count">{count}</span>}</Button>
    })}</nav>
    <div className="sidebar-organize-wrap" data-scrolled={organizeScrolled}>
      <div className="sidebar-organize" onScroll={event => setOrganizeScrolled(event.currentTarget.scrollTop > 0)}>
        <Section title="分类" onAdd={() => showSettings('categories')}>
          <nav aria-label="分类导航" onKeyDown={moveSidebarFocus}>{tree('', 0)}</nav>
        </Section>
        <Section title="订阅" onAdd={() => showSettings('feeds')}>
          <nav className="feed-nav" aria-label="订阅导航" onKeyDown={moveSidebarFocus}>
            {navigation.data?.feeds?.map(feed => {
              const active = preferences.selected_scope === 'feed' && preferences.selected_scope_id === feed.id
              return <Button key={feed.id} data-navigation={feed.id} variant={active ? 'secondary' : 'ghost'}
                className="nav-item" aria-current={active ? 'page' : undefined}
                aria-label={`浏览订阅 ${feed.title}`} onClick={() => selectScope('feed', feed.id)}>
                <FeedIcon data={preferences.remote_images === 'never' ? undefined : feed.icon_data} />
                <span className="nav-node-label" title={feed.title}>{feed.title}</span>
                {feed.unread > 0 && <span className="nav-count">{feed.unread}</span>}
              </Button>
            })}
          </nav>
        </Section>
      </div>
    </div>
    {navigation.isError && <Button variant="ghost" onClick={()=>{void navigation.refetch()}}>重试导航</Button>}
    <footer className="sidebar-footer"><div className="sidebar-bottom"><Button variant="ghost" id="open-global-chat" className="sidebar-ai" aria-label="AI 助手" onClick={() => openChat({kind:'global', id:'', label:'全局对话'})}><AnimatedGlyph icon={Sparkles} meaning="ai"/>AI 助手</Button><SemanticButton id="open-settings" icon={Settings} label="打开设置" meaning="settings" onClick={() => showSettings()} /></div></footer>
  </aside>
  function tree(parentID:string,depth:number):ReactNode {
    if(depth>=5) return null
    return <ul className="nav-tree">{children.get(parentID)?.map(category=>{
      const active=preferences.selected_scope==='category'&&preferences.selected_scope_id===category.id
      const hasChildren=Boolean(children.get(category.id)?.length),open=expanded.has(category.id)
      const icon=categoryIcons.find(icon=>icon===category.icon) ?? 'folder'
      return <li key={category.id}><div className="nav-tree-row" style={{paddingInlineStart:depth*16}}>{hasChildren && <Button variant="ghost" size="icon" className="nav-tree-disclosure" aria-label={`${open?'收起':'展开'}分类 ${category.name}`} aria-expanded={open} onClick={()=>setExpanded(current=>{const next=new Set(current);if(next.has(category.id))next.delete(category.id);else next.add(category.id);return next})}><span className="inline-flex" style={{rotate:open?'90deg':undefined}}><AnimatedGlyph meaning="chevron-right"/></span></Button>}<Button data-navigation={category.id} variant={active?'secondary':'ghost'} className="nav-item" aria-label={`浏览分类 ${categoryPath(category,navigation.data?.categories ?? [])}`} aria-current={active?'page':undefined} onClick={()=>selectScope('category', category.id)}><span className="inline-flex shrink-0" style={{color:/^#[a-f0-9]{6}$/i.test(category.color)?category.color:undefined}}><AnimatedGlyph meaning={icon}/></span><span className="nav-node-label">{category.name}</span>{category.unread>0&&<span className="nav-count">{category.unread}</span>}</Button></div>{hasChildren && (motionMode === 'off' ? open && tree(category.id,depth+1) :
        <div inert={!open} aria-hidden={!open || undefined}>
          <AnimatePresence initial={false}>
            {open && <motion.div key="children" className="overflow-hidden"
              initial={{height:0}} animate={{height:'auto'}} exit={{height:0}}
              transition={{duration:motionMode==='subtle'?.1:.2,ease:'easeInOut'}}>
              {tree(category.id,depth+1)}
            </motion.div>}
          </AnimatePresence>
        </div>)}</li>
    })}</ul>
  }

}
