import { useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { SearchableSelect } from '@/components/ui/searchable-select'
import { SegmentedControl } from '@/components/ui/segmented-control'
import { ToggleGroupItem } from '@/components/ui/toggle-group'
import { ConfirmDialog, SettingCard, SettingEmpty, SettingRow, SettingSection } from '@/features/settings/settings-primitives'
import { articleQueryKey } from '@/lib/library'
import { navigationAPI, navigationQueryKey } from '@/lib/navigation-api'
import type { CategoryDeletion, CategoryInfo, CategoryInput } from '@/lib/navigation-api'
import { feedsQueryKey } from '@/lib/rss-api'
import { cn } from '@/lib/utils'
import { useWorkspace } from '@/stores/workspace'
import { categoryIcons } from './category-icons'
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

export function CategoryManager() {
  const query = useNavigation()
  const client = useQueryClient()
  const [draft, setDraft] = useState<CategoryInput | null>(null)
  const [deletion, setDeletion] = useState<CategoryDeletion | null>(null)
  const action = useMutation({
    mutationFn: async (run: () => Promise<unknown>) => run(),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: navigationQueryKey })
      void client.invalidateQueries({ queryKey: articleQueryKey })
      void client.invalidateQueries({ queryKey: feedsQueryKey })
    },
  })
  const busy = action.isPending

  const orderedCategories = useMemo(() => {
    const list = query.data?.categories ?? []
    const byParent = new Map<string, CategoryInfo[]>()
    for (const item of list) {
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
        rows.push({ category: item, depth, path: categoryPath(item, list) })
        walk(item.id, depth + 1)
      }
    }
    walk('', 0)
    for (const item of list) {
      if (!visited.has(item.id)) rows.push({ category: item, depth: 0, path: categoryPath(item, list) })
    }
    return rows
  }, [query.data?.categories])

  return <div className="space-y-6">
    <SettingSection
      title="分类"
      actions={
        <Button size="sm" variant="outline" disabled={busy} onClick={() => { action.reset(); setDraft({ ...empty }) }}>
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
              const icon = categoryIcons.find(name => name === category.icon) ?? 'folder'
              const color = hexPattern.test(category.color) ? category.color : undefined
              return (
                <li key={category.id} className="flex flex-wrap items-center justify-between gap-2 px-3.5 py-2.5">
                  <div className="flex min-w-0 flex-1 items-center gap-2.5" style={{ paddingInlineStart: depth * 18 }} data-animated-trigger>
                    <span className="inline-flex shrink-0 text-muted-foreground" style={{ color }}>
                      <AnimatedGlyph meaning={icon} className="size-4 [&_svg]:size-4" />
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-baseline gap-2">
                        <strong className="truncate font-medium">{category.name}</strong>
                        {depth > 0 && <span className="truncate text-xs text-muted-foreground">{path}</span>}
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
                    ...(query.data?.categories ?? []).filter(item => item.id !== draft.id).map(item => ({
                      value: item.id,
                      label: categoryPath(item, query.data?.categories ?? []),
                    })),
                  ]}
                />
              </Field>
              <Field>
                <FieldLabel id="category-icon-label">图标</FieldLabel>
                <SegmentedControl disabled={busy} value={draft.icon || 'folder'} aria-labelledby="category-icon-label"
                  onValueChange={value => { if (value) setDraft({ ...draft, icon: value }) }}>
                  {categoryIcons.map(icon => (
                    <ToggleGroupItem key={icon} value={icon} aria-label={{ folder: '文件夹', book: '图书', 'radio-tower': '订阅', favorite: '星标', ai: '星光' }[icon]}>
                      <AnimatedGlyph meaning={icon} />
                    </ToggleGroupItem>
                  ))}
                </SegmentedControl>
              </Field>
              <Field>
                <FieldLabel htmlFor="category-color">颜色</FieldLabel>
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    type="button"
                    size="xs"
                    variant={!draft.color ? 'secondary' : 'outline'}
                    disabled={busy}
                    onClick={() => setDraft({ ...draft, color: '' })}
                  >
                    默认
                  </Button>
                  {colorPresets.map(preset => {
                    const active = draft.color.toLowerCase() === preset.value
                    return (
                      <button
                        key={preset.value}
                        type="button"
                        disabled={busy}
                        aria-label={preset.label}
                        aria-pressed={active}
                        title={preset.label}
                        onClick={() => setDraft({ ...draft, color: preset.value })}
                        className={cn(
                          'size-6 rounded-full border border-border transition-transform focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-50',
                          active && 'ring-2 ring-ring ring-offset-2 ring-offset-popover',
                        )}
                        style={{ backgroundColor: preset.value }}
                      />
                    )
                  })}
                  <Input
                    id="category-color"
                    aria-label="自定义颜色值"
                    placeholder="#64748b"
                    maxLength={7}
                    disabled={busy}
                    className="w-24 font-mono text-xs"
                    value={draft.color}
                    onChange={event => setDraft({ ...draft, color: event.target.value })}
                  />
                </div>
              </Field>
              <Field>
                <FieldLabel htmlFor="category-description">描述</FieldLabel>
                <Input id="category-description" disabled={busy} maxLength={2048} value={draft.description}
                  onChange={event => setDraft({ ...draft, description: event.target.value })} />
              </Field>
              <Field>
                <FieldLabel htmlFor="category-order">排序位置</FieldLabel>
                <Input
                  id="category-order"
                  type="number"
                  min={-1000000}
                  max={1000000}
                  disabled={busy}
                  value={draft.sort_position}
                  onChange={event => {
                    if (Number.isFinite(event.target.valueAsNumber)) setDraft({ ...draft, sort_position: event.target.valueAsNumber })
                  }}
                />
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
