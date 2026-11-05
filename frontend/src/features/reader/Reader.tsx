import { AnimatedGlyph } from '@/components/animated-glyph'
import { ArticleCategories } from '@/features/categories/ArticleCategories'
import { BookOpen,CheckCheck,ChevronLeft,Sparkles,Star } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { motion } from 'motion/react'
import { useEffect,useLayoutEffect,useRef } from 'react'
import type { RefObject } from 'react'
import type { Article } from '@/lib/library'
import type { Preferences } from '@/lib/preferences'
import { readerAPI } from '@/lib/reader-api'
import { SemanticButton } from '@/components/semantic-button'
import { Empty,EmptyHeader,EmptyMedia,EmptyTitle } from '@/components/ui/empty'
import { useWorkspace } from '@/stores/workspace'
import { useReaderContent } from './use-reader-content'

type ReaderProps={article?:Article;preferences:Preferences;overlay:boolean;onClose:()=>void;onFavorite:(id:string)=>void;onRead:(id:string)=>void;onAnchor:(key:string,anchor:{article_id:string;offset:number})=>void}
export function Reader(props:ReaderProps){
 const markedRef=useRef('')
 return <ReaderPane key={`${props.article?.id}:${props.article?.revision ?? 0}`} {...props} markedRef={markedRef}/>
}
function ReaderPane({article,preferences,overlay,onClose,onFavorite,onRead,onAnchor,markedRef}:ReaderProps&{markedRef:RefObject<string>}){
 const content=useRef<HTMLDivElement>(null),timer=useRef<ReturnType<typeof setTimeout>|null>(null),pending=useRef<number|null>(null),save=useRef(onAnchor),title=useRef<HTMLHeadingElement>(null)
 const showSettings=useWorkspace(state=>state.showSettings),articleID=article?.id
 const body=useReaderContent(article,preferences)
 const modeRequest=useWorkspace(state=>state.readerModeRequestID)
 useEffect(()=>{
  let current=true
  queueMicrotask(()=>{
   if(current&&articleID&&modeRequest===articleID&&useWorkspace.getState().readerModeRequestID===articleID){useWorkspace.setState({readerModeRequestID:null});body.toggle()}
  })
  return ()=>{current=false}
 },[articleID,modeRequest,body])
 const hasContent=Boolean(body.text.trim()||body.html)
 useLayoutEffect(()=>{save.current=onAnchor},[onAnchor])
 useLayoutEffect(()=>{if(content.current&&articleID)content.current.scrollTop=useWorkspace.getState().preferences?.scroll_anchors?.[`reader|${articleID}`]?.offset ?? 0},[articleID,body.mode,hasContent])
 useEffect(()=>{if(overlay&&articleID)title.current?.focus({preventScroll:true})},[articleID,overlay])
 useEffect(()=>{
  if(articleID&&preferences.reader_auto_read&&!article?.isRead&&hasContent&&!body.query.isError&&markedRef.current!==articleID){markedRef.current=articleID;onRead(articleID)}
 },[articleID,preferences.reader_auto_read,article?.isRead,hasContent,body.query.isError,onRead,markedRef])
 useLayoutEffect(()=>()=>{if(timer.current)clearTimeout(timer.current);if(articleID&&pending.current!==null)save.current(`reader|${articleID}`,{article_id:articleID,offset:pending.current});pending.current=null},[articleID])
 function openLink(resourceID=''){
  if(articleID)void readerAPI.open(articleID,resourceID).catch(error=>useWorkspace.setState({commandError:error instanceof Error?error.message:'无法打开链接。'}))
 }
 return <motion.aside className="reader-pane" aria-label="阅读面板" data-article-id={articleID} initial={false} animate={{opacity:1}} transition={{duration:.18}}>
  <header className="reader-toolbar"><div className="toolbar-start"><SemanticButton icon={ChevronLeft} meaning="chevron-left" label={overlay?'返回文章浏览':'收起阅读面板'} onClick={onClose}/><Button variant="ghost" size="sm" disabled={!article||body.extracting} aria-pressed={body.mode==='reader'} onClick={body.toggle}>Reader 模式</Button></div><div className="toolbar-actions">{article&&<ArticleCategories articleID={article.id}/>}<SemanticButton icon={Star} meaning="favorite" label={article?.isFavorite?'取消收藏':'收藏当前文章'} active={article?.isFavorite} disabled={!article} onClick={()=>article&&onFavorite(article.id)}/><SemanticButton icon={CheckCheck} meaning="read" label="切换当前文章已读状态" disabled={!article} onClick={()=>article&&onRead(article.id)}/><SemanticButton icon={Sparkles} meaning="ai" label="文章 AI 助手" onClick={()=>showSettings('ai')}/><Button variant="ghost" size="sm" disabled={!article?.url&&!body.query.data?.has_original} onClick={()=>openLink()}>原文</Button></div></header>
  {article?<div ref={content} className="reader-content" onScroll={event=>{
   if(!articleID||!hasContent)return
   const offset=event.currentTarget.scrollTop;pending.current=offset;if(timer.current)clearTimeout(timer.current)
   timer.current=setTimeout(()=>{pending.current=null;save.current(`reader|${articleID}`,{article_id:articleID,offset})},180)
  }}><article style={{maxWidth:preferences.reader_width,fontSize:preferences.reader_font_size,lineHeight:preferences.reader_line_height,fontFamily:preferences.reader_font_family==='serif'?'ui-serif, Georgia, "Songti SC", serif':preferences.reader_font_family==='monospace'?'ui-monospace, SFMono-Regular, monospace':undefined}}><p className="reader-byline">{article.feedTitle} · {article.publishedAt.slice(0,10)}</p><h1 ref={title} tabIndex={-1}>{article.title}</h1>
   {(body.extracting||body.error)&&<div className="reader-operation" role="status">{body.extracting?<><span>正在提取网页正文…</span><Button variant="ghost" size="sm" onClick={body.cancel}>取消提取</Button></>:<><p role="alert">{body.error}</p><Button variant="ghost" size="sm" onClick={()=>{void body.extract()}}>重试提取</Button></>}</div>}
   {body.mode==='reader'&&!body.extracting&&<Button variant="ghost" size="sm" onClick={()=>{void body.extract()}}>重新提取</Button>}
   <div className="reader-prose" data-images={preferences.remote_images==='never'?'blocked':'allowed'} onClick={event=>{
    if(!(event.target instanceof Element))return
    const link=event.target.closest<HTMLAnchorElement>('a[data-reader-link]')
    if(link&&event.currentTarget.contains(link)){event.preventDefault();openLink(link.dataset.readerLink)}
   }} onErrorCapture={event=>{if(event.target instanceof HTMLImageElement)event.target.hidden=true}}>
    {body.query.isError&&!hasContent?<div role="alert">正文读取失败。<Button variant="ghost" onClick={()=>{void body.query.refetch()}}>重试</Button></div>:body.query.isFetching&&!hasContent?<><Skeleton className="h-4 w-full"/><Skeleton className="h-4 w-3/4"/></>:body.html?<div dangerouslySetInnerHTML={{__html:body.html}}/>:body.text.split(/\n\n+/).map((paragraph,index)=><p key={index}>{paragraph}</p>)}
   </div></article></div>:<Empty className="reader-empty"><EmptyHeader><EmptyMedia variant="icon"><AnimatedGlyph icon={BookOpen} meaning="book"/></EmptyMedia><EmptyTitle>选择文章</EmptyTitle></EmptyHeader></Empty>}
 </motion.aside>
}
