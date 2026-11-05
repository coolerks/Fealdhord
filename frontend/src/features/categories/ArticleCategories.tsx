import { useState } from 'react'
import { useMutation,useQuery,useQueryClient } from '@tanstack/react-query'
import { Dialog,DialogContent,DialogHeader,DialogTitle,DialogTrigger } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { ToggleGroup,ToggleGroupItem } from '@/components/ui/toggle-group'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { navigationAPI,navigationQueryKey } from '@/lib/navigation-api'
import { articleQueryKey } from '@/lib/library'
import { useNavigation } from './use-navigation'
export function ArticleCategories({articleID}:{articleID:string}){
 const [open,setOpen]=useState(false),client=useQueryClient(),navigation=useNavigation()
 const key=['article-memberships',articleID]
 const query=useQuery({queryKey:key,queryFn:()=>navigationAPI.memberships(articleID),enabled:open,retry:false})
 const mutation=useMutation({mutationFn:({id,state}:{id:string;state:string})=>navigationAPI.setMembership(articleID,id,state),onSuccess:()=>{void client.invalidateQueries({queryKey:key});void client.invalidateQueries({queryKey:navigationQueryKey});void client.invalidateQueries({queryKey:articleQueryKey})}})
 return <Dialog open={open} onOpenChange={setOpen}><DialogTrigger asChild><Button variant="ghost" size="icon" aria-label="文章分类"><AnimatedGlyph meaning="folder"/></Button></DialogTrigger><DialogContent aria-describedby={undefined}><DialogHeader><DialogTitle>文章分类</DialogTitle></DialogHeader>
  {query.isPending && <p role="status">正在读取分类…</p>}{query.isError && <div role="alert">分类读取失败<Button variant="ghost" onClick={()=>{void query.refetch()}}>重试</Button></div>}
  {query.data?.map(item=><div className="category-membership" key={item.category_id}><span id={`membership-${item.category_id}`}>{navigation.data?.categories?.find(category=>category.id===item.category_id)?.name ?? '分类'}{item.effective?' · 已加入':''}</span><ToggleGroup type="single" variant="outline" disabled={mutation.isPending} aria-labelledby={`membership-${item.category_id}`} value={item.manual} onValueChange={state=>{if(state)mutation.mutate({id:item.category_id,state})}}><ToggleGroupItem value="auto">自动</ToggleGroupItem><ToggleGroupItem value="included">加入</ToggleGroupItem><ToggleGroupItem value="excluded">排除</ToggleGroupItem></ToggleGroup></div>)}
  {query.isSuccess && !query.data.length && <p>暂无分类</p>}{mutation.error && <p role="alert">{mutation.error.message}</p>}
 </DialogContent></Dialog>
}
