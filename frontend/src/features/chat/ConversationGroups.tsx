import { useState } from 'react'
import type { ReactElement } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { create } from 'zustand'
import { Archive, ChevronRight, Folder, Pencil, Trash2, X } from 'lucide-react'
import { CustomIcon, hexColorRegex } from '@/components/custom-icon'
import { IconAppearanceFields } from '@/components/icon-appearance-fields'
import { ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuTrigger } from '@/components/ui/context-menu'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { cn } from '@/lib/utils'
import { GroupDetails, PickGroupFolders, SaveGroup } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/api'
import type { Group } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'

export const groupKey = ['chat', 'groups'] as const
export const archivedGroupsKey = ['chat', 'archived-groups'] as const
export const groupHistoryKey = ['chat', 'group-conversations'] as const

export const useConversationFolds = create<{
  closed: Record<string, boolean>
  toggle: (key: string) => void
}>(set => ({
  closed: {},
  toggle: key => set(state => ({ closed: { ...state.closed, [key]: !state.closed[key] } })),
}))

export function GroupDisclosure({ label, open, controls, onToggle, group }: {
  label: string
  open: boolean
  controls: string
  onToggle: () => void
  group?: Group
}) {
  return <Button type="button" variant="ghost" size="sm" data-navigation={controls}
    className="min-w-0 flex-1 justify-start px-2.5" aria-expanded={open} aria-controls={controls} onClick={onToggle}>
    {group && <GroupIcon group={group} />}
    <span className="truncate" title={label}>{label}</span>
    {!group && <ChevronRight className={cn('size-3 shrink-0', open && 'rotate-90')} aria-hidden="true" />}
  </Button>
}

export function GroupIcon({ group }: { group: Group }) {
  const color = group.color && hexColorRegex.test(group.color) ? group.color : undefined
  return <span className="inline-flex size-4 shrink-0 items-center justify-center" style={{ color }}>
    {group.icon ? <CustomIcon value={group.icon} className="size-4" /> : <Folder className="size-4" aria-hidden="true" />}
  </span>
}

export type GroupAction = 'edit' | 'archive' | 'remove'

export function GroupMenu({ children, onAction, disabled }: { children: ReactElement; onAction: (action: GroupAction) => void; disabled: boolean }) {
  return <ContextMenu>
    <ContextMenuTrigger asChild disabled={disabled} data-conversation-menu onKeyDown={event => {
      if (disabled || (event.key !== 'ContextMenu' && !(event.shiftKey && event.key === 'F10'))) return
      event.preventDefault()
      event.stopPropagation()
      const { left, bottom } = (document.activeElement ?? event.currentTarget).getBoundingClientRect()
      event.currentTarget.dispatchEvent(new MouseEvent('contextmenu', {
        bubbles: true, cancelable: true, clientX: left, clientY: bottom,
      }))
    }}>{children}</ContextMenuTrigger>
    <ContextMenuContent aria-label="分组操作" className="action-menu">
      <ContextMenuItem disabled={disabled} onSelect={() => onAction('edit')}><Pencil />编辑分组</ContextMenuItem>
      <ContextMenuItem disabled={disabled} onSelect={() => onAction('archive')}><Archive />归档分组</ContextMenuItem>
      <ContextMenuItem disabled={disabled} variant="destructive" onSelect={() => onAction('remove')}><Trash2 />移除分组</ContextMenuItem>
    </ContextMenuContent>
  </ContextMenu>
}

export function groupActionError(error: unknown) {
  const code = error instanceof Error ? error.message : ''
  if (code.includes('ChatFolderUnavailable')) return '所选文件夹已不可用。'
  if (code.includes('ChatGroupUnavailable')) return '分组已不可用，请刷新后重试。'
  if (code.includes('Conflict')) return '请先等待组内会话回复结束或停止回复。'
  if (code.includes('ValidationError')) return '请检查分组名称、文件夹、图标与颜色。'
  return '无法保存分组变更，请重试。'
}

export function ConversationGroupDialog({ groupID, onClose, onSaved }: {
  groupID?: string
  onClose: () => void
  onSaved: (group: Group) => void
}) {
  const details = useQuery({
    queryKey: [...groupKey, groupID], queryFn: () => GroupDetails(groupID ?? ''), enabled: Boolean(groupID), retry: false,
  })
  if (groupID && (!details.data || details.isFetching)) {
    return <Dialog open onOpenChange={open => { if (!open) onClose() }}>
      <DialogContent>
        <DialogHeader><DialogTitle>编辑会话分组</DialogTitle></DialogHeader>
        {details.isError ? <><FieldError>{groupActionError(details.error)}</FieldError>
          <Button variant="outline" onClick={() => void details.refetch()}>重试</Button></> :
          <p role="status">正在读取分组…</p>}
      </DialogContent>
    </Dialog>
  }
  return <GroupForm group={details.data} onClose={onClose} onSaved={onSaved} />
}

function GroupForm({ group, onClose, onSaved }: { group?: Group; onClose: () => void; onSaved: (group: Group) => void }) {
  const [name, setName] = useState(group?.name ?? '')
  const [folders, setFolders] = useState(group?.folders ?? [])
  const [icon, setIcon] = useState(group?.icon ?? '')
  const [color, setColor] = useState(group?.color ?? '')
  const picker = useMutation({
    mutationFn: () => PickGroupFolders(),
    onSuccess: selected => setFolders(current => [...new Set([...current, ...(selected ?? [])])]),
  })
  const save = useMutation({
    mutationFn: () => SaveGroup({ id: group?.id, name: name.trim(), folders, icon, color }),
    onSuccess: onSaved,
  })
  const busy = picker.isPending || save.isPending
  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose() }}>
    <DialogContent aria-describedby={undefined}>
      <DialogHeader><DialogTitle>{group ? '编辑会话分组' : '新建会话分组'}</DialogTitle></DialogHeader>
      <form className="flex flex-col gap-4" onSubmit={event => {
        event.preventDefault()
        if (!busy && name.trim()) save.mutate()
      }}>
        <FieldGroup>
        <Field>
          <FieldLabel htmlFor="new-chat-group-name">分组名称</FieldLabel>
          <Input id="new-chat-group-name" value={name} required maxLength={160} disabled={busy}
            onChange={event => setName(event.target.value)} />
        </Field>
        <IconAppearanceFields icon={icon} color={color} fallback="folder" disabled={busy}
          onIconChange={setIcon} onColorChange={setColor} />
        </FieldGroup>
        <Button type="button" variant="outline" size="sm" disabled={busy || folders.length >= 32}
          onClick={() => picker.mutate()}><Folder />添加文件夹</Button>
        {folders.length > 0 && <ul className="flex flex-col gap-1">
          {folders.map(folder => <li key={folder} className="flex items-center justify-between gap-2">
            <span className="min-w-0 truncate text-sm" title={folder}>{folder}</span>
            <Button type="button" variant="ghost" size="icon-sm" aria-label={`移除文件夹 ${folder}`} disabled={busy}
              onClick={() => setFolders(current => current.filter(value => value !== folder))}><X aria-hidden="true" /></Button>
          </li>)}
        </ul>}
        {picker.isError && <FieldError>无法选择文件夹，请重试。</FieldError>}
        {save.isError && <FieldError>{groupActionError(save.error)}</FieldError>}
        <DialogFooter>
          <Button type="button" variant="outline" disabled={busy} onClick={onClose}>取消</Button>
          <Button type="submit" disabled={busy || !name.trim()}>{group ? '保存' : '创建分组'}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
}
