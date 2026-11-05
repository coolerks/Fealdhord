import { AnimatedGlyph } from '@/components/animated-glyph'
import { BookOpen, CheckCheck, ChevronLeft, Sparkles, Star } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { libraryAPI } from '@/lib/library-api'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { motion } from 'motion/react'
import { useEffect, useLayoutEffect, useRef } from 'react'
import type { Article } from '@/lib/library'
import type { Preferences } from '@/lib/preferences'
import { SemanticButton } from '@/components/semantic-button'
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import { useWorkspace } from '@/stores/workspace'

export function Reader({ article, preferences, overlay, onClose, onFavorite, onRead, onAnchor }: { article?: Article; preferences: Preferences; overlay: boolean; onClose: () => void; onFavorite: (id: string) => void; onRead: (id: string) => void; onAnchor: (key: string, anchor: { article_id: string; offset: number }) => void }) {
  const content = useRef<HTMLDivElement>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pending = useRef<number | null>(null)
  const save = useRef(onAnchor)
  const title = useRef<HTMLHeadingElement>(null)
  const showSettings = useWorkspace(state => state.showSettings)
  const articleID = article?.id
  const detail=useQuery({queryKey:['library','article',articleID],queryFn:()=>libraryAPI.text(articleID!),enabled:Boolean(articleID)&&!article?.contentText,staleTime:60000,retry:false})
  const contentText=article?.contentText||detail.data||''
  useLayoutEffect(() => { save.current = onAnchor }, [onAnchor])
  useLayoutEffect(() => {
    if (content.current && articleID) content.current.scrollTop = useWorkspace.getState().preferences?.scroll_anchors?.[`reader|${articleID}`]?.offset ?? 0
  }, [articleID,contentText])
  useEffect(() => { if (overlay && articleID) title.current?.focus({ preventScroll: true }) }, [articleID, overlay])
  useLayoutEffect(() => () => {
    if (timer.current) clearTimeout(timer.current)
    if (articleID && pending.current !== null) save.current(`reader|${articleID}`, { article_id: articleID, offset: pending.current })
    pending.current = null
  }, [articleID])
  return <motion.aside className="reader-pane" aria-label="阅读面板" data-article-id={articleID} initial={false} animate={{ opacity: 1 }} transition={{ duration: .18 }}>
    <header className="reader-toolbar"><div className="toolbar-start"><SemanticButton icon={ChevronLeft} meaning="chevron-left" label={overlay ? '返回文章浏览' : '收起阅读面板'} onClick={onClose} /></div><div className="toolbar-actions"><SemanticButton icon={Star} meaning="favorite" label={article?.isFavorite ? '取消收藏' : '收藏当前文章'} active={article?.isFavorite} disabled={!article} onClick={() => article && onFavorite(article.id)} /><SemanticButton icon={CheckCheck} meaning="read" label="切换当前文章已读状态" disabled={!article} onClick={() => article && onRead(article.id)} /><SemanticButton icon={Sparkles} meaning="ai" label="文章 AI 助手" onClick={() => showSettings('ai')} /></div></header>
    {article ? <div ref={content} className="reader-content" onScroll={event => {
      if (!articleID||(!contentText&&detail.isPending)) return
      const offset = event.currentTarget.scrollTop
      pending.current = offset
      if (timer.current) clearTimeout(timer.current)
      timer.current = setTimeout(() => { pending.current = null; save.current(`reader|${articleID}`, { article_id: articleID, offset }) }, 180)
    }}><article style={{ maxWidth: preferences.reader_width, fontSize: preferences.reader_font_size, lineHeight: preferences.reader_line_height }}><p className="reader-byline">{article.feedTitle} · {article.publishedAt.slice(0, 10)}</p><h1 ref={title} tabIndex={-1}>{article.title}</h1><div className="reader-prose">{detail.isError?<div role="alert">正文读取失败。<Button variant="ghost" onClick={()=>{void detail.refetch()}}>重试</Button></div>:detail.isFetching&&!contentText?<><Skeleton className="h-4 w-full"/><Skeleton className="h-4 w-3/4"/></>:contentText.split(/\n\n+/).map((paragraph, index) => <p key={index}>{paragraph}</p>)}</div></article></div> : <Empty className="reader-empty"><EmptyHeader><EmptyMedia variant="icon"><AnimatedGlyph icon={BookOpen} meaning="book"/></EmptyMedia><EmptyTitle>选择文章</EmptyTitle></EmptyHeader></Empty>}
  </motion.aside>
}
