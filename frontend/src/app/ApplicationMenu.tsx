import { Fragment, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { useQueryClient } from '@tanstack/react-query'
import { Menubar, MenubarMenu, MenubarTrigger, MenubarContent, MenubarGroup, MenubarItem, MenubarSeparator } from '@/components/ui/menubar'
import { runCommand } from '@/lib/commands'
import type { CommandID } from '@/lib/commands'
import { captureTextTarget, editText } from '@/lib/text-actions'
import { CommandPresentation, TextActionPresentation } from '@/components/command-presentation'
import { commandShortcut, textShortcut } from '@/lib/command-presentation'
import { useWorkspace } from '@/stores/workspace'
import { native } from '@/lib/native'

const groups: {title:string;sections:CommandID[][]}[] = [
 {title:'文件',sections:[['feeds','categories','refresh'],['import-opml','export-opml'],['settings','close','quit']]},
 {title:'显示',sections:[['all','unread','favorites'],['list','masonry','sidebar','reader'],['theme-system','theme-light','theme-dark'],['fullscreen']]},
 {title:'文章',sections:[['previous','next'],['reader-mode','open-original'],['favorite','read']]},
 {title:'窗口',sections:[['minimize','maximize'],['palette']]},
]
export function ApplicationMenu({ windowsSidebarToggle }: { windowsSidebarToggle?: ReactNode }) {
 const client=useQueryClient()
 const developerMode = useWorkspace(state => state.preferences?.developer_mode)
 const menuClassName = windowsSidebarToggle ? "action-menu windows-menu-popover" : "action-menu"
 const text=useRef<ReturnType<typeof captureTextTarget> | null>(null)
 const command=(id:CommandID)=><MenubarItem key={id} aria-keyshortcuts={commandShortcut(id,native.isMac())?.keys} onSelect={()=>runCommand(id,client)}><CommandPresentation id={id}/></MenubarItem>
 return <div className="application-menu" onPointerDownCapture={()=>{text.current=captureTextTarget()}} onFocusCapture={event=>{if(event.relatedTarget instanceof HTMLInputElement||event.relatedTarget instanceof HTMLTextAreaElement)text.current=captureTextTarget(event.relatedTarget)}}>
 {windowsSidebarToggle}
 <Menubar aria-label="应用菜单栏">
 {groups.slice(0,1).map(renderGroup)}
 <MenubarMenu><MenubarTrigger>编辑</MenubarTrigger><MenubarContent className={menuClassName}>
 <MenubarGroup aria-label="历史">{(['undo','redo'] as const).map(textItem)}</MenubarGroup><MenubarSeparator/>
 <MenubarGroup aria-label="文本编辑">{(['cut','copy','paste','selectAll'] as const).map(textItem)}</MenubarGroup><MenubarSeparator/>
 <MenubarGroup aria-label="搜索">{command('search')}</MenubarGroup>
 </MenubarContent></MenubarMenu>
 {groups.slice(1).map(renderGroup)}
 {renderGroup({ title: '帮助', sections: developerMode ? [['help', 'about', 'diagnostics'], ['devtools']] : [['help', 'about', 'diagnostics']] })}
 </Menubar>
 {windowsSidebarToggle && <>
   <div className="window-drag-area" onDoubleClick={() => runCommand('maximize',client)} />
   <WindowControls />
 </>}
 </div>
 function textItem(action:Parameters<typeof editText>[0]) {return <MenubarItem key={action} aria-keyshortcuts={textShortcut(action,native.isMac()).keys} onSelect={()=>{void editText(action,text.current??captureTextTarget())}}><TextActionPresentation action={action}/></MenubarItem>}
 function renderGroup(group:typeof groups[number]) {return <MenubarMenu key={group.title}><MenubarTrigger>{group.title}</MenubarTrigger><MenubarContent className={menuClassName}>{group.sections.map((section,index)=><Fragment key={index}>{index>0&&<MenubarSeparator/>}<MenubarGroup>{section.map(command)}</MenubarGroup></Fragment>)}</MenubarContent></MenubarMenu>}
}


function WindowControls() {
 const client = useQueryClient()
 const [maximized, setMaximized] = useState(false)
 useEffect(() => {
   let current = true
   const unsubscribe = native.onMaximizeChange(value => { current = false; setMaximized(value) })
   void native.isMaximized().then(value => { if (current) setMaximized(value) }).catch(() => {})
   return () => { current = false; unsubscribe() }
 }, [])
 return <div className="window-controls" role="group" aria-label="窗口控制">
   <Button variant="ghost" size="icon" aria-label="最小化窗口" onClick={() => runCommand('minimize', client)}>
     <svg aria-hidden="true" viewBox="0 0 12 12"><path d="M1 6h10" /></svg>
   </Button>
   <Button variant="ghost" size="icon" aria-label={maximized ? '还原窗口' : '最大化窗口'} onClick={() => runCommand('maximize', client)}>
     <svg aria-hidden="true" viewBox="0 0 12 12">
       {maximized ? <path d="M4 3.5v-2h6.5V8h-2M1.5 4h6.5v6.5H1.5z" /> : <path d="M1.5 1.5h9v9h-9z" />}
     </svg>
   </Button>
   <Button variant="ghost" size="icon" className="window-close" aria-label="关闭窗口" onClick={() => runCommand('close', client)}>
     <svg aria-hidden="true" viewBox="0 0 12 12"><path d="m1.5 1.5 9 9m0-9-9 9" /></svg>
   </Button>
 </div>
}
