import { useEffect, useState } from 'react'
import { Copy } from 'lucide-react'
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
import { FieldSelection } from './FieldSelection'
import { copyReaderImage, readerImageID } from '@/lib/reader-image'

export function GlobalContextMenu({children}:{children:ReactNode}) {
 const client=useQueryClient()
 const [open, setOpen] = useState(false)
 const selectedID=useWorkspace(state=>state.preferences?.selected_article_id)
 const [target,setTarget]=useState<{articleID: string; imageID?: string; text: ReturnType<typeof captureTextTarget> | null}>({articleID:'',text:null})
 const imageID = target.imageID
 const field = target.text?.field
 const hasSelection = Boolean(target.text?.text)
 const readOnly = Boolean(field?.readOnly || field?.disabled)
 const targetMatches=target.articleID===selectedID
 useEffect(()=>{const block=(event:MouseEvent)=>event.preventDefault();document.addEventListener('contextmenu',block);return()=>document.removeEventListener('contextmenu',block)},[])
 const textItem=(action:TextAction,disabled=false)=><ContextMenuItem key={action} disabled={disabled} aria-keyshortcuts={textShortcut(action,native.isMac()).keys} onSelect={()=>{if(target.text)void editText(action,target.text)}}><TextActionPresentation action={action}/></ContextMenuItem>
 const commands=(ids:CommandID[],article=false)=>ids.map(id=><ContextMenuItem key={id} aria-keyshortcuts={commandShortcut(id,native.isMac(),!article||targetMatches)?.keys} onSelect={()=>runCommand(id,client,article?target.articleID:undefined)}><CommandPresentation id={id} articleTargetMatches={!article||targetMatches}/></ContextMenuItem>)
 return <ContextMenu onOpenChange={setOpen}><ContextMenuTrigger asChild><div className="application-root" onContextMenuCapture={event=>{
  const element=event.target instanceof Element?event.target:null
  if (element?.closest('[data-reader-image-viewer]')) return
  const imageID = element instanceof HTMLImageElement && !element.hidden && element.closest('.reader-prose[data-images=allowed]')
    ? readerImageID(element) : undefined
  setTarget({articleID:element?.closest<HTMLElement>('[data-article-id]')?.dataset.articleId??'',imageID,text:captureTextTarget(event.target)})
 }}>{children}</div></ContextMenuTrigger>{open && target.text && <FieldSelection target={target.text} />}<ContextMenuContent className="action-menu global-context-menu" aria-label="阅读上下文菜单" onCloseAutoFocus={event=>{if(field?.isConnected){event.preventDefault();field.focus({preventScroll:true})}}}>
 {imageID && <><ContextMenuGroup aria-label="图片"><ContextMenuItem onSelect={() => { void copyReaderImage(imageID) }}><Copy />复制图片</ContextMenuItem></ContextMenuGroup><ContextMenuSeparator /></>}
 {(field||hasSelection)&&<><ContextMenuGroup aria-label="文本编辑">
 {field&&textItem('cut',!hasSelection||readOnly)}
 {textItem('copy',!hasSelection)}
 {field&&<>{textItem('paste',readOnly)}{textItem('selectAll')}</>}
 </ContextMenuGroup><ContextMenuSeparator/></>}
 {target.articleID&&<><ContextMenuGroup aria-label="文章操作">{commands(['favorite','read'],true)}</ContextMenuGroup><ContextMenuSeparator/></>}
 <ContextMenuGroup aria-label="视图">{commands(['search','list','masonry','sidebar'])}</ContextMenuGroup>
 <ContextMenuSeparator/><ContextMenuGroup aria-label="应用">{commands(['settings'])}</ContextMenuGroup>
 </ContextMenuContent></ContextMenu>
}
