import { native } from './native'
import { useWorkspace } from '@/stores/workspace'
export type TextTarget = { field: HTMLInputElement | HTMLTextAreaElement | null; start: number; end: number; text: string; value: string }
export function captureTextTarget(element: EventTarget | null = document.activeElement): TextTarget {
 const input=element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement ? element:null
 const field=input && input.type!=='password' && input.selectionStart!==null ? input:null
 const start=field?.selectionStart??0,end=field?.selectionEnd??0
 return {field,start,end,value:field?.value??'',text:field?field.value.slice(start,end):input?'':window.getSelection()?.toString()??''}
}
export async function editText(action:'copy'|'cut'|'paste'|'selectAll'|'undo'|'redo',target:TextTarget) {
 try {
  const {field,start,end,text}=target
  if(action==='copy'||action==='cut') {
   if(!text)return
   await native.copy(text)
   if(action==='copy')return
  }
  if(action==='cut'||action==='paste') {
   if(!field?.isConnected||field.readOnly||field.disabled)return
   const value=action==='paste'?await native.paste():''
   // 异步剪贴板读取不能把修改写到已经变化的输入快照。
   if(field.value!==target.value)return
   field.focus({preventScroll:true});field.setSelectionRange(start,end)
   const replacement=field.maxLength>=0?value.slice(0,Math.max(0,field.maxLength-(field.value.length-(end-start)))):value
   field.setRangeText(replacement,start,end,'end')
   field.dispatchEvent(new Event('input',{bubbles:true}))
  } else if(action==='selectAll') {
   if(field){field.focus();field.select()}
   else {
     const article = document.querySelector('.reader-content .reader-prose')
     if (!article || article.querySelector('[role=alert]')) return
     const selection = window.getSelection(), range = document.createRange()
     range.selectNodeContents(article)
     selection?.removeAllRanges()
     selection?.addRange(range)
   }
  } else {field?.focus();document.execCommand(action)}
 } catch {useWorkspace.setState({commandError:'文本操作未完成，请重试或使用系统编辑菜单。'})}
}
