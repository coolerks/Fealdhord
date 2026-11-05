import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { ContextMenu, ContextMenuTrigger, ContextMenuContent, ContextMenuGroup, ContextMenuItem, ContextMenuSeparator } from '@/components/ui/context-menu'
import { captureTextTarget, editText } from '@/lib/text-actions'
import { runCommand } from '@/lib/commands'
import type { CommandID } from '@/lib/commands'
import { CommandPresentation, TextActionPresentation } from '@/components/command-presentation'
import { commandShortcut, textShortcut } from '@/lib/command-presentation'
import type { TextAction } from '@/lib/command-presentation'
import { native } from '@/lib/native'
import { useWorkspace } from '@/stores/workspace'

export function GlobalContextMenu({children}:{children:ReactNode}) {
 const client=useQueryClient()
 const text=useRef<ReturnType<typeof captureTextTarget>|null>(null)
 const selectedID=useWorkspace(state=>state.preferences?.selected_article_id)
 const [target,setTarget]=useState({articleID:'',editable:false,selection:false,readOnly:false})
 const targetMatches=target.articleID===selectedID
 useEffect(()=>{const block=(event:MouseEvent)=>event.preventDefault();document.addEventListener('contextmenu',block);return()=>document.removeEventListener('contextmenu',block)},[])
 const textItem=(action:TextAction,disabled=false)=><ContextMenuItem key={action} disabled={disabled} aria-keyshortcuts={textShortcut(action,native.isMac()).keys} onSelect={()=>{if(text.current)void editText(action,text.current)}}><TextActionPresentation action={action}/></ContextMenuItem>
 const commands=(ids:CommandID[],article=false)=>ids.map(id=><ContextMenuItem key={id} aria-keyshortcuts={commandShortcut(id,native.isMac(),!article||targetMatches)?.keys} onSelect={()=>runCommand(id,client,article?target.articleID:undefined)}><CommandPresentation id={id} articleTargetMatches={!article||targetMatches}/></ContextMenuItem>)
 return <ContextMenu><ContextMenuTrigger asChild><div className="application-root" onContextMenuCapture={event=>{
  text.current=captureTextTarget(event.target)
  const element=event.target instanceof Element?event.target:null
  setTarget({articleID:element?.closest<HTMLElement>('[data-article-id]')?.dataset.articleId??'',editable:Boolean(text.current.field),selection:Boolean(text.current.text),readOnly:Boolean(text.current.field?.readOnly||text.current.field?.disabled)})
 }}>{children}</div></ContextMenuTrigger><ContextMenuContent className="action-menu" aria-label="阅读上下文菜单" onCloseAutoFocus={event=>{if(text.current?.field){event.preventDefault();text.current.field.focus({preventScroll:true})}}}>
 {(target.editable||target.selection)&&<><ContextMenuGroup aria-label="文本编辑">
 {target.editable&&textItem('cut',!target.selection||target.readOnly)}
 {textItem('copy',!target.selection)}
 {target.editable&&<>{textItem('paste',target.readOnly)}{textItem('selectAll')}</>}
 </ContextMenuGroup><ContextMenuSeparator/></>}
 {target.articleID&&<><ContextMenuGroup aria-label="文章操作">{commands(['favorite','read'],true)}</ContextMenuGroup><ContextMenuSeparator/></>}
 <ContextMenuGroup aria-label="视图">{commands(['search','list','masonry','sidebar'])}</ContextMenuGroup>
 <ContextMenuSeparator/><ContextMenuGroup aria-label="应用">{commands(['settings'])}</ContextMenuGroup>
 </ContextMenuContent></ContextMenu>
}
