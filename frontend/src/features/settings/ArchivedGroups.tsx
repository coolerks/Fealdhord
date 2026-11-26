import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { FieldError } from '@/components/ui/field'
import { archivedGroupsKey, GroupIcon, groupActionError, groupHistoryKey, groupKey } from '@/features/chat/ConversationGroups'
import { globalConversationsKey } from '@/features/chat/ConversationMenu'
import { ConfirmDialog, SettingCard, SettingRow, SettingSection } from './settings-primitives'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/api'
import type { Group } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'

export function ArchivedGroups() {
  const cache = useQueryClient()
  const [deleting, setDeleting] = useState<Group | null>(null)
  const archived = useQuery({ queryKey: archivedGroupsKey, queryFn: () => api.ArchivedGroups(), retry: false })
  const change = useMutation({
    mutationFn: ({ id, remove }: { id: string; remove: boolean }) =>
      remove ? api.DeleteGroup(id, true) : api.SetGroupArchived(id, false),
    onSuccess: async () => {
      setDeleting(null)
      await Promise.all([
        cache.invalidateQueries({ queryKey: archivedGroupsKey }),
        cache.invalidateQueries({ queryKey: groupKey }),
        cache.invalidateQueries({ queryKey: groupHistoryKey }),
        cache.invalidateQueries({ queryKey: globalConversationsKey }),
      ])
    },
  })
  const items = archived.data ?? []
  const busy = change.isPending || archived.isFetching
  const error = change.isError ? groupActionError(change.error) : undefined

  return <SettingSection title="已归档分组" actions={archived.isError && <Button size="sm" variant="outline"
    onClick={() => void archived.refetch()}>重试</Button>}>
    <SettingCard>
      {archived.isPending && <SettingRow><span role="status">正在读取…</span></SettingRow>}
      {archived.isError && <SettingRow><FieldError>无法读取已归档分组。</FieldError></SettingRow>}
      {archived.isSuccess && items.length === 0 && <SettingRow><span className="text-muted-foreground">暂无已归档分组</span></SettingRow>}
      {items.map(item => <SettingRow key={item.id}>
        <GroupIcon group={item} />
        <span className="min-w-0 flex-1 truncate" title={item.name}>{item.name}</span>
        <div className="flex gap-1">
          <Button size="sm" variant="ghost" aria-label={`恢复分组 ${item.name}`} disabled={busy}
            onClick={() => change.mutate({ id: item.id, remove: false })}>恢复</Button>
          <Button size="sm" variant="ghost" aria-label={`删除分组 ${item.name}`} disabled={busy}
            onClick={() => { change.reset(); setDeleting(item) }}>删除</Button>
        </div>
      </SettingRow>)}
    </SettingCard>
    {error && !deleting && <FieldError>{error}</FieldError>}
    {deleting && <ConfirmDialog open title={`删除“${deleting.name}”？`}
      description="磁盘目录和会话记录将保留。组内未单独归档的会话将移到普通会话列表。"
      confirmLabel="删除分组" busy={change.isPending} error={error}
      onOpenChange={open => { if (!open) setDeleting(null) }}
      onConfirm={() => change.mutate({ id: deleting.id, remove: true })} />}
  </SettingSection>
}
