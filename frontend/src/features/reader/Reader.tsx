import { openChat } from '@/features/chat/chat-scope'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { BookOpen,CalendarDays,ChevronDown,ChevronLeft } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { Tabs } from 'radix-ui'
import { FeedIcon } from '@/features/feeds/FeedIcon'
import { useCallback,useEffect,useEffectEvent,useId,useLayoutEffect,useRef,useState } from 'react'
import type { RefObject } from 'react'
import type { Article } from '@/lib/library'
import type { Preferences } from '@/lib/preferences'
import { readerAPI } from '@/lib/reader-api'
import { readerImageID } from '@/lib/reader-image'
import { SemanticButton } from '@/components/semantic-button'
import { Empty,EmptyHeader,EmptyMedia,EmptyTitle } from '@/components/ui/empty'
import { useWorkspace } from '@/stores/workspace'
import { useReaderContent } from './use-reader-content'
import { useArticleTranslation } from './use-article-translation'
import { useArticleSummary } from './use-article-summary'
import { ReaderToolbarActions } from './ReaderToolbarActions'
import { readerViews } from './toolbar-actions'
import type { ArticleView } from './toolbar-actions'
import { ReaderImageViewer } from './ReaderImageViewer'
import type { ReaderImage } from './ReaderImageViewer'
import { ReaderOutline } from './ReaderOutline'
import { ArticleHTML } from './ArticleHTML'
import { readingBlock } from './reading-block'
import './reader-prose.css'

type ReaderProps={feedIcon?:string;feedReaderOpenMode?:string;feedAutoTranslateMode?:string;article?:Article;preferences:Preferences;overlay:boolean;onClose:()=>void;onFavorite:(id:string)=>void;onRead:(id:string)=>void;onShare?:(article:Article,trigger:HTMLButtonElement)=>void;onAnchor:(key:string,anchor:{article_id:string;offset:number})=>void}
function resolveReaderOpenMode(feedReaderOpenMode: string | undefined, globalMode: string): 'feed' | 'full' {
 if (feedReaderOpenMode === 'full' || feedReaderOpenMode === 'feed') return feedReaderOpenMode
 return globalMode === 'full' ? 'full' : 'feed'
}
export function Reader(props:ReaderProps){
 const markedRef=useRef('')
 const effectiveOpenMode = resolveReaderOpenMode(props.feedReaderOpenMode, props.preferences.reader_open_mode)
 return <ReaderPane key={`${props.article?.id}:${props.article?.revision ?? 0}:${effectiveOpenMode}`} {...props} effectiveOpenMode={effectiveOpenMode} markedRef={markedRef}/>
}

export function ReaderEmpty() {
 return <Empty className="reader-empty"><EmptyHeader><EmptyMedia variant="icon"><AnimatedGlyph icon={BookOpen} meaning="book"/></EmptyMedia><EmptyTitle>选择文章</EmptyTitle></EmptyHeader></Empty>
}

