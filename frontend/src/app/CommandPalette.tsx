import { Inbox, LayoutGrid, Moon, Plus, Search, Settings, Sparkles, Star, Sun } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandSeparator } from '@/components/ui/command'
import { useWorkspace } from '@/stores/workspace'
import { runCommand, commandLabels } from '@/lib/commands'
import type { CommandID } from '@/lib/commands'
const entries:{id:CommandID;icon:typeof Inbox}[]=[{id:'all',icon:Inbox},{id:'favorites',icon:Star},{id:'search',icon:Search},{id:'masonry',icon:LayoutGrid},{id:'settings',icon:Settings},{id:'feeds',icon:Plus},{id:'ai',icon:Sparkles},{id:'theme-light',icon:Sun},{id:'theme-dark',icon:Moon},{id:'help',icon:Settings}]
export function CommandPalette() {
 const open=useWorkspace(state=>state.paletteOpen),client=useQueryClient()
 const items=(start:number,end:number)=>entries.slice(start,end).map(({id,icon:Icon})=><CommandItem key={id} onSelect={()=>{useWorkspace.setState({paletteOpen:false});runCommand(id,client)}}><Icon/>{id==='settings'?'打开设置':commandLabels[id]}</CommandItem>)
 return <Dialog open={open} onOpenChange={value=>useWorkspace.setState({paletteOpen:value})}><DialogContent className="command-dialog" showCloseButton={false} onCloseAutoFocus={event=>{event.preventDefault();if(!useWorkspace.getState().settingsOpen)(document.getElementById('open-command')??document.getElementById('open-settings'))?.focus()}}><DialogHeader className="sr-only"><DialogTitle>命令面板</DialogTitle><DialogDescription>搜索视图、设置与操作，方向键选择，回车执行。</DialogDescription></DialogHeader><Command label="搜索命令"><CommandInput placeholder="搜索命令…" aria-label="搜索命令"/><CommandList aria-label="可用命令"><CommandEmpty>没有找到匹配命令</CommandEmpty><CommandGroup heading="阅读与导航">{items(0,4)}</CommandGroup><CommandSeparator/><CommandGroup heading="应用">{items(4,entries.length)}</CommandGroup></CommandList></Command></DialogContent></Dialog>
}
