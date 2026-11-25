import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { create } from 'zustand'
import { ChevronRight, Folder, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { cn } from '@/lib/utils'
import { CreateGroup, PickGroupFolders } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/api'
import type { Group } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'

export const groupKey = ['chat', 'groups'] as const
export const groupHistoryKey = ['chat', 'group-conversations'] as const

export const useConversationFolds = create<{
  closed: Record<string, boolean>
  toggle: (key: string) => void
}>(set => ({
  closed: {},
  toggle: key => set(state => ({ closed: { ...state.closed, [key]: !state.closed[key] } })),
}))

export function GroupDisclosure({ label, open, controls, onToggle, folder = false }: {
  label: string
  open: boolean
  controls: string
  onToggle: () => void
  folder?: boolean
}) {
  return <Button type="button" variant="ghost" size="sm" data-navigation={controls}
    className="min-w-0 justify-start px-1.5" aria-expanded={open} aria-controls={controls} onClick={onToggle}>
    <ChevronRight className={cn('size-3 shrink-0', open && 'rotate-90')} aria-hidden="true" />
    {folder && <Folder className="size-4 shrink-0" aria-hidden="true" />}
    <span className="truncate" title={label}>{label}</span>
  </Button>
}

export function NewConversationGroup({ onClose, onCreated }: { onClose: () => void; onCreated: (group: Group) => void }) {
  const [name, setName] = useState('')
  const [folders, setFolders] = useState<string[]>([])
  const picker = useMutation({
    mutationFn: () => PickGroupFolders(),
    onSuccess: selected => setFolders(current => [...new Set([...current, ...(selected ?? [])])]),
  })
  const save = useMutation({
    mutationFn: () => CreateGroup(name.trim(), folders),
    onSuccess: onCreated,
  })
  const busy = picker.isPending || save.isPending
  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose() }}>
    <DialogContent aria-describedby={undefined}>
      <DialogHeader><DialogTitle>新建会话分组</DialogTitle></DialogHeader>
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
        {save.isError && <FieldError>{save.error.message === 'ChatFolderUnavailable' ? '所选文件夹已不可用。' : '无法保存分组，请检查名称与文件夹。'}</FieldError>}
        <DialogFooter>
          <Button type="button" variant="outline" disabled={busy} onClick={onClose}>取消</Button>
          <Button type="submit" disabled={busy || !name.trim()}>创建分组</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
}
