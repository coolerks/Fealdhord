import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Field,FieldGroup,FieldLabel } from '@/components/ui/field'
import { Dialog,DialogContent,DialogHeader,DialogTitle,DialogDescription } from '@/components/ui/dialog'
import { ToggleGroup,ToggleGroupItem } from '@/components/ui/toggle-group'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { navigationAPI,navigationQueryKey } from '@/lib/navigation-api'
import type { CategoryInput,CategoryDeletion } from '@/lib/navigation-api'
import { articleQueryKey } from '@/lib/library'
import { feedsQueryKey } from '@/lib/rss-api'
import { useWorkspace } from '@/stores/workspace'
import { useNavigation } from './use-navigation'
import { categoryPath } from './category-path'
import { categoryIcons } from './category-icons'
const empty:CategoryInput={id:'',revision:0,name:'',parent_id:'',icon:'folder',color:'',description:'',sort_position:0}
export function CategoryManager(){
 const query=useNavigation(),client=useQueryClient()
 const [draft,setDraft]=useState<CategoryInput|null>(null),[deletion,setDeletion]=useState<CategoryDeletion|null>(null)
 const action=useMutation({mutationFn:async(run:()=>Promise<unknown>)=>run(),onSuccess:()=>{void client.invalidateQueries({queryKey:navigationQueryKey});void client.invalidateQueries({queryKey:articleQueryKey});void client.invalidateQueries({queryKey:feedsQueryKey})}})
 const busy=action.isPending
 return <div className="category-manager"><Button disabled={busy} onClick={()=>{action.reset();setDraft({...empty})}}><AnimatedGlyph meaning="plus"/>新建分类</Button>
  {query.isPending && <p role="status">正在读取分类…</p>}{query.isError && <div role="alert">分类读取失败<Button variant="ghost" onClick={()=>{void query.refetch()}}>重试</Button></div>}
  {action.error && !draft && !deletion && <p role="alert">{action.error.message}</p>}
  <ul className="feed-list">{query.data?.categories?.map(category=><li key={category.id}><div className="feed-details"><strong>{categoryPath(category,query.data?.categories ?? [])}</strong></div><Button size="sm" variant="ghost" disabled={busy} aria-label={`编辑分类 ${categoryPath(category,query.data?.categories ?? [])}`} onClick={()=>{action.reset();setDraft({...category})}}>编辑</Button><Button size="sm" variant="ghost" disabled={busy} aria-label={`删除分类 ${categoryPath(category,query.data?.categories ?? [])}`} onClick={()=>{action.reset();action.mutate(async()=>setDeletion(await navigationAPI.previewDelete(category.id)))}}>删除</Button></li>)}</ul>
  {query.isSuccess && !query.data.categories?.length && <p>暂无分类</p>}
  <Dialog open={Boolean(draft)} onOpenChange={open=>{if(!open&&!busy)setDraft(null)}}><DialogContent aria-describedby={undefined}><DialogHeader><DialogTitle>{draft?.id?'编辑分类':'新建分类'}</DialogTitle></DialogHeader>{draft && <form className="feed-form" onSubmit={event=>{event.preventDefault();action.mutate(async()=>{await navigationAPI.save(draft);setDraft(null)})}}><FieldGroup>
   <Field><FieldLabel htmlFor="category-name">名称</FieldLabel><Input id="category-name" required maxLength={256} disabled={busy} value={draft.name} onChange={event=>setDraft({...draft,name:event.target.value})}/></Field>
   <Field><FieldLabel htmlFor="category-parent">上级分类</FieldLabel><Select disabled={busy} value={draft.parent_id || 'root-category'} onValueChange={value => setDraft({...draft,parent_id:value === 'root-category' ? '' : value})}>
     <SelectTrigger id="category-parent" className="w-full"><SelectValue /></SelectTrigger>
     <SelectContent position="popper"><SelectGroup>
       <SelectItem value="root-category">无上级</SelectItem>
       {query.data?.categories?.filter(item=>item.id!==draft.id).map(item=><SelectItem key={item.id} value={item.id}>{categoryPath(item,query.data?.categories ?? [])}</SelectItem>)}
     </SelectGroup></SelectContent>
   </Select></Field>
   <Field><FieldLabel id="category-icon-label">图标</FieldLabel><ToggleGroup type="single" variant="outline" disabled={busy} value={draft.icon||'folder'} aria-labelledby="category-icon-label" onValueChange={value=>{if(value)setDraft({...draft,icon:value})}}>{categoryIcons.map(icon=><ToggleGroupItem key={icon} value={icon} aria-label={{folder:'文件夹',book:'图书','radio-tower':'订阅',favorite:'星标',ai:'星光'}[icon]}><AnimatedGlyph meaning={icon}/></ToggleGroupItem>)}</ToggleGroup></Field>
   <Field><FieldLabel htmlFor="category-color">颜色</FieldLabel><div className="feed-actions"><Input id="category-color" type="color" disabled={busy} value={draft.color||'#808080'} onChange={event=>setDraft({...draft,color:event.target.value})}/><Button type="button" disabled={busy} variant="ghost" onClick={()=>setDraft({...draft,color:''})}>使用默认颜色</Button></div></Field>
   <Field><FieldLabel htmlFor="category-description">描述</FieldLabel><Input id="category-description" disabled={busy} maxLength={2048} value={draft.description} onChange={event=>setDraft({...draft,description:event.target.value})}/></Field>
   <Field><FieldLabel htmlFor="category-order">排序位置</FieldLabel><Input id="category-order" type="number" min={-1000000} max={1000000} disabled={busy} value={draft.sort_position} onChange={event=>{if(Number.isFinite(event.target.valueAsNumber))setDraft({...draft,sort_position:event.target.valueAsNumber})}}/></Field>
   {action.error && <p role="alert">{action.error.message}</p>}<div className="feed-actions"><Button type="submit" disabled={busy}>保存分类</Button><Button variant="ghost" type="button" disabled={busy} onClick={()=>setDraft(null)}>取消</Button></div>
  </FieldGroup></form>}</DialogContent></Dialog>
  <Dialog open={Boolean(deletion)} onOpenChange={open=>{if(!open&&!busy)setDeletion(null)}}><DialogContent><DialogHeader><DialogTitle>删除“{deletion?.name}”？</DialogTitle><DialogDescription>将 {deletion?.children} 个子分类移到上一级，移除 {deletion?.articles} 篇文章和 {deletion?.feeds} 个订阅的分类关联。文章和订阅保留。</DialogDescription></DialogHeader>{action.error && <p role="alert">{action.error.message}</p>}<Button disabled={busy} onClick={()=>{if(deletion)action.mutate(async()=>{await navigationAPI.remove(deletion);const workspace=useWorkspace.getState();if(workspace.preferences?.selected_scope==='category'&&workspace.preferences.selected_scope_id===deletion.id)workspace.patch({selected_scope:'all',selected_scope_id:''});setDeletion(null)})}}>确认删除分类</Button><Button variant="ghost" disabled={busy} onClick={()=>setDeletion(null)}>取消</Button></DialogContent></Dialog>
 </div>
}
