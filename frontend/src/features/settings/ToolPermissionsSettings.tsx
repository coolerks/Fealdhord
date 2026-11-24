import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import type { IconMeaning } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { FieldError } from '@/components/ui/field'
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from '@/components/ui/input-group'
import { SegmentedControl } from '@/components/ui/segmented-control'
import { ToggleGroupItem } from '@/components/ui/toggle-group'
import { builtinToolCategories, getToolMeta } from '@/features/chat/tool-meta'
import type { BuiltinToolCategory } from '@/features/chat/tool-meta'
import { ConfirmDialog, SettingCard, SettingEmpty, SettingRow, SettingSection, StatusBadge } from './settings-primitives'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api'
import type { ToolPermission, ToolPermissionChange } from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/models'

export const toolPermissionsKey = ['ai-platform', 'tool-permissions'] as const

type SourceFilter = 'all' | 'builtin' | 'external'
type StatusFilter = 'all' | 'customized'
type PermissionValue = 'default' | 'bypass' | 'ask' | 'deny'

type ToolEntry = {
  tool: ToolPermission
  isExternal: boolean
  displayName: string
  icon: IconMeaning
  category: BuiltinToolCategory
  searchText: string
}

type ToolGroup = {
  id: string
  title: string
  icon: IconMeaning
  isExternal: boolean
  entries: ToolEntry[]
  customizedCount: number
  availableCount: number
}

const categoryOrder = new Map<BuiltinToolCategory, number>(
  builtinToolCategories.map((item, index) => [item.id, index]),
)
const categoryInfo = new Map(builtinToolCategories.map(item => [item.id, item]))

const permissionSegments: readonly { value: PermissionValue; label: string }[] = [
  { value: 'default', label: '默认' },
  { value: 'bypass', label: '放行' },
  { value: 'ask', label: '审批' },
  { value: 'deny', label: '拒绝' },
]

function buildGroups(items: readonly ToolPermission[], query: string, source: SourceFilter, status: StatusFilter) {
  const term = query.trim().toLowerCase()
  const groupMap = new Map<string, ToolGroup>()

  for (const tool of items) {
    const isExternal = Boolean(tool.connection)
    if (source === 'builtin' && isExternal) continue
    if (source === 'external' && !isExternal) continue
    if (status === 'customized' && !tool.mode) continue

    const meta = getToolMeta(tool.name)
    const displayName = isExternal ? tool.name : meta.label
    const category: BuiltinToolCategory = meta.category ?? 'app'
    const searchText = [displayName, tool.name, tool.connection ?? '', tool.description ?? ''].join(' ').toLowerCase()
    if (term && !searchText.includes(term)) continue

    const groupId = isExternal ? `external:${tool.connection}` : `builtin:${category}`
    let group = groupMap.get(groupId)
    if (!group) {
      const cat = categoryInfo.get(category)
      group = {
        id: groupId,
        title: isExternal ? (tool.connection ?? '外部工具') : (cat?.label ?? '应用与视图'),
        icon: isExternal ? 'webhook' : (cat?.icon ?? 'sliders-horizontal'),
        isExternal,
        entries: [],
        customizedCount: 0,
        availableCount: 0,
      }
      groupMap.set(groupId, group)
    }
    group.entries.push({
      tool,
      isExternal,
      displayName,
      icon: isExternal ? 'wrench' : meta.icon,
      category,
      searchText,
    })
    if (tool.mode) group.customizedCount += 1
    if (tool.available) group.availableCount += 1
  }

  const builtin: ToolGroup[] = []
  const external: ToolGroup[] = []
  for (const group of groupMap.values()) {
    if (group.isExternal) external.push(group)
    else builtin.push(group)
  }
  builtin.sort((a, b) => {
    const aCat = a.id.replace('builtin:', '') as BuiltinToolCategory
    const bCat = b.id.replace('builtin:', '') as BuiltinToolCategory
    return (categoryOrder.get(aCat) ?? 99) - (categoryOrder.get(bCat) ?? 99)
  })
  external.sort((a, b) => a.title.localeCompare(b.title))
  return { builtin, external }
}

