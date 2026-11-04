import { useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Menubar, MenubarMenu, MenubarTrigger, MenubarContent, MenubarGroup, MenubarItem, MenubarSeparator } from '@/components/ui/menubar'
import { commandLabels, runCommand } from '@/lib/commands'
import type { CommandID } from '@/lib/commands'
import { captureTextTarget, editText } from '@/lib/text-actions'

const groups: {title:string;items:(CommandID|'separator')[]}[] = [
 {title:'文件',items:['feeds','separator','settings','close','quit']},
 {title:'显示',items:['all','unread','favorites','separator','list','masonry','sidebar','reader','separator','theme-system','theme-light','theme-dark','fullscreen']},
 {title:'文章',items:['previous','next','separator','favorite','read']},
 {title:'窗口',items:['minimize','maximize','palette']},
 {title:'帮助',items:['help','about']},
]
export function ApplicationMenu() {
 const client=useQueryClient()
 const text=useRef<ReturnType<typeof captureTextTarget> | null>(null)
 return <div className="application-menu" onPointerDownCapture={()=>{text.current=captureTextTarget()}} onFocusCapture={event=>{if(event.relatedTarget instanceof HTMLInputElement||event.relatedTarget instanceof HTMLTextAreaElement)text.current=captureTextTarget(event.relatedTarget)}}><Menubar aria-label="应用菜单栏">
 {groups.slice(0,1).map(group=>renderGroup(group))}
 <MenubarMenu><MenubarTrigger>编辑</MenubarTrigger><MenubarContent><MenubarGroup>{(['undo','redo','cut','copy','paste','selectAll'] as const).map((action,index)=><MenubarItem key={action} onSelect={()=>{void editText(action,text.current??captureTextTarget())}}>{['撤销','重做','剪切','复制','粘贴','全选'][index]}</MenubarItem>)}<MenubarSeparator/><MenubarItem onSelect={()=>runCommand('search',client)}>搜索文章</MenubarItem></MenubarGroup></MenubarContent></MenubarMenu>
 {groups.slice(1).map(group=>renderGroup(group))}
 </Menubar></div>
 function renderGroup(group:typeof groups[number]) {return <MenubarMenu key={group.title}><MenubarTrigger>{group.title}</MenubarTrigger><MenubarContent><MenubarGroup>{group.items.map((id,index)=>id==='separator'?<MenubarSeparator key={index}/>:<MenubarItem key={id} onSelect={()=>runCommand(id,client)}>{commandLabels[id]}</MenubarItem>)}{group.title==='文件'&&<><MenubarSeparator/><MenubarItem disabled>导入 OPML…</MenubarItem><MenubarItem disabled>导出 OPML…</MenubarItem></>}</MenubarGroup></MenubarContent></MenubarMenu>}
}