function ReaderPane({feedIcon,feedAutoTranslateMode,article,preferences,effectiveOpenMode,overlay,onClose,onFavorite,onRead,onShare,onAnchor,markedRef}:ReaderProps&{effectiveOpenMode:'feed'|'full';markedRef:RefObject<string>}){
 const content=useRef<HTMLDivElement>(null),timer=useRef<ReturnType<typeof setTimeout>|null>(null),pending=useRef<number|null>(null),save=useRef(onAnchor),title=useRef<HTMLHeadingElement>(null)
 const showSettings = useWorkspace(state => state.showSettings)
 const articleID=article?.id
 const publicationDate=article?.publishedAt.slice(0,10)
 const body=useReaderContent(article,preferences)
 const prose = useRef<HTMLDivElement>(null)
 const readStartBlock = useCallback(() => readingBlock(prose.current, content.current)?.dataset.trBlock, [])
 const imagesAllowed = preferences.remote_images !== 'never'
 const [showToolbarTitle,setShowToolbarTitle]=useState(false)
 const [view,setView]=useState<ArticleView>(effectiveOpenMode==='full'?'reader':'feed')
 const [gallery, setGallery] = useState<{ images: ReaderImage[]; index: number; html: string; origin: DOMRectReadOnly }>()
 const hasContent=Boolean(body.text.trim()||body.html)
 const translation = useArticleTranslation({
  articleID,
  articleRevision: article?.revision,
  sourceMode: body.mode,
  sourceVersion: body.query.data?.source_version,
  readStartBlock,
  hasContent,
  extracting: body.extracting,
  autoReady: view === body.mode || Boolean(body.error),
  defaultDisplay: preferences.reader_translation_display,
  feedAutoTranslateMode,
 })
 const [manualSummary, setManualSummary] = useState(false)
 const [collapsedSummaryVersion, setCollapsedSummaryVersion] = useState<string>()
 const summaryArea = useId()
 const summaryDetail = body.query.data
 const feedOnly = Boolean(summaryDetail?.has_full_feed_body && !summaryDetail.has_original)
 const canUseSummaryBody = view === 'reader' && !body.extracting &&
  (body.mode === 'reader' || feedOnly || Boolean(body.error && summaryDetail?.has_full_feed_body))
 const summaryVersion = canUseSummaryBody ? summaryDetail?.source_version : undefined
 const summary = useArticleSummary(articleID, body.mode, summaryVersion)
 const shouldGenerateSummary = manualSummary && Boolean(summaryVersion)
 const generateManualSummary = useEffectEvent(() => {
  setManualSummary(false)
  void summary.generate()
 })
 useEffect(() => {
  if (!shouldGenerateSummary) return
  let current = true
  queueMicrotask(() => { if (current) generateManualSummary() })
  return () => { current = false }
 }, [shouldGenerateSummary])
 const translatedTitle = translation.record?.translatedTitle || ''
 const headingTitle = translation.effectiveDisplayMode === 'translated' && translatedTitle ? translatedTitle : (article?.title ?? '')
 const subTranslatedTitle = translation.effectiveDisplayMode === 'bilingual' && translatedTitle && translatedTitle !== article?.title ? translatedTitle : ''
 const viewing = imagesAllowed && gallery?.html === body.html ? gallery : undefined
 if (gallery && !viewing) setGallery(undefined)
 function changeView(next:string){
  setGallery(undefined)
  if(next==='feed'){setManualSummary(false);body.leave();setView('feed')}
  else if(next==='reader')setView('reader')
 }
 function requestSummary() {
  if (summaryVersion) {
   void summary.generate()
  } else {
   setManualSummary(true)
   if (view === 'feed') changeView('reader')
   else if (body.error) void body.extract()
  }
 }
 const started=useRef(false)
 const enterReader=useEffectEvent(()=>body.enter())
 const ready=Boolean(article?.contentText)||!body.query.isPending
 useEffect(()=>{
  if(view!=='reader'){started.current=false;return}
  if(!articleID||!ready||started.current||feedOnly)return
  let current=true
  queueMicrotask(()=>{
   if(!current||started.current)return
   started.current=true
   enterReader()
  })
  return ()=>{current=false}
 },[articleID,view,ready,feedOnly])
 const toggleReader=useEffectEvent(()=>changeView(view==='reader'?'feed':'reader'))
 const modeRequest=useWorkspace(state=>state.readerModeRequestID)
 useEffect(()=>{
  let current=true
  queueMicrotask(()=>{
   if(current&&articleID&&modeRequest===articleID&&useWorkspace.getState().readerModeRequestID===articleID){useWorkspace.setState({readerModeRequestID:null});toggleReader()}
  })
  return ()=>{current=false}
 },[articleID,modeRequest])
 useEffect(()=>{
  const root=content.current, heading=title.current
  if(!root||!heading||typeof IntersectionObserver==='undefined')return
  const observer=new IntersectionObserver(([entry])=>{
   if(entry?.rootBounds)setShowToolbarTitle(!entry.isIntersecting && entry.boundingClientRect.bottom<=entry.rootBounds.top)
  },{root})
  observer.observe(heading)
  return ()=>observer.disconnect()
 },[articleID,view])
 const loading=body.extracting||(body.query.isFetching&&!hasContent)
 useLayoutEffect(()=>{save.current=onAnchor},[onAnchor])
 useLayoutEffect(()=>{
  if(!content.current||!articleID)return
  content.current.scrollTop=loading ? 0 : pending.current ?? useWorkspace.getState().preferences?.scroll_anchors?.[`reader|${articleID}`]?.offset ?? 0
 },[articleID,body.mode,view,hasContent,loading])
 useEffect(()=>{if(overlay&&articleID)title.current?.focus({preventScroll:true})},[articleID,overlay])
 useEffect(()=>{
  if(articleID&&preferences.reader_auto_read&&!article?.isRead&&hasContent&&!loading&&!body.query.isError&&markedRef.current!==articleID){markedRef.current=articleID;onRead(articleID)}
 },[articleID,preferences.reader_auto_read,article?.isRead,hasContent,loading,body.query.isError,onRead,markedRef])
 useLayoutEffect(()=>()=>{if(timer.current)clearTimeout(timer.current);if(articleID&&pending.current!==null)save.current(`reader|${articleID}`,{article_id:articleID,offset:pending.current});pending.current=null},[articleID])
 function openLink(resourceID=''){
  if(articleID)void readerAPI.open(articleID,resourceID).catch(error=>useWorkspace.setState({commandError:error instanceof Error?error.message:'无法打开链接。'}))
 }
 function readSelection() {
  const selection = window.getSelection()
  const root = content.current
  if (!selection?.anchorNode || !selection.focusNode || !root?.contains(selection.anchorNode) || !root.contains(selection.focusNode)) return ''
  return Array.from(selection.toString()).slice(0, 6000).join('')
 }
 function viewImage(target: HTMLImageElement) {
  if (!imagesAllowed || !readerImageID(target) || target.hidden) return false
  const images = Array.from(prose.current?.querySelectorAll<HTMLImageElement>('img:not([hidden])') ?? []).flatMap(element => {
   const id = readerImageID(element)
   return id ? [{ id, src: `/media/inline/${id}`, alt: element.alt, element }] : []
  })
  const index = images.findIndex(image => image.element === target)
  if (index === -1) return false
  setGallery({ images, index, html: body.html, origin: target.getBoundingClientRect() })
  return true
 }
 function closeGallery() {
  const target = viewing?.images[viewing.index]?.element
  setGallery(undefined)
  requestAnimationFrame(() => target?.focus({ preventScroll: true }))
 }
 return <aside className="reader-pane" aria-label="阅读面板" data-article-id={articleID}>
  <Tabs.Root className="reader-tabs" value={view} onValueChange={changeView} activationMode="manual">
  {article && <header className="workspace-toolbar reader-toolbar" inert={Boolean(viewing)} aria-hidden={viewing ? true : undefined}>
   <div className="toolbar-start"><SemanticButton icon={ChevronLeft} meaning="chevron-left" label={overlay?'返回文章浏览':'收起阅读面板'} onClick={onClose}/>
    {article && <span className="reader-toolbar-title" data-visible={showToolbarTitle} title={headingTitle} aria-hidden="true">{headingTitle}</span>}
   </div>
   <ReaderToolbarActions articleID={article.id} pinned={preferences.reader_toolbar_pinned} view={view} onViewChange={changeView}
     isFavorite={article.isFavorite} isRead={article.isRead} extracting={body.extracting} hasOriginal={Boolean(article.url || body.query.data?.has_original)}
     translation={{
      displayMode: translation.displayMode,
      targetLang: translation.targetLang,
      hasTranslation: translation.hasTranslation,
      bodyStatus: translation.bodyStatus,
      canTranslate: translation.canTranslate,
      translating: translation.translating,
      onDisplayModeChange: translation.setDisplayMode,
      onTargetLangChange: translation.setTargetLang,
      onTranslate: force => { void translation.translate(force) },
      onCancel: translation.cancel,
     }}
     summary={{
      pending: summary.pending,
      resolved: summary.result?.status === 'generated' || summary.result?.status === 'skipped',
      disabled: body.extracting || (!article.url && !summaryDetail?.has_cached && !summaryDetail?.has_full_feed_body && !summaryVersion),
      onGenerate: requestSummary,
      onCancel: summary.cancel,
     }}
     onRefresh={() => { setGallery(undefined); started.current = true; setView('reader'); void body.extract() }}
     onFavorite={() => onFavorite(article.id)} onRead={() => onRead(article.id)} onOriginal={() => openLink()}
     onShare={onShare && (trigger => onShare(article, trigger))}
     readSelection={readSelection} onAI={selection => openChat({ kind: 'article', id: article.id, label: article.title, selection })} />
  </header>}
  {article?<Tabs.Content value={view} asChild {...(!preferences.reader_toolbar_pinned.includes('view') && {
   'aria-labelledby': undefined, 'aria-label': readerViews.find(mode => mode.value === view)?.label,
  })}><div className="reader-stage" inert={Boolean(viewing)} aria-hidden={viewing ? true : undefined}>
   {!loading && <ReaderOutline mode={preferences.reader_outline} html={body.html} translationKey={`${translation.effectiveDisplayMode}:${translation.targetLang}:${translation.record?.updatedAt ?? 0}`} prose={prose} viewport={content} inert={Boolean(viewing)} />}
   <div ref={content} className="reader-content" onScroll={event=>{
   if(!articleID||!hasContent||loading)return
   const offset=event.currentTarget.scrollTop;pending.current=offset;if(timer.current)clearTimeout(timer.current)
   timer.current=setTimeout(()=>{pending.current=null;save.current(`reader|${articleID}`,{article_id:articleID,offset})},180)
  }}><article style={{maxWidth:preferences.reader_width,fontSize:preferences.reader_font_size,lineHeight:preferences.reader_line_height,fontFamily:preferences.reader_font_family==='serif'?'ui-serif, Georgia, "Songti SC", serif':preferences.reader_font_family==='monospace'?'ui-monospace, SFMono-Regular, monospace':undefined}}><p className="reader-byline">
    <span><FeedIcon data={preferences.remote_images==='never' ? undefined : feedIcon}/>{article.feedTitle}</span>
    {publicationDate && <time dateTime={publicationDate}><CalendarDays aria-hidden="true"/>{publicationDate}</time>}
   </p><h1 ref={title} tabIndex={-1}>{headingTitle}</h1>
   {subTranslatedTitle && <p className="reader-translated-title">{subTranslatedTitle}</p>}
   {(body.extracting||body.error)&&<div className="reader-operation" data-tone={body.extracting ? 'info' : 'warning'} role="status">
    {body.extracting ? <>
     <span className="min-w-0 flex-1">正在提取网页正文…</span>
     <Button variant="ghost" size="sm" onClick={body.cancel}>取消提取</Button>
    </> : <>
     <AnimatedGlyph meaning="badge-alert" className="reader-operation-icon" />
     <p role="alert" className="min-w-0 flex-1 m-0">{body.error}</p>
    </>}
   </div>}
   {!body.extracting && (translation.translating || translation.error) && (
    <div className="reader-operation" data-tone={translation.translating ? 'info' : 'warning'} role="status">
     {translation.translating ? <>
      <span className="min-w-0 flex-1">{translation.record?.translatedBlocks ? `已翻译 ${translation.record.translatedBlocks}/${translation.record.totalBlocks} 段` : '正在生成首批译文…'}</span>
      <Button variant="ghost" size="sm" onClick={translation.cancel}>取消翻译</Button>
     </> : <>
      <AnimatedGlyph meaning="badge-alert" className="reader-operation-icon" />
      <p role="alert" className="min-w-0 flex-1 m-0">{translation.error}</p>
      <div className="flex shrink-0 items-center gap-1.5">
       {translation.error.includes('用途分配') && <>
        <Button variant="outline" size="sm" onClick={() => showSettings('ai')}>配置模型</Button>
        <Button variant="ghost" size="sm" onClick={() => showSettings('article-ai')}>文章 AI 设置</Button>
       </>}
       <Button variant="ghost" size="sm" onClick={() => { void translation.translate(false) }}>{translation.bodyStatus === 'partial' ? '继续翻译' : '重试'}</Button>
      </div>
     </>}
    </div>
   )}
   {summary.pending && <span className="sr-only" role="status">正在生成文章摘要</span>}
   {summary.error && <div className="reader-operation" data-tone="warning">
    <p role="alert" className="min-w-0 flex-1 m-0">{summary.error}</p>
    {summary.needsModel
     ? <Button variant="outline" size="sm" onClick={() => showSettings('ai')}>配置模型</Button>
     : <Button variant="ghost" size="sm" onClick={requestSummary}>重试摘要</Button>}
   </div>}
   {summaryVersion && summary.result?.status === 'generated' &&
    <section aria-label="文章摘要" className="mb-5 rounded-md border bg-muted/35 px-3 py-2">
     <Button type="button" variant="ghost" size="sm" className="w-full justify-between"
      aria-expanded={collapsedSummaryVersion !== summaryVersion} aria-controls={summaryArea}
      onClick={() => setCollapsedSummaryVersion(collapsedSummaryVersion === summaryVersion ? undefined : summaryVersion)}>
      摘要 <ChevronDown data-icon="inline-end" className={collapsedSummaryVersion === summaryVersion ? '-rotate-90' : undefined} />
     </Button>
     <p id={summaryArea} hidden={collapsedSummaryVersion === summaryVersion}
      className="m-0 whitespace-pre-line break-words px-2 pb-1">{summary.result.text}</p>
    </section>}
   <div ref={prose} className="reader-prose" aria-busy={loading || undefined} data-images={imagesAllowed?'allowed':'blocked'} onClick={event=>{
    if(!(event.target instanceof Element))return
    if(event.target instanceof HTMLImageElement && viewImage(event.target)){event.preventDefault();return}
    const link=event.target.closest<HTMLAnchorElement>('a[data-reader-link]')
    if(link&&event.currentTarget.contains(link)){event.preventDefault();openLink(link.dataset.readerLink)}
   }} onKeyDown={event=>{
    if ((event.key === 'Enter' || event.key === ' ') && event.target instanceof HTMLImageElement && viewImage(event.target)) {
     event.preventDefault()
     event.stopPropagation()
    }
   }} onErrorCapture={event=>{if(event.target instanceof HTMLImageElement)event.target.hidden=true}}>
    {loading ? (
     <div className="reader-body-skeleton space-y-[1.25em]" aria-hidden="true">
      <div className="space-y-[0.65em]">
       <Skeleton className="h-[0.8em] w-full" />
       <Skeleton className="h-[0.8em] w-full" />
       <Skeleton className="h-[0.8em] w-3/4" />
      </div>
      <div className="space-y-[0.65em]">
       <Skeleton className="h-[0.8em] w-full" />
       <Skeleton className="h-[0.8em] w-11/12" />
       <Skeleton className="h-[0.8em] w-2/3" />
      </div>
     </div>
    ) : body.query.isError&&!hasContent?<div role="alert">正文读取失败。<Button variant="ghost" onClick={()=>{void body.query.refetch()}}>重试</Button></div>:body.html?<ArticleHTML viewport={content} html={body.html} imagesAllowed={imagesAllowed} theme={preferences.reader_code_theme} translationBlocks={translation.blocksMap} translationMode={translation.effectiveDisplayMode} />:body.text.split(/\n\n+/).map((paragraph,index)=><p key={index}>{paragraph}</p>)}
   </div></article></div>
   </div></Tabs.Content>:<ReaderEmpty />}
  {viewing && <ReaderImageViewer images={viewing.images} initialIndex={viewing.index} origin={viewing.origin} readingViewport={content}
   wheelMode={preferences.reader_image_wheel} backdropClose={preferences.reader_image_backdrop_close} onClose={closeGallery} />}
  </Tabs.Root>
 </aside>
}
