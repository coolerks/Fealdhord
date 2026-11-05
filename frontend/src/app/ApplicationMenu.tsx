import { Fragment, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Menubar, MenubarMenu, MenubarTrigger, MenubarContent, MenubarGroup, MenubarItem, MenubarSeparator } from '@/components/ui/menubar'
import { runCommand } from '@/lib/commands'
import type { CommandID } from '@/lib/commands'
import { captureTextTarget, editText } from '@/lib/text-actions'
import { CommandPresentation, TextActionPresentation } from '@/components/command-presentation'
import { commandShortcut, textShortcut } from '@/lib/command-presentation'
import { native } from '@/lib/native'

const groups: {title:string;sections:CommandID[][]}[] = [
 {title:'文件',sections:[['feeds','categories','refresh'],['import-opml','export-opml'],['settings','close','quit']]},
 {title:'显示',sections:[['all','unread','favorites'],['list','masonry','sidebar','reader'],['theme-system','theme-light','theme-dark'],['fullscreen']]},
 {title:'文章',sections:[['previous','next'],['reader-mode','open-original'],['favorite','read']]},
 {title:'窗口',sections:[['minimize','maximize'],['palette']]},
 {title:'帮助',sections:[['help','about']]},
]
export function ApplicationMenu() {
 const client=useQueryClient()
 const text=useRef<ReturnType<typeof captureTextTarget> | null>(null)
 const command=(id:CommandID)=><MenubarItem key={id} aria-keyshortcuts={commandShortcut(id,native.isMac())?.keys} onSelect={()=>runCommand(id,client)}><CommandPresentation id={id}/></MenubarItem>
 return <div className="application-menu" onPointerDownCapture={()=>{text.current=captureTextTarget()}} onFocusCapture={event=>{if(event.relatedTarget instanceof HTMLInputElement||event.relatedTarget instanceof HTMLTextAreaElement)text.current=captureTextTarget(event.relatedTarget)}}><Menubar aria-label="应用菜单栏">
 {groups.slice(0,1).map(renderGroup)}
 <MenubarMenu><MenubarTrigger>编辑</MenubarTrigger><MenubarContent className="action-menu">
 <MenubarGroup aria-label="历史">{(['undo','redo'] as const).map(textItem)}</MenubarGroup><MenubarSeparator/>
 <MenubarGroup aria-label="文本编辑">{(['cut','copy','paste','selectAll'] as const).map(textItem)}</MenubarGroup><MenubarSeparator/>
 <MenubarGroup aria-label="搜索">{command('search')}</MenubarGroup>
 </MenubarContent></MenubarMenu>
 {groups.slice(1).map(renderGroup)}
 </Menubar></div>
 function textItem(action:Parameters<typeof editText>[0]) {return <MenubarItem key={action} aria-keyshortcuts={textShortcut(action,native.isMac()).keys} onSelect={()=>{void editText(action,text.current??captureTextTarget())}}><TextActionPresentation action={action}/></MenubarItem>}
 function renderGroup(group:typeof groups[number]) {return <MenubarMenu key={group.title}><MenubarTrigger>{group.title}</MenubarTrigger><MenubarContent className="action-menu">{group.sections.map((section,index)=><Fragment key={index}>{index>0&&<MenubarSeparator/>}<MenubarGroup>{section.map(command)}</MenubarGroup></Fragment>)}</MenubarContent></MenubarMenu>}
}
