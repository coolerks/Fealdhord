import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { FieldError } from '@/components/ui/field'
import { ConversationConfirmation, archivedConversationsKey, conversationActionError, globalConversationsKey } from '@/features/chat/ConversationMenu'
import { chatMessagesKey } from '@/features/chat/useChatMessages'
import { SettingCard, SettingRow, SettingSection } from './settings-primitives'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/api'
import type { Conversation } from '../../../bindings/github.com/coolerks/Fealdhord/internal/chat/models'

const pageSize = 50

export function ArchivedConversations() {
  const cache = useQueryClient()
  const [offset, setOffset] = useState(0)
  const [deleting, setDeleting] = useState<Conversation | null>(null)
  const archived = useQuery({
    queryKey: [...archivedConversationsKey, offset], queryFn: () => api.Archived(offset), retry: false,
  })
  const items = archived.data?.items ?? []
  const change = useMutation({
    mutationFn: ({ item, remove }: { item: Conversation; remove: boolean }) =>
      remove ? api.Delete(item.id, true) : api.SetArchived(item.id, false),
    onSuccess: async (_result, { item, remove }) => {
      if (remove) cache.removeQueries({ queryKey: chatMessagesKey(item.id) })
      setDeleting(null)
      if (items.length === 1 && offset > 0 && !archived.data?.hasMore) setOffset(offset - pageSize)
      await Promise.all([
        cache.invalidateQueries({ queryKey: archivedConversationsKey }),
        cache.invalidateQueries({ queryKey: globalConversationsKey }),
      ])
    },
  })
  const busy = change.isPending || archived.isFetching
  const error = change.isError ? conversationActionError(change.error) : undefined
  return <SettingSection title="已归档会话" actions={archived.isError && <Button size="sm" variant="outline"
    onClick={() => void archived.refetch()}>重试</Button>}>
    <SettingCard>
      {archived.isPending && <SettingRow><span role="status">正在读取…</span></SettingRow>}
      {archived.isError && <SettingRow><FieldError>无法读取已归档会话。</FieldError></SettingRow>}
      {archived.isSuccess && items.length === 0 && <SettingRow><span className="text-muted-foreground">暂无已归档会话</span></SettingRow>}
      {items.map(item => <SettingRow key={item.id}>
        <span className="min-w-0 flex-1 truncate" title={item.title}>{item.title}</span>
        <div className="flex gap-1">
          <Button size="sm" variant="ghost" aria-label={`恢复 ${item.title}`} disabled={busy}
            onClick={() => change.mutate({ item, remove: false })}>恢复</Button>
          <Button size="sm" variant="ghost" aria-label={`删除 ${item.title}`} disabled={busy}
            onClick={() => { change.reset(); setDeleting(item) }}>删除</Button>
        </div>
      </SettingRow>)}
      {(offset > 0 || archived.data?.hasMore) && <SettingRow>
        <Button size="sm" variant="outline" disabled={busy || offset === 0} onClick={() => setOffset(offset - pageSize)}>上一页</Button>
        <Button size="sm" variant="outline" disabled={busy || !archived.data?.hasMore} onClick={() => setOffset(offset + pageSize)}>下一页</Button>
      </SettingRow>}
    </SettingCard>
    {error && !deleting && <FieldError>{error}</FieldError>}
    <ConversationConfirmation action="delete" item={deleting} busy={change.isPending} error={error}
      onClose={() => { setDeleting(null); change.reset() }}
      onConfirm={() => { if (deleting) change.mutate({ item: deleting, remove: true }) }} />
  </SettingSection>
}
