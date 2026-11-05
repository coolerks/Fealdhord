import { ArrowDownWideNarrow, LayoutGrid, List, PanelRightClose, PanelRightOpen, RefreshCw, Search, SlidersHorizontal, X } from 'lucide-react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuLabel, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuSeparator, DropdownMenuTrigger, DropdownMenuItem } from '@/components/ui/dropdown-menu'
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from '@/components/ui/input-group'
import { SemanticButton } from '@/components/semantic-button'
import { useWorkspace } from '@/stores/workspace'
import { scopes } from './Sidebar'
import { runCommand } from '@/lib/commands'
import { useQueryClient } from '@tanstack/react-query'

export function Toolbar({ count, overlay, hasSelection }: { count: number; overlay: boolean; hasSelection: boolean }) {
  const client=useQueryClient()
  const preferences = useWorkspace(state => state.preferences)!
  const patch = useWorkspace(state => state.patch)
  const search = useWorkspace(state => state.search)
  const searchOpen = useWorkspace(state => state.searchOpen)
  const sort = useWorkspace(state => state.sort)
  const showSettings = useWorkspace(state => state.showSettings)
  const title = scopes.find(scope => scope.id === preferences.selected_scope)?.title ?? '全部文章'
  return <><header className="articles-toolbar"><div className="articles-heading"><h1>{title}</h1><span>{count} 篇</span></div><div className="toolbar-actions" role="group" aria-label="文章操作">
    <SemanticButton icon={RefreshCw} label="订阅接入后可刷新" meaning="refresh" disabled />
    <SemanticButton icon={Search} label="搜索文章" meaning="search" active={searchOpen} onClick={() => {if(searchOpen)useWorkspace.setState({searchOpen:false});else runCommand('search',client)}} />
    <SemanticButton id="reader-toggle" disabled={overlay && !hasSelection} icon={preferences.reader_open ? PanelRightClose : PanelRightOpen} label={overlay && !hasSelection ? '选择文章后打开阅读面板' : preferences.reader_open ? '收起阅读面板' : '展开阅读面板'} meaning="panel-right-open" onClick={() => patch({ reader_open: !preferences.reader_open })} />
  </div><div className="view-toolbar"><ToggleGroup type="single" value={preferences.article_view_mode} onValueChange={value => { if (value === 'list' || value === 'masonry') patch({ article_view_mode: value }) }} spacing={0} variant="outline" aria-label="文章视图"><ToggleGroupItem value="list" aria-label="列表视图" title="列表视图" className="meaning-list"><AnimatedGlyph icon={List} meaning="list"/></ToggleGroupItem><ToggleGroupItem value="masonry" aria-label="瀑布流视图" title="瀑布流视图" className="meaning-grid"><AnimatedGlyph icon={LayoutGrid} meaning="grid"/></ToggleGroupItem></ToggleGroup>
    <DropdownMenu><DropdownMenuTrigger asChild><Button variant="ghost" size="icon" aria-label="排序与显示选项"><AnimatedGlyph icon={SlidersHorizontal} meaning="sliders-horizontal"/></Button></DropdownMenuTrigger><DropdownMenuContent align="end"><DropdownMenuGroup><DropdownMenuLabel>排序</DropdownMenuLabel><DropdownMenuRadioGroup value={sort} onValueChange={value => { if (value === 'newest' || value === 'oldest') useWorkspace.setState({ sort: value }) }}><DropdownMenuRadioItem value="newest"><AnimatedGlyph icon={ArrowDownWideNarrow} meaning="arrow-down-0-1"/>最新在前</DropdownMenuRadioItem><DropdownMenuRadioItem value="oldest"><AnimatedGlyph icon={ArrowDownWideNarrow} meaning="arrow-down-1-0"/>最早在前</DropdownMenuRadioItem></DropdownMenuRadioGroup></DropdownMenuGroup><DropdownMenuSeparator /><DropdownMenuGroup><DropdownMenuItem onSelect={() => showSettings('appearance')}><AnimatedGlyph meaning="palette"/>外观与显示设置</DropdownMenuItem></DropdownMenuGroup></DropdownMenuContent></DropdownMenu>
  </div></header>
  {searchOpen && <div className="search-row"><InputGroup><InputGroupAddon><AnimatedGlyph icon={Search} meaning="search"/></InputGroupAddon><InputGroupInput id="article-search" aria-label="搜索当前文章" placeholder="搜索标题、来源、摘要…" maxLength={256} value={search} onChange={event => useWorkspace.setState({ search: event.target.value })} /><InputGroupAddon align="inline-end"><InputGroupButton aria-label="清除并关闭搜索" size="icon-xs" onClick={() => useWorkspace.setState({ search: '', searchOpen: false })}><AnimatedGlyph icon={X} meaning="x"/></InputGroupButton></InputGroupAddon></InputGroup></div>}
  </>
}
