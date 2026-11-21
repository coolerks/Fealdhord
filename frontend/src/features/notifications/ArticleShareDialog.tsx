import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { FieldError, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Selection, toggle } from './notification-fields'
import type { Article } from '@/lib/library'
import { notify } from '@/lib/notifications'
import { ListChannels, SendArticle } from '../../../bindings/github.com/coolerks/Fealdhord/internal/notification/api'

const contentFields = [
  ['title', '标题'],
  ['summary', '摘要'],
  ['link', '链接'],
  ['full_text', '全文'],
] as const

type Props = {
  articles: Article[]
  batch: boolean
  defaultCount: number
  summaryLimit: number
  onClose: () => void
  onCloseAutoFocus: (event: Event) => void
}

export function ArticleShareDialog({ articles, batch, defaultCount, summaryLimit, onClose, onCloseAutoFocus }: Props) {
  const client = useQueryClient()
  const channels = useQuery({ queryKey: ['notifications', 'channels'], queryFn: () => ListChannels(), retry: false })
  const [destinations, setDestinations] = useState<string[]>([])
  const [content, setContent] = useState({ title: true, summary: true, link: true, full_text: false })
  const [count, setCount] = useState(String(batch ? Math.min(defaultCount, articles.length) : 1))
  const articleCount = Number(count)
  const validCount = Number.isInteger(articleCount) && articleCount > 0 && articleCount <= articles.length
  const options = (channels.data ?? []).filter(channel => channel.enabled && channel.verified)
    .map(channel => ({ value: channel.id, label: channel.name }))
  const selected = destinations.filter(id => options.some(option => option.value === id))
  const allSelected = options.length > 0 && selected.length === options.length
  const send = useMutation({
    mutationFn: () => SendArticle({
      ...(batch ? { article_ids: articles.slice(0, articleCount).map(article => article.id) } : { article_id: articles[0].id }),
      channel_ids: selected,
      summary_limit: summaryLimit,
      ...content,
    }),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ['notifications', 'deliveries'] })
      void notify('文章已加入发送队列')
      onClose()
    },
  })
  const busy = send.isPending
  const canSend = validCount && channels.isSuccess && selected.length > 0 && Object.values(content).some(Boolean)

  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose() }}>
    <DialogContent className="sm:max-w-md" onCloseAutoFocus={onCloseAutoFocus} showCloseButton={!busy}>
      <DialogHeader>
        <DialogTitle>发送到</DialogTitle>
        <DialogDescription className="wrap-anywhere">{batch ? '按当前列表顺序发送' : articles[0].title}</DialogDescription>
      </DialogHeader>
      <form className="space-y-4" onSubmit={event => { event.preventDefault(); if (canSend && !busy) send.mutate() }}>
        {batch && <div className="flex items-center justify-between gap-3">
          <FieldLabel htmlFor="article-share-count">发送前几篇</FieldLabel>
          <Input id="article-share-count" type="number" min={1} max={articles.length} step={1}
            value={count} onChange={event => setCount(event.target.value)} disabled={busy} className="w-24" />
        </div>}
        <fieldset className="space-y-2" disabled={busy}>
          <legend className="mb-2 text-sm font-medium">发送内容</legend>
          <div className="flex flex-wrap gap-x-5 gap-y-2">
            {contentFields.map(([field, label]) => (
              <label key={field} className="flex items-center gap-2 text-sm">
                <Checkbox checked={content[field]} onCheckedChange={checked => setContent(previous => ({ ...previous, [field]: checked === true }))} />
                {label}
              </label>
            ))}
          </div>
        </fieldset>
        {channels.isPending ? <p role="status" className="text-sm text-muted-foreground">正在读取通知渠道…</p>
          : channels.isError ? <FieldError>无法读取通知渠道</FieldError>
            : options.length === 0 ? <p className="text-sm text-muted-foreground">请先在通知设置中启用渠道并通过发送测试。</p>
              : <div className="space-y-2">
                <Selection id="article-share-channels" label="通知渠道" options={options} value={selected} disabled={busy}
                  placeholder="选择渠道" onChange={id => setDestinations(previous => toggle(previous, id))} />
                <label className="flex items-center gap-2 text-sm">
                  <Checkbox checked={allSelected ? true : selected.length > 0 ? 'indeterminate' : false} disabled={busy}
                    onCheckedChange={checked => setDestinations(checked ? options.map(option => option.value) : [])} />
                  全部渠道
                </label>
              </div>}
        {send.error && <FieldError>{send.error.message}</FieldError>}
        <DialogFooter>
          <Button type="button" variant="outline" disabled={busy} onClick={onClose}>取消</Button>
          <Button type="submit" disabled={!canSend || busy}>{busy ? '正在准备…' : '发送'}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
}
