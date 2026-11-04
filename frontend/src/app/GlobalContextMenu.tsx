import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { ContextMenu, ContextMenuTrigger, ContextMenuContent, ContextMenuGroup, ContextMenuItem, ContextMenuSeparator, ContextMenuLabel } from '@/components/ui/context-menu'
import { captureTextTarget, editText } from '@/lib/text-actions'
import { commandLabels, runCommand } from '@/lib/commands'
import type { CommandID } from '@/lib/commands'

export function GlobalContextMenu({children}:{children:ReactNode}) {
 const client=useQueryClient()
 const text=useRef<ReturnType<typeof captureTextTarget>|null>(null)
 const [target,setTarget]=useState({articleID:'',editable:false,selection:false,readOnly:false})
 useEffect(()=>{const block=(event:MouseEvent)=>event.preventDefault();document.addEventListener('contextmenu',block);return()=>document.removeEventListener('contextmenu',block)},[])
 return <ContextMenu><ContextMenuTrigger asChild><div className="application-root" onContextMenuCapture={event=>{
  text.current=captureTextTarget(event.target)
  const element=event.target instanceof Element?event.target:null
  setTarget({articleID:element?.closest<HTMLElement>('[data-article-id]')?.dataset.articleId??'',editable:Boolean(text.current.field),selection:Boolean(text.current.text),readOnly:Boolean(text.current.field?.readOnly||text.current.field?.disabled)})
 }}>{children}</div></ContextMenuTrigger><ContextMenuContent aria-label="阅读上下文菜单" onCloseAutoFocus={event=>{if(text.current?.field){event.preventDefault();text.current.field.focus({preventScroll:true})}}}>
 <ContextMenuLabel>{target.editable?'编辑文本':target.articleID?'文章操作':'阅读空间'}</ContextMenuLabel>
 <ContextMenuGroup>{(target.editable||target.selection)&&<>
 <ContextMenuItem disabled={!target.selection} onSelect={()=>{if(text.current)void editText('copy',text.current)}}>复制</ContextMenuItem>
 {target.editable&&<><ContextMenuItem disabled={!target.selection||target.readOnly} onSelect={()=>{if(text.current)void editText('cut',text.current)}}>剪切</ContextMenuItem><ContextMenuItem disabled={target.readOnly} onSelect={()=>{if(text.current)void editText('paste',text.current)}}>粘贴</ContextMenuItem><ContextMenuItem onSelect={()=>{if(text.current)void editText('selectAll',text.current)}}>全选</ContextMenuItem></>}
 <ContextMenuSeparator/></>}
 {target.articleID&&(['favorite','read'] as const).map(id=><ContextMenuItem key={id} onSelect={()=>runCommand(id,client,target.articleID)}>{commandLabels[id]}</ContextMenuItem>)}
 {(['search','list','masonry','sidebar','settings'] as CommandID[]).map(id=><ContextMenuItem key={id} onSelect={()=>runCommand(id,client)}>{commandLabels[id]}</ContextMenuItem>)}
 </ContextMenuGroup></ContextMenuContent></ContextMenu>
}
