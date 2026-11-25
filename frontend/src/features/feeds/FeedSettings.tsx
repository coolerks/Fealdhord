import { useId, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { GripVertical } from 'lucide-react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { parseFeedBuiltinIcon } from '@/components/custom-icon'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { InputGroup, InputGroupAddon, InputGroupInput } from '@/components/ui/input-group'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { SegmentedControl } from '@/components/ui/segmented-control'
import { Switch } from '@/components/ui/switch'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { useNavigation } from '@/features/categories/use-navigation'
import { ConfirmDialog, SettingCard, SettingEmpty, SettingRow, SettingSection, StatusBadge } from '@/features/settings/settings-primitives'
import { invalidateRSS, startRefresh } from '@/lib/refresh'
import { feedsQueryKey, rssAPI } from '@/lib/rss-api'
import type { Candidate, Feed, UpdateRequest } from '@/lib/rss-api'
import { useRefresh } from '@/stores/refresh'
import { useWorkspace } from '@/stores/workspace'
import type { CategorySummary } from '../../../bindings/github.com/coolerks/Fealdhord/internal/database/models'
import { FeedIcon, FeedIconField } from './FeedIcon'

type FeedReaderOpenMode = 'global' | 'full' | 'feed'
type FeedAutoTranslateMode = 'global' | 'on' | 'off'

function normalizeFeedReaderOpenMode(value?: string): FeedReaderOpenMode {
  return value === 'full' || value === 'feed' ? value : 'global'
}

function normalizeFeedAutoTranslateMode(value?: string): FeedAutoTranslateMode {
  return value === 'on' || value === 'off' ? value : 'global'
}

function FeedAutoTranslateModeField({ value, onChange, disabled }: {
  value: FeedAutoTranslateMode; onChange: (mode: FeedAutoTranslateMode) => void; disabled: boolean
}) {
  const labelID = useId()
  return (
    <Field>
      <FieldLabel id={labelID}>自动翻译</FieldLabel>
      <SegmentedControl
        size="sm"
        aria-labelledby={labelID}
        disabled={disabled}
        value={value}
        onValueChange={next => { if (next === 'global' || next === 'on' || next === 'off') onChange(next) }}
      >
        <ToggleGroupItem value="global"><AnimatedGlyph meaning="settings" />跟随全局设置</ToggleGroupItem>
        <ToggleGroupItem value="on"><AnimatedGlyph meaning="languages" />开启</ToggleGroupItem>
        <ToggleGroupItem value="off"><AnimatedGlyph meaning="x" />关闭</ToggleGroupItem>
      </SegmentedControl>
    </Field>
  )
}

function FeedReaderOpenModeField({ value, onChange, disabled }: {
  value: FeedReaderOpenMode; onChange: (mode: FeedReaderOpenMode) => void; disabled: boolean
}) {
  const labelID = useId()
  return (
    <Field>
      <FieldLabel id={labelID}>点击文章时</FieldLabel>
      <SegmentedControl
        size="sm"
        aria-labelledby={labelID}
        disabled={disabled}
        value={value}
        onValueChange={next => { if (next === 'global' || next === 'full' || next === 'feed') onChange(next) }}
      >
        <ToggleGroupItem value="global"><AnimatedGlyph meaning="settings" />跟随全局设置</ToggleGroupItem>
        <ToggleGroupItem value="full"><AnimatedGlyph meaning="scan-text" />阅读模式</ToggleGroupItem>
        <ToggleGroupItem value="feed"><AnimatedGlyph meaning="book-text" />订阅预览</ToggleGroupItem>
      </SegmentedControl>
    </Field>
  )
}

function Categories({ items, value, onChange, disabled }: {
  items: CategorySummary[]; value: string[]; onChange: (ids: string[]) => void; disabled: boolean
}) {
  if (!items.length) return null
  return (
    <Field>
      <FieldLabel>默认分类</FieldLabel>
      <SearchableSelect
        label="默认分类"
        className="w-full"
        disabled={disabled}
        value={value}
        placeholder="选择默认分类"
        options={items.map(item => ({ value: item.id, label: item.name }))}
        onValueChange={id => onChange(value.includes(id) ? value.filter(item => item !== id) : [...value, id])}
      />
    </Field>
  )
}

export function FeedSettings() {
  const client = useQueryClient()
  const navigation = useNavigation()
  const editController = useRef<AbortController | null>(null)
  const [initialCategories, setInitialCategories] = useState<string[]>([])
  const query = useQuery({ queryKey: feedsQueryKey, queryFn: rssAPI.list, retry: false })
  const [url, setURL] = useState('')
  const [title, setTitle] = useState('')
  const [ids, setIDs] = useState<string[]>([])
  const [candidates, setCandidates] = useState<Candidate[]>([])
  const [selected, setSelected] = useState('')
  const [archived, setArchived] = useState(false)
  const [filter, setFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'error'>('all')
  const [removing, setRemoving] = useState<Feed | null>(null)
  const [useDefaultIcon, setUseDefaultIcon] = useState(false)
  const [selectedIcon, setSelectedIcon] = useState('')
  const [selectedIconColor, setSelectedIconColor] = useState('')
  const [customIconURL, setCustomIconURL] = useState<string>()
  const [readerOpenMode, setReaderOpenMode] = useState<FeedReaderOpenMode>('global')
  const [autoTranslateMode, setAutoTranslateMode] = useState<FeedAutoTranslateMode>('global')
  const [editing, setEditing] = useState<UpdateRequest | null>(null)
  const [dragFeedId, setDragFeedId] = useState<string | null>(null)
  const [dropFeed, setDropFeed] = useState<{ id: string; position: 'before' | 'after' } | null>(null)
  const prefs = useWorkspace(state => state.preferences)!
  const originalFeed = query.data?.items?.find(feed => feed.id === editing?.id)
  const editURLChanged = Boolean(editing && originalFeed && editing.url !== originalFeed.url)
  const storedIcon = originalFeed?.custom_icon_url || parseFeedBuiltinIcon(originalFeed?.icon_data) ? undefined : originalFeed?.icon_data
  const previewURL = editing ? (!editURLChanged && !storedIcon ? editing.url : '') : selected
  const allowIcons = prefs.remote_images !== 'never'
  const iconQuery = useQuery({
    queryKey: ['rss', 'favicon', previewURL],
    queryFn: ({ signal }) => rssAPI.previewIcon(previewURL, signal),
    enabled: allowIcons && Boolean(previewURL),
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
  const previewIcon = allowIcons ? iconQuery.data : undefined
  const refreshPending = useRefresh(state => state.pending)
  const action = useMutation({ mutationFn: async (run: () => Promise<unknown>) => run(), onSuccess: () => invalidateRSS(client) })
  const discovery = useMutation({
    mutationFn: rssAPI.discover,
    onSuccess: items => {
      setCandidates(items)
      setSelected(items.length === 1 ? items[0].url : '')
      setTitle(items.length === 1 ? items[0].title : '')
    },
  })
  const busy = action.isPending || discovery.isPending
  const categories = query.data?.categories ?? []
  const allItems = useMemo(() => query.data?.items ?? [], [query.data?.items])
  const hasErrors = useMemo(() => allItems.some(feed => !feed.archived && Boolean(feed.last_error)), [allItems])
  const visibleFeeds = useMemo(() => {
    const term = filter.trim().toLowerCase()
    return allItems.filter(feed => {
      if (!archived && feed.archived) return false
      if (statusFilter === 'error' && !feed.last_error) return false
      if (!term) return true
      return feed.title.toLowerCase().includes(term) || feed.url.toLowerCase().includes(term)
    })
  }, [allItems, archived, statusFilter, filter])

  function discover() {
    setCustomIconURL(undefined)
    setUseDefaultIcon(false)
    setReaderOpenMode('global')
    setCandidates([])
    setSelected('')
    action.reset()
    discovery.mutate(url.trim())
  }

  return <div className="space-y-6">
    <SettingSection title="添加订阅">
      <SettingCard>
        <form
          className="flex flex-wrap items-end gap-2 p-3.5"
          onSubmit={event => { event.preventDefault(); discover() }}
        >
          <Field className="min-w-0 flex-1">
            <FieldLabel htmlFor="feed-url">订阅或网页地址</FieldLabel>
            <Input
              id="feed-url"
              type="url"
              required
              maxLength={4096}
              disabled={busy}
              placeholder="https://example.com/feed.xml"
              value={url}
              onChange={event => {
                setURL(event.target.value)
                setCandidates([])
                setSelected('')
                discovery.reset()
              }}
            />
          </Field>
          <Button type="submit" disabled={busy || !url.trim()}>
            <AnimatedGlyph meaning="search" />
            {discovery.isPending ? '正在查找…' : '查找订阅'}
          </Button>
        </form>
        {discovery.error && <div className="px-3.5 py-2.5"><FieldError>{discovery.error.message}</FieldError></div>}
        {discovery.isSuccess && !candidates.length && (
          <p role="status" className="px-3.5 py-2.5 text-[13px] text-muted-foreground">没有找到订阅</p>
        )}
        {candidates.length > 0 && (
          <form
            className="space-y-4 bg-muted/20 p-3.5"
            onSubmit={event => {
              event.preventDefault()
              if (!selected) return
              action.mutate(async () => {
                await rssAPI.add({
                  url: selected,
                  title,
                  category_ids: ids,
                  reader_open_mode: readerOpenMode,
                  ...(autoTranslateMode !== 'global' ? { auto_translate_mode: autoTranslateMode } : {}),
                  ...(allowIcons && customIconURL ? { custom_icon_url: customIconURL } : (selectedIcon || selectedIconColor) ? { icon: selectedIcon || 'radio-tower', icon_color: selectedIconColor, use_default_icon: true } : useDefaultIcon ? { use_default_icon: true } : {}),
                })
                setURL('')
                setCandidates([])
                setSelected('')
                setTitle('')
                setIDs([])
                setSelectedIcon('')
                setSelectedIconColor('')
                setReaderOpenMode('global')
                setAutoTranslateMode('global')
              })
            }}
          >
            <Field>
              <FieldLabel id="feed-candidates-label">选择订阅</FieldLabel>
              <ToggleGroup
                type="single"
                variant="outline"
                className="flex w-full flex-col items-stretch gap-1.5"
                aria-labelledby="feed-candidates-label"
                value={selected}
                disabled={busy}
                onValueChange={value => {
                  setCustomIconURL(undefined)
                  setUseDefaultIcon(false)
                  setSelected(value)
                  setTitle(candidates.find(item => item.url === value)?.title ?? '')
                }}
              >
                {candidates.map(item => (
                  <ToggleGroupItem key={item.url} value={item.url} className="feed-candidate h-auto w-full justify-start px-3 py-2">
                    <span className="font-medium">{item.title || item.type}</span>
                    <small className="text-xs text-muted-foreground">{item.url}</small>
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </Field>
            <Field>
              <FieldLabel htmlFor="feed-title">订阅名称</FieldLabel>
              <Input id="feed-title" value={title} maxLength={512} disabled={busy} onChange={event => setTitle(event.target.value)} />
            </Field>
            {selected && (
              <FeedIconField
                data={previewIcon}
                useDefault={useDefaultIcon || !previewIcon}
                onChange={value => { setCustomIconURL(undefined); setUseDefaultIcon(value); if (!value) { setSelectedIcon(''); setSelectedIconColor('') } }}
                disabled={busy}
                siteAvailable={Boolean(previewIcon)}
                customAllowed={allowIcons}
                customURL={customIconURL}
                onCustomURLChange={setCustomIconURL}
                icon={selectedIcon}
                onIconChange={icon => { setCustomIconURL(undefined); setUseDefaultIcon(true); setSelectedIcon(icon) }}
                iconColor={selectedIconColor}
                onIconColorChange={color => { setCustomIconURL(undefined); setUseDefaultIcon(true); setSelectedIconColor(color) }}
              />
            )}
            <FeedReaderOpenModeField value={readerOpenMode} onChange={setReaderOpenMode} disabled={busy} />
            <FeedAutoTranslateModeField value={autoTranslateMode} onChange={setAutoTranslateMode} disabled={busy} />
            <Categories items={categories} value={ids} onChange={setIDs} disabled={busy} />
            {action.error && !editing && !removing && <FieldError>{action.error.message}</FieldError>}
            <div className="flex justify-end gap-2">
              <Button type="button" size="sm" variant="ghost" disabled={busy} onClick={() => { setCandidates([]); setSelected('') }}>取消</Button>
              <Button type="submit" size="sm" disabled={busy || !selected}>
                <AnimatedGlyph meaning="plus" />添加订阅
              </Button>
            </div>
          </form>
        )}
      </SettingCard>
    </SettingSection>

    <SettingSection
      title="已订阅"
      actions={
        <div className="flex flex-wrap items-center gap-3">
          {(hasErrors || statusFilter === 'error') && (
            <SegmentedControl
              aria-label="筛选订阅状态"
              value={statusFilter}
              onValueChange={value => { if (value === 'all' || value === 'error') setStatusFilter(value) }}
            >
              <ToggleGroupItem value="all">全部</ToggleGroupItem>
              <ToggleGroupItem value="error">异常</ToggleGroupItem>
            </SegmentedControl>
          )}
          <div className="flex items-center gap-2">
            <FieldLabel htmlFor="archived-feeds" className="text-xs font-normal text-muted-foreground">显示已归档订阅</FieldLabel>
            <Switch id="archived-feeds" checked={archived} onCheckedChange={setArchived} />
          </div>
        </div>
      }
    >
      {allItems.length > 0 && (
        <InputGroup data-animated-trigger>
          <InputGroupAddon><AnimatedGlyph meaning="search" className="size-3.5 [&_svg]:size-3.5" /></InputGroupAddon>
          <InputGroupInput
            aria-label="筛选订阅"
            placeholder="按名称或地址筛选…"
            value={filter}
            onChange={event => setFilter(event.target.value)}
          />
        </InputGroup>
      )}

      {action.error && !candidates.length && !editing && !removing && <FieldError>{action.error.message}</FieldError>}

      <SettingCard>
        {query.isPending && <p role="status" className="px-3.5 py-4 text-[13px] text-muted-foreground">正在载入订阅…</p>}
        {query.isError && (
          <SettingRow>
            <FieldError>订阅载入失败</FieldError>
            <Button size="sm" variant="outline" onClick={() => { void query.refetch() }}>重试</Button>
          </SettingRow>
        )}
        {query.isSuccess && visibleFeeds.length === 0 && (
          <SettingEmpty
            icon={<AnimatedGlyph meaning="radio-tower" />}
            title={filter.trim() || statusFilter === 'error' ? '没有匹配的订阅' : '暂无订阅'}
          />
        )}
        {visibleFeeds.length > 0 && (() => {
          const canReorder = !archived && statusFilter === 'all' && !filter.trim() && visibleFeeds.length > 1
          function handleFeedDrop(targetId: string, position: 'before' | 'after') {
            if (!dragFeedId || dragFeedId === targetId || busy) return
            const activeIds = allItems.filter(item => !item.archived).map(item => item.id)
            const fromIndex = activeIds.indexOf(dragFeedId)
            const toIndex = activeIds.indexOf(targetId)
            setDragFeedId(null)
            setDropFeed(null)
            if (fromIndex === -1 || toIndex === -1) return
            const nextIds = [...activeIds]
            nextIds.splice(fromIndex, 1)
            const insertAt = nextIds.indexOf(targetId) + (position === 'after' ? 1 : 0)
            nextIds.splice(insertAt, 0, dragFeedId)
            if (nextIds.every((id, index) => id === activeIds[index])) return
            action.reset()
            action.mutate(() => rssAPI.reorder(nextIds))
          }
          return (
          <ul className="divide-y divide-border">
            {visibleFeeds.map(feed => {
              const isDragging = dragFeedId === feed.id
              const indicator = dropFeed?.id === feed.id ? dropFeed.position : null
              return (
              <li
                key={feed.id}
                draggable={canReorder && !busy}
                onDragStart={event => {
                  if (!canReorder || busy) return
                  setDragFeedId(feed.id)
                  event.dataTransfer.effectAllowed = 'move'
                  event.dataTransfer.setData('text/plain', feed.id)
                }}
                onDragOver={event => {
                  if (!canReorder || !dragFeedId || dragFeedId === feed.id || busy) return
                  const rect = event.currentTarget.getBoundingClientRect()
                  const position: 'before' | 'after' = event.clientY - rect.top < rect.height / 2 ? 'before' : 'after'
                  event.preventDefault()
                  event.dataTransfer.dropEffect = 'move'
                  setDropFeed(current => (current?.id === feed.id && current.position === position ? current : { id: feed.id, position }))
                }}
                onDragLeave={event => {
                  if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
                    setDropFeed(current => (current?.id === feed.id ? null : current))
                  }
                }}
                onDrop={event => {
                  event.preventDefault()
                  if (dropFeed?.id === feed.id) handleFeedDrop(feed.id, dropFeed.position)
                  else { setDragFeedId(null); setDropFeed(null) }
                }}
                onDragEnd={() => { setDragFeedId(null); setDropFeed(null) }}
                className={cn(
                  'relative px-3.5 py-2.5 transition-colors',
                  isDragging && 'opacity-40',
                  indicator === 'before' && 'before:pointer-events-none before:absolute before:inset-x-3 before:top-0 before:h-0.5 before:rounded-full before:bg-primary',
                  indicator === 'after' && 'after:pointer-events-none after:absolute after:inset-x-3 after:bottom-0 after:h-0.5 after:rounded-full after:bg-primary',
                )}
              >
                <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
                  <div className="flex min-w-0 flex-1 items-center gap-2" data-animated-trigger>
                    {canReorder && (
                      <span className="inline-flex shrink-0 cursor-grab text-muted-foreground/60 hover:text-muted-foreground active:cursor-grabbing" aria-hidden="true">
                        <GripVertical className="size-4" />
                      </span>
                    )}
                    <div className="min-w-0 flex-1 space-y-0.5">
                    <div className="flex min-w-0 flex-wrap items-center gap-2">
                      <FeedIcon data={allowIcons || parseFeedBuiltinIcon(feed.icon_data) ? feed.icon_data : undefined} />
                      <strong className="min-w-0 font-medium [overflow-wrap:anywhere]">{feed.title}</strong>
                      {feed.archived && <StatusBadge tone="muted">已归档</StatusBadge>}
                      {!feed.archived && feed.last_error && <StatusBadge tone="error">异常</StatusBadge>}
                    </div>
                    <p className="truncate text-xs text-muted-foreground">{feed.url}</p>
                    {feed.last_error && (
                      <p className="text-xs text-destructive">
                        {feed.last_error}
                        {feed.next_check_at && ` · 下次尝试 ${new Date(feed.next_check_at).toLocaleString('zh-CN')}`}
                      </p>
                    )}
                    </div>
                  </div>
                  <div className="flex shrink-0 flex-wrap items-center gap-1">
                    {feed.archived ? (
                      <Button size="sm" variant="outline" disabled={busy}
                        onClick={() => action.mutate(() => rssAPI.restore({ id: feed.id, revision: feed.revision }))}>
                        恢复订阅
                      </Button>
                    ) : (
                      <>
                        <Button size="sm" variant="ghost" disabled={busy || refreshPending} aria-label={`刷新 ${feed.title}`}
                          onClick={() => { void startRefresh(client, [feed.id]) }}>
                          <AnimatedGlyph meaning="refresh" />刷新
                        </Button>
                        <Button size="sm" variant="ghost" disabled={busy} aria-label={`编辑 ${feed.title}`} onClick={() => {
                          action.reset()
                          setInitialCategories(feed.category_ids ?? [])
                          const builtinIcon = parseFeedBuiltinIcon(feed.icon_data)
                          setEditing({
                            apply_existing: true,
                            id: feed.id,
                            revision: feed.revision,
                            url: feed.url,
                            title: feed.title,
                            custom_icon_url: feed.custom_icon_url,
                            icon: builtinIcon?.icon ?? '',
                            icon_color: builtinIcon?.color ?? '',
                            reader_open_mode: normalizeFeedReaderOpenMode(feed.reader_open_mode),
                            auto_translate_mode: normalizeFeedAutoTranslateMode(feed.auto_translate_mode),
                            category_ids: feed.category_ids ?? [],
                          })
                        }}>编辑</Button>
                        <Button size="sm" variant="ghost" disabled={busy} aria-label={`归档 ${feed.title}`} onClick={() => {
                          action.reset()
                          setRemoving(feed)
                        }}>归档</Button>
                      </>
                    )}
                  </div>
                </div>
              </li>
              )
            })}
          </ul>
          )
        })()}
      </SettingCard>
    </SettingSection>

    <Dialog open={Boolean(editing)} onOpenChange={open => { if (!open && !action.isPending) setEditing(null) }}>
      {editing && (
        <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>编辑订阅</DialogTitle>
          </DialogHeader>
          <form onSubmit={event => {
            event.preventDefault()
            action.mutate(async () => {
              editController.current = new AbortController()
              try {
                await rssAPI.update(
                  editURLChanged && editing.use_default_icon === undefined && !editing.custom_icon_url
                    ? { ...editing, use_default_icon: !storedIcon }
                    : editing,
                  editController.current.signal,
                )
                setEditing(null)
              } catch (error) {
                invalidateRSS(client)
                throw error
              } finally {
                editController.current = null
              }
            })
          }}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="edit-feed-title">名称</FieldLabel>
                <Input id="edit-feed-title" required maxLength={512} value={editing.title} disabled={busy}
                  onChange={event => setEditing({ ...editing, title: event.target.value })} />
              </Field>
              <Field>
                <FieldLabel htmlFor="edit-feed-url">订阅地址</FieldLabel>
                <Input id="edit-feed-url" type="url" required maxLength={4096} value={editing.url} disabled={busy}
                  onChange={event => setEditing({ ...editing, url: event.target.value })} />
              </Field>
              <FeedIconField
                data={allowIcons && !editURLChanged ? storedIcon || previewIcon : undefined}
                useDefault={!allowIcons || (editing.use_default_icon ?? !storedIcon)}
                siteAvailable={allowIcons && Boolean(storedIcon || previewIcon || editURLChanged)}
                disabled={busy}
                onChange={use_default_icon => setEditing({ ...editing, use_default_icon, custom_icon_url: undefined, icon: use_default_icon ? editing.icon : '', icon_color: use_default_icon ? editing.icon_color : '' })}
                customAllowed={allowIcons}
                customURL={editing.custom_icon_url}
                customData={allowIcons && editing.custom_icon_url === originalFeed?.custom_icon_url && originalFeed?.custom_icon_url ? originalFeed.icon_data : undefined}
                onCustomURLChange={custom_icon_url => setEditing({ ...editing, custom_icon_url, use_default_icon: undefined, icon: '', icon_color: '' })}
                icon={editing.icon ?? ''}
                onIconChange={icon => setEditing({ ...editing, icon, use_default_icon: true, custom_icon_url: undefined })}
                iconColor={editing.icon_color ?? ''}
                onIconColorChange={icon_color => setEditing({ ...editing, icon_color, use_default_icon: true, custom_icon_url: undefined })}
              />
              <FeedReaderOpenModeField
                value={normalizeFeedReaderOpenMode(editing.reader_open_mode)}
                onChange={next => setEditing({ ...editing, reader_open_mode: next })}
                disabled={busy}
              />
              <FeedAutoTranslateModeField
                value={normalizeFeedAutoTranslateMode(editing.auto_translate_mode)}
                onChange={next => setEditing({ ...editing, auto_translate_mode: next })}
                disabled={busy}
              />
              <Categories items={categories} value={editing.category_ids ?? []} disabled={busy}
                onChange={category_ids => setEditing({ ...editing, category_ids })} />
              {JSON.stringify([...(editing.category_ids ?? [])].sort()) !== JSON.stringify([...initialCategories].sort()) && (
                <Field orientation="horizontal">
                  <FieldLabel htmlFor="apply-existing-categories" className="font-normal">
                    应用到现有 {navigation.data?.feeds?.find(feed => feed.id === editing.id)?.total ?? '…'} 篇文章
                  </FieldLabel>
                  <Switch id="apply-existing-categories" checked={editing.apply_existing !== false} disabled={busy}
                    onCheckedChange={value => setEditing({ ...editing, apply_existing: value })} />
                </Field>
              )}
              {action.error && <FieldError>{action.error.message}</FieldError>}
              <DialogFooter>
                {action.isPending && editController.current && (
                  <Button type="button" size="sm" variant="ghost" onClick={() => editController.current?.abort()}>取消保存</Button>
                )}
                <Button disabled={busy} size="sm" type="button" variant="outline" onClick={() => setEditing(null)}>取消</Button>
                <Button disabled={busy} size="sm" type="submit">保存订阅</Button>
              </DialogFooter>
            </FieldGroup>
          </form>
        </DialogContent>
      )}
    </Dialog>

    <ConfirmDialog
      open={Boolean(removing)}
      title={removing ? `归档“${removing.title}”？` : ''}
      description="停止刷新并从常规浏览中隐藏文章。恢复后会重新显示，收藏和已读状态会保留。"
      confirmLabel="确认归档"
      destructive={false}
      busy={busy}
      error={removing && action.error ? action.error.message : undefined}
      onOpenChange={open => { if (!open) setRemoving(null) }}
      onConfirm={() => {
        if (!removing) return
        action.mutate(async () => {
          await rssAPI.archive({ id: removing.id, revision: removing.revision })
          setRemoving(null)
        })
      }}
    />
  </div>
}
