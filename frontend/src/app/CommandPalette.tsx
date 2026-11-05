import { useQueryClient } from '@tanstack/react-query'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandSeparator } from '@/components/ui/command'
import { useWorkspace } from '@/stores/workspace'
import { runCommand } from '@/lib/commands'
import type { CommandID } from '@/lib/commands'
import { CommandPresentation } from '@/components/command-presentation'
import { commandShortcut } from '@/lib/command-presentation'
import { native } from '@/lib/native'
const entries:CommandID[]=['all','favorites','search','masonry','settings','feeds','ai','theme-light','theme-dark','help']
export function CommandPalette() {
 const open=useWorkspace(state=>state.paletteOpen),client=useQueryClient()
 const items=(start:number,end:number)=>entries.slice(start,end).map(id=><CommandItem key={id} aria-keyshortcuts={commandShortcut(id,native.isMac())?.keys} onSelect={()=>{useWorkspace.setState({paletteOpen:false});runCommand(id,client)}}><CommandPresentation id={id} label={id==='settings'?'打开设置':undefined}/></CommandItem>)
 return <Dialog open={open} onOpenChange={value=>useWorkspace.setState({paletteOpen:value})}><DialogContent className="command-dialog" showCloseButton={false} onCloseAutoFocus={event=>{event.preventDefault();if(!useWorkspace.getState().settingsOpen)(document.getElementById('open-command')??document.getElementById('open-settings'))?.focus()}}><DialogHeader className="sr-only"><DialogTitle>命令面板</DialogTitle><DialogDescription>搜索视图、设置与操作，方向键选择，回车执行。</DialogDescription></DialogHeader><Command label="搜索命令"><CommandInput placeholder="搜索命令…" aria-label="搜索命令"/><CommandList aria-label="可用命令"><CommandEmpty>没有找到匹配命令</CommandEmpty><CommandGroup heading="阅读与导航">{items(0,4)}</CommandGroup><CommandSeparator/><CommandGroup heading="应用">{items(4,entries.length)}</CommandGroup></CommandList></Command></DialogContent></Dialog>
}
