import { AnimatedGlyph } from '@/components/animated-glyph'
import { ArticleCategories } from '@/features/categories/ArticleCategories'
import { BookOpen,BookText,CheckCheck,ChevronLeft,ExternalLink,ScanText,Sparkles,Star } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { Tabs } from 'radix-ui'
import { motion } from 'motion/react'
import { useEffect,useEffectEvent,useLayoutEffect,useRef,useState } from 'react'
import type { RefObject } from 'react'
import type { Article } from '@/lib/library'
import type { Preferences } from '@/lib/preferences'
import { readerAPI } from '@/lib/reader-api'
import { SemanticButton } from '@/components/semantic-button'
import { Empty,EmptyHeader,EmptyMedia,EmptyTitle } from '@/components/ui/empty'
import { useWorkspace } from '@/stores/workspace'
import { useReaderContent } from './use-reader-content'

type ArticleView = 'feed' | 'reader'

type ReaderProps={article?:Article;preferences:Preferences;overlay:boolean;onClose:()=>void;onFavorite:(id:string)=>void;onRead:(id:string)=>void;onAnchor:(key:string,anchor:{article_id:string;offset:number})=>void}
export function Reader(props:ReaderProps){
 const markedRef=useRef('')
 return <ReaderPane key={`${props.article?.id}:${props.article?.revision ?? 0}:${props.preferences.reader_open_mode}`} {...props} markedRef={markedRef}/>
}
function ReaderPane({article,preferences,overlay,onClose,onFavorite,onRead,onAnchor,markedRef}:ReaderProps&{markedRef:RefObject<string>}){
 const content=useRef<HTMLDivElement>(null),timer=useRef<ReturnType<typeof setTimeout>|null>(null),pending=useRef<number|null>(null),save=useRef(onAnchor),title=useRef<HTMLHeadingElement>(null)
 const showSettings=useWorkspace(state=>state.showSettings),articleID=article?.id
 const body=useReaderContent(article,preferences)
 const [view,setView]=useState<ArticleView>(preferences.reader_open_mode==='full'?'reader':'feed')
 function changeView(next:string){
  if(next==='feed'){body.leave();setView('feed')}
  else if(next==='reader')setView('reader')
 }
 const started=useRef(false)
 const enterReader=useEffectEvent(()=>body.enter())
 const ready=Boolean(article?.contentText)||!body.query.isPending
 useEffect(()=>{
  if(view!=='reader'){started.current=false;return}
  if(!articleID||!ready||started.current)return
  let current=true
  queueMicrotask(()=>{
   if(!current||started.current)return
   started.current=true
   enterReader()
  })
  return ()=>{current=false}
 },[articleID,view,ready])
 const toggleReader=useEffectEvent(()=>changeView(view==='reader'?'feed':'reader'))
 const modeRequest=useWorkspace(state=>state.readerModeRequestID)
 useEffect(()=>{
  let current=true
  queueMicrotask(()=>{
   if(current&&articleID&&modeRequest===articleID&&useWorkspace.getState().readerModeRequestID===articleID){useWorkspace.setState({readerModeRequestID:null});toggleReader()}
  })
  return ()=>{current=false}
 },[articleID,modeRequest])
 const hasContent=Boolean(body.text.trim()||body.html)
 useLayoutEffect(()=>{save.current=onAnchor},[onAnchor])
 useLayoutEffect(()=>{if(content.current&&articleID)content.current.scrollTop=useWorkspace.getState().preferences?.scroll_anchors?.[`reader|${articleID}`]?.offset ?? 0},[articleID,body.mode,view,hasContent])
 useEffect(()=>{if(overlay&&articleID)title.current?.focus({preventScroll:true})},[articleID,overlay])
 useEffect(()=>{
  if(articleID&&preferences.reader_auto_read&&!article?.isRead&&hasContent&&!body.query.isError&&markedRef.current!==articleID){markedRef.current=articleID;onRead(articleID)}
 },[articleID,preferences.reader_auto_read,article?.isRead,hasContent,body.query.isError,onRead,markedRef])
 useLayoutEffect(()=>()=>{if(timer.current)clearTimeout(timer.current);if(articleID&&pending.current!==null)save.current(`reader|${articleID}`,{article_id:articleID,offset:pending.current});pending.current=null},[articleID])
 function openLink(resourceID=''){
  if(articleID)void readerAPI.open(articleID,resourceID).catch(error=>useWorkspace.setState({commandError:error instanceof Error?error.message:'无法打开链接。'}))
 }
 return <motion.aside className="reader-pane" aria-label="阅读面板" data-article-id={articleID} initial={false} animate={{opacity:1}} transition={{duration:.18}}>
  <Tabs.Root className="reader-tabs" value={view} onValueChange={changeView} activationMode="manual">
  <header className="workspace-toolbar reader-toolbar">
   <div className="toolbar-start"><SemanticButton icon={ChevronLeft} meaning="chevron-left" label={overlay?'返回文章浏览':'收起阅读面板'} onClick={onClose}/></div>
   <div className="toolbar-actions">
    {article&&<ArticleCategories articleID={article.id}/>}
    <SemanticButton icon={Star} meaning="favorite" label="收藏当前文章" active={article?.isFavorite} disabled={!article} onClick={()=>article&&onFavorite(article.id)}/>
    <SemanticButton icon={CheckCheck} meaning="read" label="切换当前文章已读状态" disabled={!article} onClick={()=>article&&onRead(article.id)}/>
    <SemanticButton icon={Sparkles} meaning="ai" label="文章 AI 助手" onClick={()=>showSettings('ai')}/>
    {article&&<Tabs.List className="reader-view-actions" aria-label="文章显示方式">
     <Tabs.Trigger className="reader-view-tab" value="feed" aria-label="订阅预览" title="订阅预览"><AnimatedGlyph icon={BookText} meaning="book-text"/></Tabs.Trigger>
     <Tabs.Trigger className="reader-view-tab" value="reader" aria-label="阅读模式" title="阅读模式"><AnimatedGlyph icon={ScanText} meaning="scan-text"/></Tabs.Trigger>
    </Tabs.List>}
    <SemanticButton icon={ExternalLink} meaning="external-link" label="在浏览器打开原文" disabled={!article?.url&&!body.query.data?.has_original} onClick={()=>openLink()}/>
   </div>
  </header>
  {article?<Tabs.Content value={view} asChild><div ref={content} className="reader-content" onScroll={event=>{
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
   </div></article></div></Tabs.Content>:<Empty className="reader-empty"><EmptyHeader><EmptyMedia variant="icon"><AnimatedGlyph icon={BookOpen} meaning="book"/></EmptyMedia><EmptyTitle>选择文章</EmptyTitle></EmptyHeader></Empty>}
  </Tabs.Root>
 </motion.aside>
}