function ToolGroupCard({
  group,
  expanded,
  pendingIds,
  onToggle,
  onChangeOne,
  onChangeGroup,
}: {
  group: ToolGroup
  expanded: boolean
  pendingIds: ReadonlySet<string>
  onToggle: () => void
  onChangeOne: (id: string, mode: PermissionValue) => void
  onChangeGroup: (group: ToolGroup, mode: PermissionValue) => void
}) {
  const groupPending = group.entries.some(entry => pendingIds.has(entry.tool.id))
  const allDisconnected = group.availableCount === 0

  return (
    <div className="divide-y divide-border">
      <div className="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
        <button
          type="button"
          aria-expanded={expanded}
          onClick={onToggle}
          className="flex min-w-0 flex-1 items-center gap-2 rounded-md py-0.5 text-left transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <span className="inline-flex shrink-0 text-muted-foreground transition-transform duration-150" style={{ rotate: expanded ? '90deg' : undefined }}>
            <AnimatedGlyph meaning="chevron-right" className="size-3.5 [&_svg]:size-3.5" />
          </span>
          <AnimatedGlyph meaning={group.icon} className="size-3.5 shrink-0 text-muted-foreground [&_svg]:size-3.5" />
          <span className="truncate text-[13px] font-medium">{group.title}</span>
          <span className="shrink-0 text-xs text-muted-foreground tabular-nums">{group.entries.length} 项</span>
          {group.customizedCount > 0 && (
            <StatusBadge tone="active">已改 {group.customizedCount}</StatusBadge>
          )}
          {allDisconnected && (
            <StatusBadge tone="muted">已断开</StatusBadge>
          )}
        </button>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              size="xs"
              variant="ghost"
              disabled={groupPending || (allDisconnected && group.customizedCount === 0)}
              aria-label={`批量设置 ${group.title}`}
            >
              批量设置
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              disabled={allDisconnected}
              onSelect={() => onChangeGroup(group, 'bypass')}
            >
              <AnimatedGlyph meaning="key" />
              整组放行
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={allDisconnected}
              onSelect={() => onChangeGroup(group, 'ask')}
            >
              <AnimatedGlyph meaning="clipboard-check" />
              整组需要审批
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={allDisconnected}
              onSelect={() => onChangeGroup(group, 'deny')}
            >
              <AnimatedGlyph meaning="x" />
              整组拒绝
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              disabled={group.customizedCount === 0}
              onSelect={() => onChangeGroup(group, 'default')}
            >
              <AnimatedGlyph meaning="undo" />
              整组恢复默认
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {expanded && (
        <ul className="divide-y divide-border bg-background/40">
          {group.entries.map(({ tool, displayName, icon }) => {
            const value: PermissionValue = (tool.mode as PermissionValue) || 'default'
            const rowPending = pendingIds.has(tool.id)
            const tooltip = [tool.name !== displayName ? tool.name : '', tool.description ?? ''].filter(Boolean).join('\n')
            return (
              <li
                key={tool.id}
                className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2 px-3.5 py-2 pl-9"
              >
                <div className="flex min-w-0 flex-1 items-center gap-2.5" title={tooltip || undefined}>
                  <AnimatedGlyph meaning={icon} className="size-4 shrink-0 text-muted-foreground [&_svg]:size-4" />
                  <div className="min-w-0 flex-1 space-y-0.5">
                    <div className="flex min-w-0 items-center gap-2">
                      <span className="truncate text-[13px] font-medium leading-snug">{displayName}</span>
                      {!tool.available && <StatusBadge tone="muted">不可用</StatusBadge>}
                      {tool.available && !tool.mode && tool.requiresApproval && (
                        <StatusBadge tone="muted">默认需审批</StatusBadge>
                      )}
                    </div>
                    {tool.description && (
                      <p className="truncate text-xs leading-snug text-muted-foreground">{tool.description}</p>
                    )}
                  </div>
                </div>

                <SegmentedControl
                  size="sm"
                  aria-label={`${displayName} 调用权限`}
                  value={value}
                  disabled={rowPending}
                  onValueChange={next => {
                    if (next === 'default' || next === 'bypass' || next === 'ask' || next === 'deny') {
                      if (next !== value) onChangeOne(tool.id, next)
                    }
                  }}
                >
                  {permissionSegments.map(segment => (
                    <ToggleGroupItem
                      key={segment.value}
                      value={segment.value}
                      disabled={rowPending || (segment.value !== 'default' && !tool.available)}
                      title={
                        segment.value === 'default'
                          ? (tool.requiresApproval ? '默认（需要审批）' : '默认（放行）')
                          : undefined
                      }
                      className="h-6 px-2 text-xs"
                    >
                      {segment.label}
                    </ToggleGroupItem>
                  ))}
                </SegmentedControl>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}

export function ToolPermissionsSettings() {
  const cache = useQueryClient()
  const permissions = useQuery({ queryKey: toolPermissionsKey, queryFn: api.ToolPermissions, retry: false })
  const [query, setQuery] = useState('')
  const [source, setSource] = useState<SourceFilter>('all')
  const [status, setStatus] = useState<StatusFilter>('all')
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(() => new Set())
  const [pendingIds, setPendingIds] = useState<Set<string>>(() => new Set())
  const [confirmResetAll, setConfirmResetAll] = useState(false)
  const [saveError, setSaveError] = useState('')

  const items = useMemo(() => permissions.data ?? [], [permissions.data])
  const hasExternal = useMemo(() => items.some(tool => Boolean(tool.connection)), [items])
  const customizedItems = useMemo(() => items.filter(tool => Boolean(tool.mode)), [items])
  const activeSource: SourceFilter = hasExternal ? source : 'all'
  const autoExpand = Boolean(query.trim()) || status === 'customized'

  const { builtin, external } = useMemo(
    () => buildGroups(items, query, activeSource, status),
    [items, query, activeSource, status],
  )
  const hasVisibleGroups = builtin.length > 0 || external.length > 0

  const batchSave = useMutation({
    mutationFn: async (changes: ToolPermissionChange[]) => {
      if (changes.length === 1) {
        await api.SaveToolPermission(changes[0].id, changes[0].mode)
      } else {
        await api.SaveToolPermissions(changes)
      }
    },
    onMutate: async changes => {
      setSaveError('')
      const ids = changes.map(item => item.id)
      setPendingIds(current => {
        const next = new Set(current)
        for (const id of ids) next.add(id)
        return next
      })
      await cache.cancelQueries({ queryKey: toolPermissionsKey })
      const modeByID = new Map(changes.map(item => [item.id, item.mode]))
      cache.setQueryData<ToolPermission[] | null>(toolPermissionsKey, current => {
        if (!current) return current
        return current
          .map(tool => (modeByID.has(tool.id) ? { ...tool, mode: modeByID.get(tool.id)! } : tool))
          .filter(tool => tool.available || Boolean(tool.mode))
      })
      return { ids }
    },
    onError: () => {
      setSaveError('保存工具权限失败，请重试。')
      void cache.invalidateQueries({ queryKey: toolPermissionsKey })
    },
    onSettled: (_data, _error, _variables, context) => {
      if (context?.ids) {
        setPendingIds(current => {
          const next = new Set(current)
          for (const id of context.ids) next.delete(id)
          return next
        })
      }
      void cache.invalidateQueries({ queryKey: toolPermissionsKey })
    },
  })

  function toggleGroup(id: string) {
    setExpandedGroups(current => {
      const next = new Set(current)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function handleChangeOne(id: string, mode: PermissionValue) {
    batchSave.mutate([{ id, mode: mode === 'default' ? '' : mode }])
  }

  function handleChangeGroup(group: ToolGroup, mode: PermissionValue) {
    const wireMode = mode === 'default' ? '' : mode
    const changes: ToolPermissionChange[] = []
    for (const { tool } of group.entries) {
      if (wireMode === '') {
        if (tool.mode) changes.push({ id: tool.id, mode: '' })
      } else if (tool.available && tool.mode !== wireMode) {
        changes.push({ id: tool.id, mode: wireMode })
      }
    }
    if (changes.length > 0) batchSave.mutate(changes)
  }

  function handleResetAll() {
    const changes = customizedItems.map(tool => ({ id: tool.id, mode: '' }))
    if (changes.length === 0) {
      setConfirmResetAll(false)
      return
    }
    batchSave.mutate(changes, {
      onSuccess: () => setConfirmResetAll(false),
    })
  }

  function renderSection(title: string, groups: ToolGroup[]) {
    if (groups.length === 0) return null
    return (
      <SettingSection title={title}>
        <SettingCard>
          {groups.map(group => (
            <ToolGroupCard
              key={group.id}
              group={group}
              expanded={autoExpand || expandedGroups.has(group.id)}
              pendingIds={pendingIds}
              onToggle={() => toggleGroup(group.id)}
              onChangeOne={handleChangeOne}
              onChangeGroup={handleChangeGroup}
            />
          ))}
        </SettingCard>
      </SettingSection>
    )
  }

  return (
    <div className="space-y-5">
      {permissions.isError ? (
        <SettingSection
          title="工具权限"
          actions={<Button size="sm" variant="outline" onClick={() => void permissions.refetch()}>重试</Button>}
        >
          <SettingCard>
            <SettingRow><FieldError>无法读取工具权限。</FieldError></SettingRow>
          </SettingCard>
        </SettingSection>
      ) : (
        <>
          <div className="space-y-3">
            <InputGroup data-animated-trigger>
              <InputGroupAddon>
                <AnimatedGlyph meaning="search" className="size-3.5 [&_svg]:size-3.5" />
              </InputGroupAddon>
              <InputGroupInput
                aria-label="筛选工具"
                placeholder="搜索工具名称、连接或说明…"
                value={query}
                onChange={event => setQuery(event.target.value)}
              />
              {query && (
                <InputGroupAddon align="inline-end">
                  <InputGroupButton
                    size="icon-xs"
                    aria-label="清空搜索"
                    onClick={() => setQuery('')}
                  >
                    <AnimatedGlyph meaning="x" className="size-3.5 [&_svg]:size-3.5" />
                  </InputGroupButton>
                </InputGroupAddon>
              )}
            </InputGroup>

            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex flex-wrap items-center gap-2">
                {hasExternal && (
                  <SegmentedControl
                    aria-label="筛选工具来源"
                    value={activeSource}
                    onValueChange={next => {
                      if (next === 'all' || next === 'builtin' || next === 'external') setSource(next)
                    }}
                  >
                    <ToggleGroupItem value="all">全部</ToggleGroupItem>
                    <ToggleGroupItem value="builtin">内置</ToggleGroupItem>
                    <ToggleGroupItem value="external">外部</ToggleGroupItem>
                  </SegmentedControl>
                )}

                <SegmentedControl
                  aria-label="筛选权限状态"
                  value={status}
                  onValueChange={next => {
                    if (next === 'all' || next === 'customized') setStatus(next)
                  }}
                >
                  <ToggleGroupItem value="all">全部</ToggleGroupItem>
                  <ToggleGroupItem value="customized">
                    已自定义{customizedItems.length > 0 ? ` ${customizedItems.length}` : ''}
                  </ToggleGroupItem>
                </SegmentedControl>
              </div>

              {customizedItems.length > 0 && (
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={batchSave.isPending}
                  onClick={() => setConfirmResetAll(true)}
                >
                  <AnimatedGlyph meaning="undo" />
                  恢复全部默认
                </Button>
              )}
            </div>
          </div>

          {saveError && <FieldError>{saveError}</FieldError>}

          {permissions.isPending ? (
            <SettingCard>
              <SettingRow><span role="status" className="text-[13px] text-muted-foreground">正在读取工具权限…</span></SettingRow>
            </SettingCard>
          ) : !hasVisibleGroups ? (
            <SettingCard>
              <SettingEmpty
                icon={<AnimatedGlyph meaning="sliders-horizontal" />}
                title={
                  query.trim()
                    ? '没有匹配的工具'
                    : status === 'customized'
                      ? '还没有自定义权限规则'
                      : '暂无可用工具'
                }
              />
            </SettingCard>
          ) : (
            <div className="space-y-5">
              {renderSection('内置工具', builtin)}
              {renderSection('外部 MCP 工具', external)}
            </div>
          )}
        </>
      )}

      <ConfirmDialog
        open={confirmResetAll}
        title="恢复全部工具为默认权限？"
        description={`将清除全部 ${customizedItems.length} 项自定义权限规则。`}
        confirmLabel="确认恢复"
        busy={batchSave.isPending}
        error={confirmResetAll && saveError ? saveError : undefined}
        onOpenChange={open => { if (!open) setConfirmResetAll(false) }}
        onConfirm={handleResetAll}
      />
    </div>
  )
}
