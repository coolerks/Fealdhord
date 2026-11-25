import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldError, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { SearchableSelect } from '@/components/ui/searchable-select'
import * as api from '../../../bindings/github.com/coolerks/Fealdhord/internal/appagent/api'
import { SettingCard, SettingRow } from './settings-primitives'

const shellSettingsKey = ['ai-platform', 'shell-settings'] as const
const selectId = 'default-shell'

function saveErrorMessage(error: Error, custom: boolean) {
  const message = String(error)
  if (message.includes('InvalidShellPath')) return '请输入 Shell 可执行文件的绝对路径。'
  if (message.includes('ShellUnavailable')) {
    return custom ? '此路径下的 Shell 无法执行，请检查文件是否存在及执行权限。' : '所选 Shell 已不可用，请选择其他 Shell。'
  }
  return '保存默认 Shell 失败，请重试。'
}

export function DefaultShellSetting() {
  const cache = useQueryClient()
  const settings = useQuery({ queryKey: shellSettingsKey, queryFn: () => api.ShellSettings(), retry: false })
  const [draft, setDraft] = useState<string | null>(null)
  const save = useMutation({
    mutationFn: (selection: { id: string } | { path: string }) =>
      'path' in selection ? api.SaveCustomShell(selection.path) : api.SaveDefaultShell(selection.id),
    onSuccess: async data => {
      await cache.cancelQueries({ queryKey: shellSettingsKey })
      cache.setQueryData(shellSettingsKey, data)
      setDraft(null)
    },
    onError: () => { void settings.refetch() },
  })
  const selected = settings.data?.defaultShell
  const customPath = settings.data?.customPath
  const error = save.error && saveErrorMessage(save.error, draft !== null)
  const options = (settings.data?.options ?? []).map(option => {
    const isDefault = option.id === 'auto'
    const label = isDefault ? `应用默认（${option.label}）` : option.label
    return {
      value: option.id,
      label: option.path ? label : `${label} · 不可用`,
      keywords: [option.label, option.path ?? ''],
      description: option.path,
      disabled: !isDefault && !option.path,
    }
  })
  options.push({
    value: 'custom',
    label: selected === 'custom' && !settings.data?.customAvailable ? '自定义路径 · 不可用' : '自定义路径',
    keywords: ['自定义', customPath ?? ''],
    description: customPath,
    disabled: false,
  })

  function closeEditor() {
    if (save.isPending) return
    save.reset()
    setDraft(null)
  }

  return (
    <SettingCard>
      <SettingRow>
        <div className="min-w-0">
          <label htmlFor={selectId} className="text-sm font-medium">默认 Shell</label>
          <p className="text-xs text-muted-foreground">用于全局 AI 的命令工具，保存后从下一轮对话起生效。</p>
          {selected === 'custom' && customPath && <p className="mt-1 break-all font-mono text-xs text-muted-foreground">{customPath}</p>}
        </div>
        {settings.isError ? (
          <Button size="sm" variant="outline" onClick={() => void settings.refetch()}>重试读取 Shell 设置</Button>
        ) : (
          <SearchableSelect id={selectId} label="默认 Shell" className="min-w-40"
            options={options} value={selected ?? ''}
            placeholder="正在读取…" disabled={!settings.data || save.isPending} alwaysShowSearch announceSelection
            onValueChange={id => {
              if (id === 'custom') {
                save.reset()
                setDraft(customPath ?? '')
              } else if (id !== selected) {
                save.mutate({ id })
              }
            }} />
        )}
      </SettingRow>
      {settings.isError && <SettingRow><FieldError>无法读取 Shell 设置。</FieldError></SettingRow>}
      {error && draft === null && <SettingRow><FieldError>{error}</FieldError></SettingRow>}
      <Dialog open={draft !== null} onOpenChange={open => { if (!open) closeEditor() }}>
        {draft !== null && (
          <DialogContent className="sm:max-w-md" showCloseButton={!save.isPending} onCloseAutoFocus={event => {
            const select = document.getElementById(selectId)
            if (select) {
              event.preventDefault()
              select.focus()
            }
          }}>
            <DialogHeader>
              <DialogTitle>自定义 Shell 路径</DialogTitle>
              <DialogDescription>输入可执行文件的绝对路径。也可用 ~ 指代用户目录。</DialogDescription>
            </DialogHeader>
            <form className="grid gap-4" onSubmit={event => {
              event.preventDefault()
              if (draft.trim() && !save.isPending) save.mutate({ path: draft })
            }}>
              <Field data-invalid={Boolean(error)}>
                <FieldLabel htmlFor="custom-shell-path">可执行文件路径</FieldLabel>
                <Input id="custom-shell-path" value={draft} disabled={save.isPending} aria-invalid={Boolean(error)}
                  aria-describedby={error ? 'custom-shell-error' : undefined} spellCheck={false} autoComplete="off"
                  onChange={event => {
                    setDraft(event.target.value)
                    if (save.isError) save.reset()
                  }} />
                {error && <FieldError id="custom-shell-error">{error}</FieldError>}
              </Field>
              <DialogFooter>
                <Button type="button" variant="outline" disabled={save.isPending} onClick={closeEditor}>取消</Button>
                <Button type="submit" disabled={!draft.trim() || save.isPending}>{save.isPending ? '正在保存…' : '保存'}</Button>
              </DialogFooter>
            </form>
          </DialogContent>
        )}
      </Dialog>
    </SettingCard>
  )
}
