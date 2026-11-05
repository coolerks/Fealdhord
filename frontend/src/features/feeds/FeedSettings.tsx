import { useRef,useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Switch } from '@/components/ui/switch'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuGroup, DropdownMenuCheckboxItem } from '@/components/ui/dropdown-menu'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { rssAPI, feedsQueryKey } from '@/lib/rss-api'
import type { Candidate, Feed, UpdateRequest } from '@/lib/rss-api'
import { invalidateRSS, startRefresh } from '@/lib/refresh'
import { useWorkspace } from '@/stores/workspace'
import { useNavigation } from '@/features/categories/use-navigation'
import { useRefresh } from '@/stores/refresh'
import type { CategorySummary } from '../../../bindings/github.com/coolerks/Fealdhord/internal/database/models'

function Categories({items, value, onChange, disabled}:{items:CategorySummary[];value:string[];onChange:(ids:string[])=>void;disabled:boolean}) {
  if(!items.length) return null
  return <DropdownMenu><DropdownMenuTrigger asChild><Button type="button" variant="outline" disabled={disabled}>默认分类{value.length ? ` · ${value.length}` : ''}</Button></DropdownMenuTrigger><DropdownMenuContent><DropdownMenuGroup>{items.map(item=><DropdownMenuCheckboxItem key={item.id} checked={value.includes(item.id)} onSelect={event=>event.preventDefault()} onCheckedChange={checked=>onChange(checked ? [...value.filter(id=>id!==item.id),item.id] : value.filter(id=>id!==item.id))}>{item.name}</DropdownMenuCheckboxItem>)}</DropdownMenuGroup></DropdownMenuContent></DropdownMenu>
}
export function FeedSettings() {
  const client=useQueryClient()
  const navigation=useNavigation()
  const editController=useRef<AbortController|null>(null)
  const [initialCategories,setInitialCategories]=useState<string[]>([])
  const query=useQuery({queryKey:feedsQueryKey,queryFn:rssAPI.list,retry:false})
  const [url,setURL]=useState(''), [title,setTitle]=useState(''), [ids,setIDs]=useState<string[]>([])
  const [candidates,setCandidates]=useState<Candidate[]>([]), [selected,setSelected]=useState('')
  const [archived,setArchived]=useState(false), [removing,setRemoving]=useState<Feed|null>(null)
  const [editing,setEditing]=useState<UpdateRequest|null>(null)
  const [cacheMessage,setCacheMessage]=useState('')
  const prefs=useWorkspace(state=>state.preferences)!, patch=useWorkspace(state=>state.patch)
  const refreshPending=useRefresh(state=>state.pending)
  const action=useMutation({mutationFn:async(run:()=>Promise<unknown>)=>run(),onSuccess:()=>invalidateRSS(client)})
  const discovery=useMutation({mutationFn:rssAPI.discover,onSuccess:items=>{setCandidates(items);setSelected(items.length===1?items[0].url:'');setTitle(items.length===1?items[0].title:'')}})
  const busy=action.isPending || discovery.isPending
  const categories=query.data?.categories ?? []
  function discover() { setCandidates([]);setSelected('');action.reset();discovery.mutate(url.trim()) }
  return <div className="feed-settings"><FieldGroup>
    <form className="feed-form" onSubmit={event=>{event.preventDefault();discover()}}>
      <Field><FieldLabel htmlFor="feed-url">订阅或网页地址</FieldLabel><Input id="feed-url" type="url" required maxLength={4096} disabled={busy} value={url} onChange={event=>{setURL(event.target.value);setCandidates([]);setSelected('');discovery.reset()}} /></Field>
      <Button type="submit" disabled={busy || !url.trim()}><AnimatedGlyph meaning="search"/>{discovery.isPending?'正在查找…':'查找订阅'}</Button>
    </form>
    {discovery.error && <p role="alert">{discovery.error.message}</p>}
    {discovery.isSuccess && !candidates.length && <p role="status">没有找到订阅</p>}
    {candidates.length>0 && <form className="feed-form" onSubmit={event=>{event.preventDefault();if(selected)action.mutate(async()=>{await rssAPI.add({url:selected,title,category_ids:ids});setURL('');setCandidates([]);setSelected('');setTitle('');setIDs([])})}}>
      <Field><FieldLabel id="feed-candidates-label">选择订阅</FieldLabel><ToggleGroup type="single" variant="outline" aria-labelledby="feed-candidates-label" value={selected} disabled={busy} onValueChange={value=>{setSelected(value);setTitle(candidates.find(item=>item.url===value)?.title ?? '')}}>{candidates.map(item=><ToggleGroupItem key={item.url} value={item.url} className="feed-candidate"><span>{item.title || item.type}</span><small>{item.url}</small></ToggleGroupItem>)}</ToggleGroup></Field>
      <Field><FieldLabel htmlFor="feed-title">订阅名称</FieldLabel><Input id="feed-title" value={title} maxLength={512} disabled={busy} onChange={event=>setTitle(event.target.value)}/></Field>
      <Categories items={categories} value={ids} onChange={setIDs} disabled={busy}/><Button type="submit" disabled={busy || !selected}><AnimatedGlyph meaning="plus"/>添加订阅</Button>
    </form>}
    {action.error && <p role="alert">{action.error.message}</p>}
    <Field orientation="horizontal"><FieldLabel htmlFor="archived-feeds">显示已归档订阅</FieldLabel><Switch id="archived-feeds" checked={archived} onCheckedChange={setArchived}/></Field>
    {query.isPending && <p role="status">正在载入订阅…</p>}
    {query.isError && <div role="alert">订阅载入失败<Button variant="ghost" onClick={()=>{void query.refetch()}}>重试</Button></div>}
    {query.isSuccess && !(query.data.items ?? []).some(feed=>archived || !feed.archived) && <p>暂无订阅</p>}
    <ul className="feed-list">{(query.data?.items ?? []).filter(feed=>archived || !feed.archived).map(feed=><li key={feed.id}>
      <div className="feed-details"><strong>{feed.title}</strong>{feed.archived && <span>已归档</span>}<small>{feed.url}</small>{feed.last_error && <p>{feed.last_error}{feed.next_check_at && ` · 下次尝试 ${new Date(feed.next_check_at).toLocaleString('zh-CN')}`}</p>}</div>
      <div className="feed-actions">{feed.archived ? <Button size="sm" variant="outline" disabled={busy} onClick={()=>action.mutate(()=>rssAPI.restore({id:feed.id,revision:feed.revision}))}>恢复订阅</Button> : <>
        <Button size="sm" variant="ghost" disabled={busy || refreshPending} aria-label={`刷新 ${feed.title}`} onClick={()=>{void startRefresh(client,[feed.id])}}><AnimatedGlyph meaning="refresh"/>刷新</Button>
        <Button size="sm" variant="ghost" disabled={busy} aria-label={`编辑 ${feed.title}`} onClick={()=>{action.reset();setInitialCategories(feed.category_ids ?? []);setEditing({apply_existing:true,id:feed.id,revision:feed.revision,url:feed.url,title:feed.title,category_ids:feed.category_ids ?? []})}}>编辑</Button>
        <Button size="sm" variant="ghost" disabled={busy} aria-label={`归档 ${feed.title}`} onClick={()=>{action.reset();setRemoving(feed)}}>归档</Button>
      </>}</div>
    </li>)}</ul>
    <Field><FieldLabel id="remote-images-label">远程图片</FieldLabel><ToggleGroup type="single" variant="outline" aria-labelledby="remote-images-label" value={prefs.remote_images} onValueChange={value=>{if(value)patch({remote_images:value})}}>{[['always','总是允许'],['subscriptions','仅活动订阅'],['never','从不加载']].map(([value,label])=><ToggleGroupItem key={value} value={value}>{label}</ToggleGroupItem>)}</ToggleGroup></Field>
    <Field orientation="horizontal"><FieldLabel htmlFor="web-image-fallback">从原文页面补充首图</FieldLabel><Switch id="web-image-fallback" checked={prefs.web_image_fallback} onCheckedChange={value=>patch({web_image_fallback:value})}/></Field>
    <Field orientation="horizontal"><FieldLabel htmlFor="media-cache">磁盘图片缓存</FieldLabel><Switch id="media-cache" checked={prefs.media_cache_enabled} onCheckedChange={value=>patch({media_cache_enabled:value})}/></Field>
    <Button variant="outline" disabled={busy} onClick={()=>{setCacheMessage('');action.mutate(async()=>{await rssAPI.clearCache();setCacheMessage('图片缓存已清理')})}}>清理图片缓存</Button>{cacheMessage && <p role="status">{cacheMessage}</p>}
  </FieldGroup>
  <Dialog open={Boolean(editing)} onOpenChange={open=>{if(!open && !action.isPending)setEditing(null)}}><DialogContent aria-describedby={undefined}><DialogHeader><DialogTitle>编辑订阅</DialogTitle></DialogHeader>{editing && <form className="feed-form" onSubmit={event=>{event.preventDefault();action.mutate(async()=>{editController.current=new AbortController();try{await rssAPI.update(editing,editController.current.signal);setEditing(null)}catch(error){invalidateRSS(client);throw error}finally{editController.current=null}})}}>
    <Field><FieldLabel htmlFor="edit-feed-title">名称</FieldLabel><Input id="edit-feed-title" required maxLength={512} value={editing.title} disabled={busy} onChange={event=>setEditing({...editing,title:event.target.value})}/></Field>
    <Field><FieldLabel htmlFor="edit-feed-url">订阅地址</FieldLabel><Input id="edit-feed-url" type="url" required maxLength={4096} value={editing.url} disabled={busy} onChange={event=>setEditing({...editing,url:event.target.value})}/></Field>
    <Categories items={categories} value={editing.category_ids ?? []} disabled={busy} onChange={category_ids=>setEditing({...editing,category_ids})}/>
    {JSON.stringify([...(editing.category_ids ?? [])].sort())!==JSON.stringify([...initialCategories].sort()) && <Field orientation="horizontal"><FieldLabel htmlFor="apply-existing-categories">应用到现有 {navigation.data?.feeds?.find(feed=>feed.id===editing.id)?.total ?? '…'} 篇文章</FieldLabel><Switch id="apply-existing-categories" checked={editing.apply_existing!==false} disabled={busy} onCheckedChange={value=>setEditing({...editing,apply_existing:value})}/></Field>}
    {action.isPending && editController.current && <Button type="button" variant="ghost" onClick={()=>editController.current?.abort()}>取消保存</Button>}
    {action.error && <p role="alert">{action.error.message}</p>}<Button disabled={busy} type="submit">保存订阅</Button><Button disabled={busy} type="button" variant="ghost" onClick={()=>setEditing(null)}>取消</Button>
  </form>}</DialogContent></Dialog>
  <Dialog open={Boolean(removing)} onOpenChange={open=>{if(!open && !action.isPending)setRemoving(null)}}><DialogContent><DialogHeader><DialogTitle>归档“{removing?.title}”？</DialogTitle><DialogDescription>停止刷新此订阅，保留文章、已读和收藏。之后可以恢复。</DialogDescription></DialogHeader>{action.error && <p role="alert">{action.error.message}</p>}<Button disabled={busy} onClick={()=>{if(removing)action.mutate(async()=>{await rssAPI.archive({id:removing.id,revision:removing.revision});setRemoving(null)})}}>确认归档</Button><Button variant="ghost" disabled={busy} onClick={()=>setRemoving(null)}>取消</Button></DialogContent></Dialog>
  </div>
}
