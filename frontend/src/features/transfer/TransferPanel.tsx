import { Download, Upload } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { FieldError } from '@/components/ui/field'
import { SettingCard, SettingRow, SettingSection, StatusBadge } from '@/features/settings/settings-primitives'
import { notify } from '@/lib/notifications'
import { invalidateRSS } from '@/lib/refresh'
import { transferAPI } from '@/lib/transfer-api'
import type { PreviewData } from '@/lib/transfer-api'
import { useWorkspace } from '@/stores/workspace'
import { selectOutline } from './selection'

const pageSize = 200
const statuses: Record<string, { label: string; tone: 'active' | 'muted' | 'warning' | 'error' }> = {
  new: { label: '新增', tone: 'active' },
  existing: { label: '已存在', tone: 'muted' },
  duplicate: { label: '重复源', tone: 'warning' },
  restore: { label: '恢复', tone: 'active' },
  invalid: { label: '无法导入', tone: 'error' },
}

export function TransferPanel() {
  const client = useQueryClient()
  const intent = useWorkspace(state => state.transferIntent)
  const [preview, setPreview] = useState<PreviewData | null>(null)
  const [selected, setSelected] = useState(new Set<string>())
  const [page, setPage] = useState(0)
  const [applyExisting, setApplyExisting] = useState(true)
  const [refresh, setRefresh] = useState(true)
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const active = useRef<{ controller: AbortController; token: string; previewID: string; kind: string } | null>(null)
  const alive = useRef(false)
  const previewID = useRef('')

  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
      active.current?.controller.abort()
      void transferAPI.discard(active.current?.token, previewID.current || active.current?.previewID).catch(() => {})
      active.current = null
    }
  }, [])

  const clearPreview = useCallback(() => {
    if (previewID.current) void transferAPI.discard('', previewID.current).catch(() => {})
    previewID.current = ''
    setPreview(null)
    setSelected(new Set())
    setPage(0)
  }, [])

  const choose = useCallback(async (kind: 'import' | 'export') => {
    if (active.current) return
    const operation = { controller: new AbortController(), token: '', previewID: '', kind }
    active.current = operation
    setBusy(kind === 'import' ? '正在读取预览…' : '正在导出…')
    setError('')
    clearPreview()
    const current = () => alive.current && active.current === operation && !operation.controller.signal.aborted
    try {
      const grant = await transferAPI.choose(kind)
      operation.token = grant.token
      if (!current()) { void transferAPI.discard(grant.token).catch(() => {}); return }
      if (grant.cancelled) return
      if (kind === 'import') {
        const result = await transferAPI.preview(grant.token, operation.controller.signal)
        if (!current()) { void transferAPI.discard(grant.token, result.preview_id).catch(() => {}); return }
        operation.previewID = result.preview_id
        previewID.current = result.preview_id
        setPreview(result)
        setSelected(new Set(result.items.filter(item => item.status !== 'invalid').map(item => item.key)))
      } else {
        const result = await transferAPI.export(grant.token, operation.controller.signal)
        if (current()) void notify('OPML 导出完成', `${result.name}（${result.bytes} 字节）`)
      }
    } catch (cause) {
      if (current()) setError(cause instanceof Error ? cause.message : '文件处理失败，请重新选择。')
    } finally {
      if (!operation.previewID) void transferAPI.discard(operation.token).catch(() => {})
      if (active.current === operation) {
        active.current = null
        if (alive.current) setBusy('')
      }
    }
  }, [clearPreview])

  async function confirm() {
    if (active.current || !preview || selected.size === 0) return
    const operation = { controller: new AbortController(), token: '', previewID: preview.preview_id, kind: 'confirm' }
    active.current = operation
    setBusy('正在保存导入…')
    setError('')
    try {
      const result = await transferAPI.import({
        preview_id: preview.preview_id,
        selected: [...selected],
        apply_existing: applyExisting,
        refresh,
      }, operation.controller.signal)
      invalidateRSS(client)
      if (alive.current && active.current === operation) {
        const report = result.report
        void notify(
          'OPML 导入完成',
          `已新增 ${report.feeds_added} 个订阅，复用 ${report.feeds_reused} 个，恢复 ${report.feeds_restored} 个；新增 ${report.categories_added} 个分类、${report.associations_added} 个关联。${result.refresh_queued ? '已排入刷新队列。' : ''}${result.refresh_error ?? ''}`,
          Boolean(result.refresh_error),
        )
      }
    } catch (cause) {
      invalidateRSS(client)
      if (alive.current && active.current === operation) {
        setError(cause instanceof Error ? cause.message : '导入失败，请重新预览。')
      }
    } finally {
      void transferAPI.discard('', operation.previewID).catch(() => {})
      if (active.current === operation) {
        active.current = null
        if (alive.current) {
          setBusy('')
          clearPreview()
        }
      }
    }
  }

  function cancel() {
    const operation = active.current
    operation?.controller.abort()
    active.current = null
    void transferAPI.discard(operation?.token, previewID.current || operation?.previewID).catch(() => {})
    clearPreview()
    setBusy('')
    if (operation?.kind === 'confirm') {
      invalidateRSS(client)
      void notify('导入请求已取消', '请检查订阅列表后重新预览。')
    } else if (operation?.kind === 'export') {
      void notify('导出请求已取消', '请检查目标文件。')
    }
  }

  useEffect(() => {
    let current = true
    queueMicrotask(() => {
      if (!current || !intent || useWorkspace.getState().transferIntent !== intent) return
      useWorkspace.setState({ transferIntent: null })
      void choose(intent)
    })
    return () => { current = false }
  }, [intent, choose])

  const total = preview?.items.length ?? 0
  const pages = Math.max(1, Math.ceil(total / pageSize))

  return <SettingSection title="OPML 导入与导出">
    <SettingCard>
      <SettingRow>
        <span className="text-[13px]">导入或导出订阅与分类结构</span>
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" variant="outline" disabled={!!busy} onClick={() => { void choose('import') }}>
            <Upload />导入 OPML
          </Button>
          <Button size="sm" variant="outline" disabled={!!busy} onClick={() => { void choose('export') }}>
            <Download />导出 OPML
          </Button>
          {(busy || preview) && <Button size="sm" variant="ghost" onClick={cancel}>取消</Button>}
        </div>
      </SettingRow>
      {busy && <p role="status" className="px-3.5 py-2.5 text-xs text-muted-foreground">{busy}</p>}
      {error && <div className="px-3.5 py-2.5"><FieldError>{error}</FieldError></div>}
    </SettingCard>

    {preview && (
      <section aria-label="OPML 导入预览" className="mt-3">
        <SettingCard>
          <SettingRow className="bg-muted/25">
            <div className="flex min-w-0 items-center gap-2">
              <h3 className="truncate text-[13px] font-medium">{preview.name}</h3>
              <span className="shrink-0 text-xs text-muted-foreground tabular-nums">已选择 {selected.size} 项</span>
            </div>
            <div className="flex items-center gap-1">
              <Button size="xs" variant="ghost" disabled={!!busy}
                onClick={() => setSelected(new Set(preview.items.filter(item => item.status !== 'invalid').map(item => item.key)))}>
                全选
              </Button>
              <Button size="xs" variant="ghost" disabled={!!busy} onClick={() => setSelected(new Set())}>全不选</Button>
            </div>
          </SettingRow>
          <ul className="max-h-[42vh] divide-y divide-border overflow-y-auto">
            {preview.items.slice(page * pageSize, (page + 1) * pageSize).map(item => {
              const badge = statuses[item.status] ?? { label: item.status, tone: 'muted' as const }
              return (
                <li key={item.key} className="px-3.5 py-2" style={{ paddingInlineStart: `${14 + Math.min(item.depth, 5) * 14}px` }}>
                  <label className="flex items-center gap-2.5 text-[13px]" data-animated-trigger aria-disabled={!!busy || item.status === 'invalid'}>
                    <Checkbox
                      disabled={!!busy || item.status === 'invalid'}
                      checked={selected.has(item.key)}
                      onCheckedChange={value => setSelected(previous => selectOutline(preview.items, previous, item.key, value === true))}
                    />
                    <AnimatedGlyph meaning={item.kind === 'folder' ? 'folder' : 'radio-tower'} className="size-4 text-muted-foreground [&_svg]:size-4" />
                    <span className="min-w-0 flex-1 [overflow-wrap:anywhere]">{item.title || '无标题'}</span>
                    <StatusBadge tone={badge.tone}>{badge.label}</StatusBadge>
                  </label>
                  {item.reason && <p className="mt-1 pl-6 text-xs text-muted-foreground">{item.reason}</p>}
                </li>
              )
            })}
          </ul>
          {pages > 1 && (
            <SettingRow className="py-2 text-xs text-muted-foreground">
              <span className="tabular-nums">{page * pageSize + 1}–{Math.min(total, (page + 1) * pageSize)} / {total}</span>
              <div className="flex items-center gap-1.5">
                <Button size="xs" variant="outline" disabled={page === 0} onClick={() => setPage(value => value - 1)}>上一页</Button>
                <Button size="xs" variant="outline" disabled={page + 1 >= pages} onClick={() => setPage(value => value + 1)}>下一页</Button>
              </div>
            </SettingRow>
          )}
          <div className="flex flex-wrap items-center justify-between gap-3 bg-muted/25 px-3.5 py-3">
            <div className="flex flex-wrap items-center gap-4">
              <label className="flex items-center gap-2 text-xs">
                <Checkbox disabled={!!busy} checked={applyExisting} onCheckedChange={value => setApplyExisting(value === true)} />
                分类关联应用于已有文章
              </label>
              <label className="flex items-center gap-2 text-xs">
                <Checkbox disabled={!!busy} checked={refresh} onCheckedChange={value => setRefresh(value === true)} />
                导入后刷新订阅
              </label>
            </div>
            <Button size="sm" disabled={!!busy || selected.size === 0} onClick={() => { void confirm() }}>确认导入</Button>
          </div>
        </SettingCard>
      </section>
    )}
  </SettingSection>
}
