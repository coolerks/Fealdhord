import { useRef, useState } from 'react'
import { Tabs } from 'radix-ui'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { SemanticButton } from '@/components/semantic-button'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem,
  DropdownMenuPortal, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuSeparator,
  DropdownMenuSub, DropdownMenuSubContent, DropdownMenuSubTrigger, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { ArticleCategories } from '@/features/categories/ArticleCategories'
import { defaultTranslationLanguages } from '@/lib/translation-api'
import type { TranslationDisplayMode } from '@/lib/translation-api'
import { useWorkspace } from '@/stores/workspace'
import { readerToolbarOptions, readerViews } from './toolbar-actions'
import type { ArticleView, ReaderToolbarAction } from './toolbar-actions'

export type ReaderTranslationControls = {
  displayMode: TranslationDisplayMode
  targetLang: string
  hasTranslation: boolean
  bodyStatus?: string
  canTranslate: boolean
  translating: boolean
  onDisplayModeChange: (mode: TranslationDisplayMode) => void
  onTargetLangChange: (lang: string) => void
  onTranslate: (force?: boolean) => void
  onCancel: () => void
}

type Props = {
  articleID: string
  pinned: string[]
  view: ArticleView
  isFavorite: boolean
  isRead: boolean
  extracting: boolean
  hasOriginal: boolean
  translation?: ReaderTranslationControls
  summary?: { pending: boolean; resolved: boolean; disabled: boolean; onGenerate: () => void; onCancel: () => void }
  onViewChange: (view: string) => void
  onRefresh: () => void
  onFavorite: () => void
  onRead: () => void
  onOriginal: () => void
  onShare?: (trigger: HTMLButtonElement) => void
  readSelection: () => string
  onAI: (selection: string) => void
}

const translationDisplayOptions: ReadonlyArray<{ value: TranslationDisplayMode; label: string }> = [
  { value: 'bilingual', label: '双语对照' },
  { value: 'translated', label: '仅看译文' },
  { value: 'original', label: '仅看原文' },
]
const languageOptions = defaultTranslationLanguages.map(({ code, label, promptName }) => ({
  value: code, label, keywords: [promptName],
}))

function TranslationMenuItems({ translation, onChooseLanguage }: {
  translation: ReaderTranslationControls
  onChooseLanguage: () => void
}) {
  const completed = translation.bodyStatus === 'completed'
  const skipped = translation.bodyStatus === 'skipped_same_lang'
  const hasPartial = translation.bodyStatus === 'partial'
  const languageLabel = languageOptions.find(item => item.value === translation.targetLang)?.label ?? translation.targetLang
  return <>
    <DropdownMenuRadioGroup
      aria-label="译文显示方式"
      value={translation.displayMode}
      onValueChange={next => {
        if (next === 'bilingual' || next === 'translated' || next === 'original') {
          translation.onDisplayModeChange(next)
        }
      }}
    >
      {translationDisplayOptions.map(option => (
        <DropdownMenuRadioItem key={option.value} value={option.value}>{option.label}</DropdownMenuRadioItem>
      ))}
    </DropdownMenuRadioGroup>
    <DropdownMenuSeparator />
    <DropdownMenuItem onSelect={onChooseLanguage}>
      <AnimatedGlyph meaning="languages" />目标语言（{languageLabel}）
    </DropdownMenuItem>
    <DropdownMenuSeparator />
    {translation.translating ? (
      <DropdownMenuItem onSelect={translation.onCancel}>
        <AnimatedGlyph meaning="x" />取消翻译
      </DropdownMenuItem>
    ) : (
      <DropdownMenuItem disabled={!translation.canTranslate} onSelect={() => translation.onTranslate(completed || skipped)}>
        <AnimatedGlyph meaning="refresh" />{completed ? '重新翻译' : skipped ? '仍要翻译' : hasPartial ? '继续翻译' : '翻译正文'}
      </DropdownMenuItem>
    )}
  </>
}

