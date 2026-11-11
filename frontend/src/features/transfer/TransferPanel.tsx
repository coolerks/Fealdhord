import { Checkbox } from '@/components/ui/checkbox'
import { useCallback,useEffect,useRef,useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { transferAPI } from '@/lib/transfer-api'
import type { PreviewData } from '@/lib/transfer-api'
import { notify } from '@/lib/notifications'
import { invalidateRSS } from '@/lib/refresh'
import { useWorkspace } from '@/stores/workspace'
import { selectOutline } from './selection'
const pageSize=200
const statuses:Record<string,string>={new:'新增',existing:'已存在',duplicate:'重复源',restore:'恢复',invalid:'无法导入'}
export function TransferPanel(){
 const client=useQueryClient(),intent=useWorkspace(state=>state.transferIntent)
 const [preview,setPreview]=useState<PreviewData|null>(null),[selected,setSelected]=useState(new Set<string>()),[page,setPage]=useState(0)
 const [applyExisting,setApplyExisting]=useState(true),[refresh,setRefresh]=useState(true),[busy,setBusy]=useState(''),[error,setError]=useState('')
 const active=useRef<{controller:AbortController;token:string;previewID:string;kind:string}|null>(null),alive=useRef(false),previewID=useRef('')
 useEffect(()=>{alive.current=true;return()=>{alive.current=false;active.current?.controller.abort();void transferAPI.discard(active.current?.token,previewID.current||active.current?.previewID).catch(()=>{});active.current=null}},[])
 const clearPreview=useCallback(()=>{if(previewID.current)void transferAPI.discard('',previewID.current).catch(()=>{});previewID.current='';setPreview(null);setSelected(new Set());setPage(0)},[])
 const choose=useCallback(async(kind:'import'|'export')=>{
  if(active.current)return
  const operation={controller:new AbortController(),token:'',previewID:'',kind}
  active.current=operation;setBusy(kind==='import'?'正在读取预览…':'正在导出…');setError('');clearPreview()
  const current=()=>alive.current&&active.current===operation&&!operation.controller.signal.aborted
  try{
   const grant=await transferAPI.choose(kind);operation.token=grant.token
   if(!current()){void transferAPI.discard(grant.token).catch(()=>{});return}
   if(grant.cancelled)return
   if(kind==='import'){
    const result=await transferAPI.preview(grant.token,operation.controller.signal)
    if(!current()){void transferAPI.discard(grant.token,result.preview_id).catch(()=>{});return}
    operation.previewID=result.preview_id;previewID.current=result.preview_id;setPreview(result);setSelected(new Set(result.items.filter(item=>item.status!=='invalid').map(item=>item.key)))
   }else{
    const result=await transferAPI.export(grant.token,operation.controller.signal)
    if(current()) void notify('OPML 导出完成', `${result.name}（${result.bytes} 字节）`)
   }
  }catch(cause){if(current())setError(cause instanceof Error?cause.message:'文件处理失败，请重新选择。')}
  finally{if(!operation.previewID)void transferAPI.discard(operation.token).catch(()=>{});if(active.current===operation){active.current=null;if(alive.current)setBusy('')}}
 },[clearPreview])
 async function confirm(){
  if(active.current||!preview||selected.size===0)return
  const operation={controller:new AbortController(),token:'',previewID:preview.preview_id,kind:'confirm'}
  active.current=operation;setBusy('正在保存导入…');setError('')
  try{
   const result=await transferAPI.import({preview_id:preview.preview_id,selected:[...selected],apply_existing:applyExisting,refresh},operation.controller.signal)
   invalidateRSS(client)
   if(alive.current&&active.current===operation){const report=result.report;void notify('OPML 导入完成', `已新增 ${report.feeds_added} 个订阅，复用 ${report.feeds_reused} 个，恢复 ${report.feeds_restored} 个；新增 ${report.categories_added} 个分类、${report.associations_added} 个关联。${result.refresh_queued?'已排入刷新队列。':''}${result.refresh_error ?? ''}`, Boolean(result.refresh_error))}
  }catch(cause){invalidateRSS(client);if(alive.current&&active.current===operation)setError(cause instanceof Error?cause.message:'导入失败，请重新预览。')}
  finally{void transferAPI.discard('',operation.previewID).catch(()=>{});if(active.current===operation){active.current=null;if(alive.current){setBusy('');clearPreview()}}}
 }
 function cancel(){const operation=active.current;operation?.controller.abort();active.current=null;void transferAPI.discard(operation?.token,previewID.current||operation?.previewID).catch(()=>{});clearPreview();setBusy('');if(operation?.kind==='confirm'){invalidateRSS(client);void notify('导入请求已取消', '请检查订阅列表后重新预览。')}else if(operation?.kind==='export') void notify('导出请求已取消', '请检查目标文件。')}
 useEffect(()=>{
  let current=true
  queueMicrotask(()=>{if(!current||!intent||useWorkspace.getState().transferIntent!==intent)return;useWorkspace.setState({transferIntent:null});void choose(intent)})
  return()=>{current=false}
 },[intent,choose])
 const total=preview?.items.length??0, pages=Math.max(1,Math.ceil(total/pageSize))
 return <div className="transfer-panel">
  <div className="feed-actions"><Button variant="outline" disabled={!!busy} onClick={()=>{void choose('import')}}><AnimatedGlyph meaning="folder"/>导入 OPML</Button><Button variant="outline" disabled={!!busy} onClick={()=>{void choose('export')}}><AnimatedGlyph meaning="folder"/>导出 OPML</Button>{(busy||preview)&&<Button variant="ghost" onClick={cancel}>取消</Button>}</div>
  {busy&&<p role="status">{busy}</p>}{error&&<p role="alert">{error}</p>}
  {preview&&<section aria-label="OPML 导入预览"><h3>{preview.name}</h3><div className="feed-actions"><Button size="sm" variant="ghost" disabled={!!busy} onClick={()=>setSelected(new Set(preview.items.filter(item=>item.status!=='invalid').map(item=>item.key)))}>全选</Button><Button size="sm" variant="ghost" disabled={!!busy} onClick={()=>setSelected(new Set())}>全不选</Button><span>已选择 {selected.size} 项</span></div>
   <ul className="transfer-items">{preview.items.slice(page*pageSize,(page+1)*pageSize).map(item=><li key={item.key} style={{paddingInlineStart:`${Math.min(item.depth,5)*12}px`}}><label data-animated-trigger aria-disabled={!!busy||item.status==='invalid'}><Checkbox disabled={!!busy||item.status==='invalid'} checked={selected.has(item.key)} onCheckedChange={value=>setSelected(previous=>selectOutline(preview.items,previous,item.key,value===true))}/><AnimatedGlyph meaning={item.kind==='folder'?'folder':'radio-tower'}/><span>{item.title||'无标题'}</span><small>{statuses[item.status]??item.status}</small></label>{item.reason&&<p>{item.reason}</p>}</li>)}</ul>
   {pages>1&&<div className="feed-actions"><Button size="sm" variant="outline" disabled={page===0} onClick={()=>setPage(value=>value-1)}>上一页</Button><span>{page*pageSize+1}–{Math.min(total,(page+1)*pageSize)} / {total}</span><Button size="sm" variant="outline" disabled={page+1>=pages} onClick={()=>setPage(value=>value+1)}>下一页</Button></div>}
   <label className="transfer-option"><Checkbox disabled={!!busy} checked={applyExisting} onCheckedChange={value=>setApplyExisting(value===true)}/>分类关联应用于已有文章</label><label className="transfer-option"><Checkbox disabled={!!busy} checked={refresh} onCheckedChange={value=>setRefresh(value===true)}/>导入后刷新订阅</label>
   <Button disabled={!!busy||selected.size===0} onClick={()=>{void confirm()}}>确认导入</Button>
  </section>}
 </div>
}
