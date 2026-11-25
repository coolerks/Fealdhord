import { useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { GripVertical } from 'lucide-react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { CustomIcon } from '@/components/custom-icon'
import { Button } from '@/components/ui/button'
import { ColorPicker } from '@/components/ui/color-picker'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { IconPicker } from '@/components/ui/icon-picker'
import { Input } from '@/components/ui/input'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { ConfirmDialog, SettingCard, SettingEmpty, SettingRow, SettingSection } from '@/features/settings/settings-primitives'
import { articleQueryKey } from '@/lib/library'
import { navigationAPI, navigationQueryKey } from '@/lib/navigation-api'
import type { CategoryDeletion, CategoryInfo, CategoryInput, CategoryOrderItem } from '@/lib/navigation-api'
import { feedsQueryKey } from '@/lib/rss-api'
import { cn } from '@/lib/utils'
import { useWorkspace } from '@/stores/workspace'
import { categoryPath } from './category-path'
import { useNavigation } from './use-navigation'

const empty: CategoryInput = { id: '', revision: 0, name: '', parent_id: '', icon: 'folder', color: '', description: '', sort_position: 0 }

const colorPresets = [
  { value: '#64748b', label: '石板灰' },
  { value: '#b77d4d', label: '暖琥珀' },
  { value: '#c2594b', label: '陶土红' },
  { value: '#4e8a63', label: '松针绿' },
  { value: '#3b8299', label: '湖水蓝' },
  { value: '#4c73b8', label: '钴蓝' },
  { value: '#7c5db5', label: '暮紫' },
  { value: '#b25b88', label: '梅红' },
] as const

const hexPattern = /^#[a-f0-9]{6}$/i

type DropPosition = 'before' | 'inside' | 'after'
type DropTarget = { id: string; position: DropPosition }

function isDescendant(list: CategoryInfo[], ancestorId: string, candidateId: string): boolean {
  let current = list.find(item => item.id === candidateId)
  let guard = 0
  while (current && current.parent_id && guard < 10) {
    if (current.parent_id === ancestorId) return true
    current = list.find(item => item.id === current!.parent_id)
    guard++
  }
  return false
}

function subtreeHeight(list: CategoryInfo[], rootId: string, guard = 0): number {
  if (guard >= 6) return 1
  const kids = list.filter(item => item.parent_id === rootId)
  if (!kids.length) return 1
  return 1 + Math.max(...kids.map(kid => subtreeHeight(list, kid.id, guard + 1)))
}

function nodeDepth(list: CategoryInfo[], id: string): number {
  let depth = 0
  let current = list.find(item => item.id === id)
  while (current && current.parent_id && depth < 10) {
    depth++
    current = list.find(item => item.id === current!.parent_id)
  }
  return depth
}

function buildReorderedCategories(
  list: CategoryInfo[],
  dragId: string,
  targetId: string,
  position: DropPosition,
): CategoryOrderItem[] | null {
  if (dragId === targetId || isDescendant(list, dragId, targetId)) return null
  const dragged = list.find(item => item.id === dragId)
  const target = list.find(item => item.id === targetId)
  if (!dragged || !target) return null

  const newParentId = position === 'inside' ? target.id : target.parent_id
  const targetParentDepth = newParentId ? nodeDepth(list, newParentId) + 1 : 0
  if (targetParentDepth + subtreeHeight(list, dragId) > 5) return null

  const byParent = new Map<string, CategoryInfo[]>()
  for (const item of list) {
    if (item.id === dragId) continue
    const siblings = byParent.get(item.parent_id)
    if (siblings) siblings.push(item)
    else byParent.set(item.parent_id, [item])
  }

  const targetSiblings = [...(byParent.get(newParentId) ?? [])]
  if (position === 'inside') {
    targetSiblings.push(dragged)
  } else {
    const index = targetSiblings.findIndex(item => item.id === targetId)
    if (index === -1) targetSiblings.push(dragged)
    else targetSiblings.splice(position === 'before' ? index : index + 1, 0, dragged)
  }
  byParent.set(newParentId, targetSiblings)

  const updates: CategoryOrderItem[] = []
  for (const [parentID, siblings] of byParent.entries()) {
    siblings.forEach((item, index) => {
      if (item.id === dragId || item.parent_id !== parentID || item.sort_position !== index) {
        updates.push({ id: item.id, parent_id: parentID, sort_position: index })
      }
    })
  }
  return updates.length > 0 ? updates : null
}

export function CategoryManager() {
  const query = useNavigation()
  const client = useQueryClient()
  const [draft, setDraft] = useState<CategoryInput | null>(null)
  const [deletion, setDeletion] = useState<CategoryDeletion | null>(null)
  const [dragId, setDragId] = useState<string | null>(null)
  const [dropTarget, setDropTarget] = useState<DropTarget | null>(null)
  const action = useMutation({
    mutationFn: async (run: () => Promise<unknown>) => run(),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: navigationQueryKey })
      void client.invalidateQueries({ queryKey: articleQueryKey })
      void client.invalidateQueries({ queryKey: feedsQueryKey })
    },
  })
  const busy = action.isPending

  const categories = useMemo(() => query.data?.categories ?? [], [query.data?.categories])

  const orderedCategories = useMemo(() => {
    const byParent = new Map<string, CategoryInfo[]>()
    for (const item of categories) {
      const siblings = byParent.get(item.parent_id)
      if (siblings) siblings.push(item)
      else byParent.set(item.parent_id, [item])
    }
    const rows: { category: CategoryInfo; depth: number; path: string }[] = []
    const visited = new Set<string>()
    function walk(parentID: string, depth: number) {
      if (depth >= 6) return
      for (const item of byParent.get(parentID) ?? []) {
        if (visited.has(item.id)) continue
        visited.add(item.id)
        rows.push({ category: item, depth, path: categoryPath(item, categories) })
        walk(item.id, depth + 1)
      }
    }
    walk('', 0)
    for (const item of categories) {
      if (!visited.has(item.id)) rows.push({ category: item, depth: 0, path: categoryPath(item, categories) })
    }
    return rows
  }, [categories])

  function handleDrop(targetId: string, position: DropPosition) {
    if (!dragId || busy) return
    const updates = buildReorderedCategories(categories, dragId, targetId, position)
    setDragId(null)
    setDropTarget(null)
    if (!updates) return
    action.reset()
    action.mutate(() => navigationAPI.reorder(updates))
  }

  return <div className="space-y-6">
    <SettingSection
      title="分类"
      actions={
        <Button size="sm" variant="outline" disabled={busy} onClick={() => {
          action.reset()
          const rootCount = categories.filter(item => !item.parent_id).length
          setDraft({ ...empty, sort_position: rootCount })
        }}>
          <AnimatedGlyph meaning="plus" />新建分类
        </Button>
      }
    >
      {action.error && !draft && !deletion && <FieldError>{action.error.message}</FieldError>}
      <SettingCard>
        {query.isPending && <p role="status" className="px-3.5 py-4 text-[13px] text-muted-foreground">正在读取分类…</p>}
        {query.isError && (
          <SettingRow>
            <FieldError>分类读取失败</FieldError>
            <Button size="sm" variant="outline" onClick={() => { void query.refetch() }}>重试</Button>
          </SettingRow>
        )}
        {query.isSuccess && orderedCategories.length === 0 && (
          <SettingEmpty icon={<AnimatedGlyph meaning="folder-tree" />} title="暂无分类" />
        )}
        {orderedCategories.length > 0 && (
          <ul className="divide-y divide-border">
            {orderedCategories.map(({ category, depth, path }) => {
              const color = hexPattern.test(category.color) ? category.color : undefined
              const isDragging = dragId === category.id
              const indicator = dropTarget?.id === category.id ? dropTarget.position : null
              return (
                <li
                  key={category.id}
                  draggable={!busy}
                  onDragStart={event => {
                    if (busy) return
                    setDragId(category.id)
                    event.dataTransfer.effectAllowed = 'move'
                    event.dataTransfer.setData('text/plain', category.id)
                  }}
                  onDragOver={event => {
                    if (!dragId || dragId === category.id || busy) return
                    if (isDescendant(categories, dragId, category.id)) return
                    const rect = event.currentTarget.getBoundingClientRect()
                    const ratio = rect.height > 0 ? (event.clientY - rect.top) / rect.height : 0.5
                    let position: DropPosition = ratio < 0.28 ? 'before' : ratio > 0.72 ? 'after' : 'inside'
                    if (position === 'inside' && nodeDepth(categories, category.id) + 1 + subtreeHeight(categories, dragId) > 5) {
                      position = ratio < 0.5 ? 'before' : 'after'
                    }
                    event.preventDefault()
                    event.dataTransfer.dropEffect = 'move'
                    setDropTarget(current => (current?.id === category.id && current.position === position ? current : { id: category.id, position }))
                  }}
                  onDragLeave={event => {
                    if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
                      setDropTarget(current => (current?.id === category.id ? null : current))
                    }
                  }}
                  onDrop={event => {
                    event.preventDefault()
                    if (dropTarget?.id === category.id) {
                      handleDrop(category.id, dropTarget.position)
                    } else {
                      setDragId(null)
                      setDropTarget(null)
                    }
                  }}
                  onDragEnd={() => {
                    setDragId(null)
                    setDropTarget(null)
                  }}
                  className={cn(
                    'relative flex flex-wrap items-center justify-between gap-2 px-3.5 py-2.5 transition-colors',
                    isDragging && 'opacity-40',
                    indicator === 'before' && 'before:pointer-events-none before:absolute before:inset-x-3 before:top-0 before:h-0.5 before:rounded-full before:bg-primary',
                    indicator === 'after' && 'after:pointer-events-none after:absolute after:inset-x-3 after:bottom-0 after:h-0.5 after:rounded-full after:bg-primary',
                    indicator === 'inside' && 'bg-primary/8 ring-1 ring-inset ring-primary/50',
                  )}
                >
                  <div className="flex min-w-0 flex-1 items-center gap-2" style={{ paddingInlineStart: depth * 18 }} data-animated-trigger>
                    <span className="inline-flex shrink-0 cursor-grab text-muted-foreground/60 hover:text-muted-foreground active:cursor-grabbing" aria-hidden="true">
                      <GripVertical className="size-4" />
                    </span>
                    <span className="inline-flex shrink-0 text-muted-foreground" style={{ color }}>
                      <CustomIcon value={category.icon} fallback="folder" className="size-4 [&_svg]:size-4" />
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-baseline gap-2">
                        <strong className="truncate font-medium">{category.name}</strong>
                        {depth > 0 && <span className="truncate text-xs text-muted-foreground">{path}</span>}
                        {indicator === 'inside' && <span className="rounded bg-primary/15 px-1.5 py-0.5 text-[11px] font-medium text-primary">移入子分类</span>}
                      </div>
                      {category.description && <p className="truncate text-xs text-muted-foreground">{category.description}</p>}
                    </div>
                    {category.total > 0 && (
                      <span className="shrink-0 text-xs text-muted-foreground tabular-nums">{category.total} 篇</span>
                    )}
                  </div>
                  <div className="flex shrink-0 items-center gap-1">
                    <Button size="sm" variant="ghost" disabled={busy} aria-label={`编辑分类 ${path}`}
                      onClick={() => { action.reset(); setDraft({ ...category }) }}>编辑</Button>
                    <Button size="sm" variant="ghost" disabled={busy} aria-label={`删除分类 ${path}`}
                      onClick={() => { action.reset(); action.mutate(async () => setDeletion(await navigationAPI.previewDelete(category.id))) }}>删除</Button>
                  </div>
                </li>
              )
            })}
          </ul>
        )}
      </SettingCard>
    </SettingSection>

    <Dialog open={Boolean(draft)} onOpenChange={open => { if (!open && !busy) setDraft(null) }}>
      {draft && (
        <DialogContent aria-describedby={undefined} className="max-h-[85dvh] overflow-y-auto sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{draft.id ? '编辑分类' : '新建分类'}</DialogTitle>
          </DialogHeader>
          <form onSubmit={event => {
            event.preventDefault()
            action.mutate(async () => {
              await navigationAPI.save(draft)
              setDraft(null)
            })
          }}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="category-name">名称</FieldLabel>
                <Input id="category-name" required maxLength={256} disabled={busy} value={draft.name}
                  onChange={event => setDraft({ ...draft, name: event.target.value })} />
              </Field>
              <Field>
                <FieldLabel htmlFor="category-parent">上级分类</FieldLabel>
                <SearchableSelect
                  id="category-parent"
                  label="上级分类"
                  className="w-full"
                  disabled={busy}
                  value={draft.parent_id || 'root-category'}
                  onValueChange={value => setDraft({ ...draft, parent_id: value === 'root-category' ? '' : value })}
                  options={[
                    { value: 'root-category', label: '无上级' },
                    ...categories.filter(item => item.id !== draft.id && !isDescendant(categories, draft.id, item.id)).map(item => ({
                      value: item.id,
                      label: categoryPath(item, categories),
                    })),
                  ]}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="category-icon">图标</FieldLabel>
                <IconPicker
                  id="category-icon"
                  label="分类图标"
                  className="w-full"
                  disabled={busy}
                  value={draft.icon || 'folder-open'}
                  fallback="folder"
                  onChange={icon => setDraft({ ...draft, icon })}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="category-color">颜色</FieldLabel>
                <ColorPicker
                  id="category-color"
                  value={draft.color}
                  presets={colorPresets}
                  disabled={busy}
                  onChange={color => setDraft({ ...draft, color })}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="category-description">描述</FieldLabel>
                <Input id="category-description" disabled={busy} maxLength={2048} value={draft.description}
                  onChange={event => setDraft({ ...draft, description: event.target.value })} />
              </Field>
              {action.error && <FieldError>{action.error.message}</FieldError>}
              <DialogFooter>
                <Button variant="outline" size="sm" type="button" disabled={busy} onClick={() => setDraft(null)}>取消</Button>
                <Button size="sm" type="submit" disabled={busy}>保存分类</Button>
              </DialogFooter>
            </FieldGroup>
          </form>
        </DialogContent>
      )}
    </Dialog>

    <ConfirmDialog
      open={Boolean(deletion)}
      title={deletion ? `删除“${deletion.name}”？` : ''}
      description={deletion ? `将 ${deletion.children} 个子分类移到上一级，移除 ${deletion.articles} 篇文章和 ${deletion.feeds} 个订阅的分类关联。文章和订阅保留。` : undefined}
      confirmLabel="确认删除分类"
      busy={busy}
      error={deletion && action.error ? action.error.message : undefined}
      onOpenChange={open => { if (!open) setDeletion(null) }}
      onConfirm={() => {
        if (!deletion) return
        action.mutate(async () => {
          await navigationAPI.remove(deletion)
          const workspace = useWorkspace.getState()
          if (workspace.preferences?.selected_scope === 'category' && workspace.preferences.selected_scope_id === deletion.id) {
            workspace.patch({ selected_scope: 'all', selected_scope_id: '' })
          }
          setDeletion(null)
        })
      }}
    />
  </div>
}
