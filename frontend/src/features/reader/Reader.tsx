import { BookOpen, CheckCheck, ChevronLeft, Sparkles, Star } from 'lucide-react'
import { motion } from 'motion/react'
import { useEffect, useLayoutEffect, useRef } from 'react'
import type { Article } from '@/lib/library'
import type { Preferences } from '@/lib/preferences'
import { SemanticButton } from '@/components/semantic-button'
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription } from '@/components/ui/empty'
import { useWorkspace } from '@/stores/workspace'

export function Reader({ article, preferences, overlay, onClose, onFavorite, onRead, onAnchor }: { article?: Article; preferences: Preferences; overlay: boolean; onClose: () => void; onFavorite: (id: string) => void; onRead: (id: string) => void; onAnchor: (key: string, anchor: { article_id: string; offset: number }) => void }) {
  const content = useRef<HTMLDivElement>(null)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pending = useRef<number | null>(null)
  const save = useRef(onAnchor)
  const title = useRef<HTMLHeadingElement>(null)
  const showSettings = useWorkspace(state => state.showSettings)
  const articleID = article?.id
  useLayoutEffect(() => { save.current = onAnchor }, [onAnchor])
  useLayoutEffect(() => {
    if (content.current && articleID) content.current.scrollTop = useWorkspace.getState().preferences?.scroll_anchors?.[`reader|${articleID}`]?.offset ?? 0
  }, [articleID])
  useEffect(() => { if (overlay && articleID) title.current?.focus({ preventScroll: true }) }, [articleID, overlay])
  useLayoutEffect(() => () => {
    if (timer.current) clearTimeout(timer.current)
    if (articleID && pending.current !== null) save.current(`reader|${articleID}`, { article_id: articleID, offset: pending.current })
    pending.current = null
  }, [articleID])
  return <motion.aside className="reader-pane" aria-label="阅读面板" data-article-id={articleID} initial={false} animate={{ opacity: 1 }} transition={{ duration: .18 }}>
    <header className="reader-toolbar"><div className="toolbar-start"><SemanticButton icon={ChevronLeft} meaning="collapse" label={overlay ? '返回文章浏览' : '收起阅读面板'} onClick={onClose} /><span>阅读</span></div><div className="toolbar-actions"><SemanticButton icon={Star} meaning="favorite" label={article?.isFavorite ? '取消收藏' : '收藏当前文章'} active={article?.isFavorite} disabled={!article} onClick={() => article && onFavorite(article.id)} /><SemanticButton icon={CheckCheck} meaning="read" label="切换当前文章已读状态" disabled={!article} onClick={() => article && onRead(article.id)} /><SemanticButton icon={Sparkles} meaning="ai" label="文章 AI 助手" onClick={() => showSettings('ai')} /></div></header>
    {article ? <div ref={content} className="reader-content" onScroll={event => {
      if (!articleID) return
      const offset = event.currentTarget.scrollTop
      pending.current = offset
      if (timer.current) clearTimeout(timer.current)
      timer.current = setTimeout(() => { pending.current = null; save.current(`reader|${articleID}`, { article_id: articleID, offset }) }, 180)
    }}><article style={{ maxWidth: preferences.reader_width, fontSize: preferences.reader_font_size, lineHeight: preferences.reader_line_height }}><p className="reader-byline">{article.feedTitle} · {article.publishedAt.slice(0, 10)}</p><h1 ref={title} tabIndex={-1}>{article.title}</h1><div className="reader-prose">{article.contentText.split(/\n\n+/).map((paragraph, index) => <p key={index}>{paragraph}</p>)}</div></article></div> : <Empty className="reader-empty"><EmptyHeader><EmptyMedia variant="icon"><BookOpen /></EmptyMedia><EmptyTitle>留一处，慢慢读</EmptyTitle><EmptyDescription>从文章列表选择一篇内容，<br />在这里开始专注阅读。</EmptyDescription></EmptyHeader><p className="reader-keyhint"><kbd>J</kbd><kbd>K</kbd> 切换文章 <span>·</span> <kbd>F</kbd> 收藏</p></Empty>}
  </motion.aside>
}
