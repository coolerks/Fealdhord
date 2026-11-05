import { useEffect,useRef,useState } from 'react'
import { useQuery,useQueryClient } from '@tanstack/react-query'
import type { Detail } from '@/lib/reader-api'
import { readerAPI } from '@/lib/reader-api'
import type { Article } from '@/lib/library'
import type { Preferences } from '@/lib/preferences'
export function useReaderContent(article:Article|undefined,preferences:Preferences){
 const [mode,setMode]=useState<'feed'|'reader'>('feed'),[extracting,setExtracting]=useState(false),[error,setError]=useState('')
 const controller=useRef<AbortController|null>(null),generation=useRef(0)
 const client=useQueryClient(),id=article?.id,revision=article?.revision ?? 0
 const key=['reader','content',id,revision,mode,preferences.remote_images]
 const query=useQuery({queryKey:key,queryFn:({signal})=>readerAPI.get(id!,mode,signal),enabled:Boolean(id)&&!article?.contentText,staleTime:60000,retry:false})
 useEffect(()=>()=>{generation.current++;controller.current?.abort()},[id,revision])
 useEffect(()=>{
  if(mode==='reader' && query.error && 'code' in query.error && query.error.code==='NotFound'){setMode('feed');setError('文章已更新，已返回订阅正文。')}
 },[mode,query.error])
 async function extract(){
  if(!id||extracting)return
  const request=new AbortController(),version=++generation.current
  controller.current=request;setExtracting(true);setError('')
  try{
   const detail=await readerAPI.extract(id,request.signal)
   if(request.signal.aborted||generation.current!==version)return
   client.setQueryData(['reader','content',id,revision,'reader',preferences.remote_images],detail)
   client.setQueryData<Detail>(['reader','content',id,revision,'feed',preferences.remote_images],previous=>previous?{...previous,has_cached:true}:previous)
   setMode('reader')
  }catch(error){if(generation.current===version)setError(error instanceof Error?error.message:'提取失败，当前正文已保留。')}
  finally{if(generation.current===version){controller.current=null;setExtracting(false)}}
 }
 function cancel(){controller.current?.abort();generation.current++;controller.current=null;setExtracting(false);setError('提取已取消，当前正文已保留。')}
 function toggle(){if(extracting)return;if(mode==='reader')setMode('feed');else if(query.data?.has_cached)setMode('reader');else void extract()}
 return {query,mode,extracting,error,extract,cancel,toggle,text:article?.contentText||query.data?.text||'',html:article?.contentText?'':query.data?.html||''}
}
