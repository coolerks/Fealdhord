import type { ComponentProps } from 'react'
import { useMutation,useQuery,useQueryClient } from '@tanstack/react-query'
import { Dialog,DialogContent,DialogHeader,DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { SegmentedControl } from '@/components/ui/segmented-control'
import {ToggleGroupItem } from '@/components/ui/toggle-group'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { navigationAPI,navigationQueryKey } from '@/lib/navigation-api'
import { articleQueryKey } from '@/lib/library'
import { useNavigation } from './use-navigation'
type Props = {
 articleID: string
 open: boolean
 onOpenChange: (open: boolean) => void
 onCloseAutoFocus?: ComponentProps<typeof DialogContent>['onCloseAutoFocus']
}
export function ArticleCategories({articleID,open,onOpenChange,onCloseAutoFocus}:Props){
 const client=useQueryClient(),navigation=useNavigation()
 const key=['article-memberships',articleID]
 const query=useQuery({queryKey:key,queryFn:()=>navigationAPI.memberships(articleID),enabled:open,retry:false})
 const mutation=useMutation({mutationFn:({id,state}:{id:string;state:string})=>navigationAPI.setMembership(articleID,id,state),onSuccess:()=>{void client.invalidateQueries({queryKey:key});void client.invalidateQueries({queryKey:navigationQueryKey});void client.invalidateQueries({queryKey:articleQueryKey})}})
 return <Dialog open={open} onOpenChange={onOpenChange}><DialogContent aria-describedby={undefined} onCloseAutoFocus={onCloseAutoFocus}><DialogHeader><DialogTitle>文章分类</DialogTitle></DialogHeader>
  {query.isPending && <p role="status">正在读取分类…</p>}{query.isError && <div role="alert">分类读取失败<Button variant="ghost" onClick={()=>{void query.refetch()}}>重试</Button></div>}
  {query.data?.map(item=><div className="category-membership" key={item.category_id}><span id={`membership-${item.category_id}`}>{navigation.data?.categories?.find(category=>category.id===item.category_id)?.name ?? '分类'}{item.effective?' · 已加入':''}</span><SegmentedControl disabled={mutation.isPending} aria-labelledby={`membership-${item.category_id}`} value={item.manual} onValueChange={state=>{if(state)mutation.mutate({id:item.category_id,state})}}><ToggleGroupItem value="auto"><AnimatedGlyph meaning="refresh"/>自动</ToggleGroupItem><ToggleGroupItem value="included"><AnimatedGlyph meaning="plus"/>加入</ToggleGroupItem><ToggleGroupItem value="excluded"><AnimatedGlyph meaning="x"/>排除</ToggleGroupItem></SegmentedControl></div>)}
  {query.isSuccess && !query.data.length && <p>暂无分类</p>}{mutation.error && <p role="alert">{mutation.error.message}</p>}
 </DialogContent></Dialog>
}