export function ReaderToolbarActions({
  articleID, pinned, view, isFavorite, isRead, extracting, hasOriginal, translation, summary, onViewChange,
  onRefresh, onFavorite, onRead, onOriginal, onShare, readSelection, onAI,
}: Props) {
  const [categoriesOpen, setCategoriesOpen] = useState(false)
  const [languageOpen, setLanguageOpen] = useState(false)
  const categoryTrigger = useRef<HTMLButtonElement>(null)
  const translateTrigger = useRef<HTMLButtonElement>(null)
  const moreTrigger = useRef<HTMLButtonElement>(null)
  const returnFocus = useRef<HTMLButtonElement | null>(null)
  const selection = useRef('')
  const afterMenuClose = useRef<(() => void) | null>(null)
  const showSettings = useWorkspace(state => state.showSettings)
  const pinnedOptions = readerToolbarOptions.filter(option => pinned.includes(option.value))
  const overflowOptions = readerToolbarOptions.filter(option => !pinned.includes(option.value))
  const readLabel = isRead ? '标为未读' : '标为已读'

  function openCategories(trigger: HTMLButtonElement | null) {
    returnFocus.current = trigger
    setCategoriesOpen(true)
  }

  function chooseLanguage(trigger: HTMLButtonElement | null) {
    afterMenuClose.current = () => {
      returnFocus.current = trigger
      setLanguageOpen(true)
    }
  }

  function runAfterMenuClose(event: Event, trigger: HTMLButtonElement | null) {
    const run = afterMenuClose.current
    afterMenuClose.current = null
    if (!run) return
    event.preventDefault()
    trigger?.focus()
    run()
  }

  function isDisabled(action: ReaderToolbarAction) {
    return (action === 'translate' && !translation) || (action === 'refresh' && extracting) ||
      ((action === 'original' || action === 'refresh') && !hasOriginal)
  }

  function runAction(action: ReaderToolbarAction, fromMenu = false) {
    switch (action) {
      case 'refresh': onRefresh(); break
      case 'favorite': onFavorite(); break
      case 'read': onRead(); break
      case 'original': onOriginal(); break
      case 'categories':
        if (fromMenu) afterMenuClose.current = () => openCategories(moreTrigger.current)
        else openCategories(categoryTrigger.current)
        break
      case 'ai':
        if (fromMenu) {
          const text = selection.current
          afterMenuClose.current = () => onAI(text)
        } else onAI(readSelection())
        break
    }
  }

  return <div className="toolbar-actions reader-toolbar-actions">
    <div className="reader-pinned-actions">
      {pinnedOptions.map(({ value, label, icon, meaning }) => {
        if (value === 'view') {
          return <Tabs.List key={value} className="reader-view-actions" aria-label={label}>
            {readerViews.map(mode => <Tabs.Trigger key={mode.value} className="reader-view-tab" value={mode.value} aria-label={mode.label} title={mode.label}>
              <AnimatedGlyph icon={mode.icon} meaning={mode.meaning} />
            </Tabs.Trigger>)}
          </Tabs.List>
        }
        if (value === 'translate' && translation) {
          const active = translation.translating || (translation.hasTranslation && translation.displayMode !== 'original')
          return <DropdownMenu key={value}>
            <DropdownMenuTrigger asChild>
              <SemanticButton ref={translateTrigger} icon={icon} meaning={meaning} label={label} active={active} aria-pressed={active} />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" onCloseAutoFocus={event => runAfterMenuClose(event, translateTrigger.current)}>
              <TranslationMenuItems translation={translation} onChooseLanguage={() => chooseLanguage(translateTrigger.current)} />
            </DropdownMenuContent>
          </DropdownMenu>
        }
        return <SemanticButton key={value} ref={value === 'categories' ? categoryTrigger : undefined} icon={icon} meaning={meaning} label={value === 'read' ? readLabel : label}
          disabled={isDisabled(value)}
          active={value === 'favorite' && isFavorite} aria-pressed={value === 'favorite' ? isFavorite : undefined}
          onMouseDown={value === 'ai' ? event => { if (event.button === 0 && readSelection()) event.preventDefault() } : undefined}
          onClick={() => runAction(value)} />
      })}
    </div>
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button ref={moreTrigger} variant="ghost" size="icon" aria-label="更多" title="更多"
          onPointerDown={() => { selection.current = readSelection() }}
          onKeyDown={event => { if (['Enter', ' ', 'ArrowDown'].includes(event.key)) selection.current = readSelection() }}>
          <AnimatedGlyph meaning="ellipsis" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" onCloseAutoFocus={event => runAfterMenuClose(event, moreTrigger.current)}>
        <DropdownMenuGroup>
          {overflowOptions.map(({ value, icon, meaning, label }) => {
            if (value === 'view') {
              return <DropdownMenuSub key={value}>
                <DropdownMenuSubTrigger>
                  <AnimatedGlyph icon={icon} meaning={meaning} />{label}
                </DropdownMenuSubTrigger>
                <DropdownMenuPortal>
                  <DropdownMenuSubContent sideOffset={4}>
                    <DropdownMenuRadioGroup aria-label={label} value={view} onValueChange={onViewChange}>
                      {readerViews.map(mode => <DropdownMenuRadioItem key={mode.value} value={mode.value}>
                        <AnimatedGlyph icon={mode.icon} meaning={mode.meaning} />{mode.label}
                      </DropdownMenuRadioItem>)}
                    </DropdownMenuRadioGroup>
                  </DropdownMenuSubContent>
                </DropdownMenuPortal>
              </DropdownMenuSub>
            }
            if (value === 'translate' && translation) {
              return <DropdownMenuSub key={value}>
                <DropdownMenuSubTrigger>
                  <AnimatedGlyph icon={icon} meaning={meaning} />{label}
                </DropdownMenuSubTrigger>
                <DropdownMenuPortal>
                  <DropdownMenuSubContent sideOffset={4}>
                    <TranslationMenuItems translation={translation} onChooseLanguage={() => chooseLanguage(moreTrigger.current)} />
                  </DropdownMenuSubContent>
                </DropdownMenuPortal>
              </DropdownMenuSub>
            }
            if (value === 'favorite') {
              return <DropdownMenuCheckboxItem key={value} checked={isFavorite} onSelect={onFavorite}>
                <AnimatedGlyph icon={icon} meaning={meaning} />{label}
              </DropdownMenuCheckboxItem>
            }
            return <DropdownMenuItem key={value} disabled={isDisabled(value)} onSelect={() => runAction(value, true)}>
              <AnimatedGlyph icon={icon} meaning={meaning} />{value === 'read' ? readLabel : label}
            </DropdownMenuItem>
          })}
        </DropdownMenuGroup>
        {summary && <DropdownMenuGroup>
          <DropdownMenuItem disabled={summary.disabled || summary.resolved}
            onSelect={summary.pending ? summary.onCancel : summary.onGenerate}>
            <AnimatedGlyph meaning={summary.pending ? 'x' : 'book-text'} />{summary.pending ? '取消摘要' : '生成摘要'}
          </DropdownMenuItem>
        </DropdownMenuGroup>}
        {onShare && <DropdownMenuItem onSelect={() => {
          afterMenuClose.current = () => { if (moreTrigger.current) onShare(moreTrigger.current) }
        }}><AnimatedGlyph meaning="send" />发送到…</DropdownMenuItem>}
        {(overflowOptions.length > 0 || onShare || summary) && <DropdownMenuSeparator />}
        <DropdownMenuGroup>
          <DropdownMenuItem onSelect={() => { afterMenuClose.current = () => showSettings('reader') }}>
            <AnimatedGlyph meaning="settings" />工具栏设置…
          </DropdownMenuItem>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
    {categoriesOpen && <ArticleCategories articleID={articleID} open={categoriesOpen} onOpenChange={setCategoriesOpen}
      onCloseAutoFocus={event => {
        event.preventDefault()
        const trigger = returnFocus.current
        if (trigger?.isConnected) trigger.focus()
        else moreTrigger.current?.focus()
      }} />}
    {translation && <Dialog open={languageOpen} onOpenChange={setLanguageOpen}>
      <DialogContent aria-describedby={undefined} onCloseAutoFocus={event => {
        event.preventDefault()
        const trigger = returnFocus.current
        if (trigger?.isConnected) trigger.focus()
        else moreTrigger.current?.focus()
      }}>
        <DialogHeader><DialogTitle>目标语言</DialogTitle></DialogHeader>
        <SearchableSelect label="目标语言" value={translation.targetLang} options={languageOptions}
          announceSelection onValueChange={translation.onTargetLangChange} />
      </DialogContent>
    </Dialog>}
  </div>
}
