import { useQuery } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Field, FieldLabel } from '@/components/ui/field'
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { FetchModels } from '../../../bindings/github.com/coolerks/Fealdhord/internal/aiconfig/api'
import type { Provider } from '../../../bindings/github.com/coolerks/Fealdhord/internal/aiconfig/models'

export function ModelCatalogField({ provider, value, excluded, disabled, onSelect }: {
  provider: Provider
  value: string
  excluded: string[]
  disabled: boolean
  onSelect: (id: string, name: string) => void
}) {
  const catalog = useQuery({
    queryKey: ['ai-model-catalog', provider.id, provider.revision],
    queryFn: async ({ signal }) => {
      const request = FetchModels(provider.id)
      const cancel = () => { void request.cancel() }
      signal.addEventListener('abort', cancel, { once: true })
      if (signal.aborted) cancel()
      try {
        return await request
      } finally {
        signal.removeEventListener('abort', cancel)
      }
    },
    retry: false,
  })
  const items = (catalog.data ?? []).filter(item => !excluded.includes(item.id))
  return <Field>
    <FieldLabel htmlFor="ai-model-catalog">服务模型</FieldLabel>
    <div className="flex flex-wrap gap-2">
      <Select value={items.some(item => item.id === value) ? value : ''} disabled={disabled || !items.length}
        onValueChange={id => {
          const item = items.find(entry => entry.id === id)
          if (item) onSelect(item.id, item.displayName)
        }}>
        <SelectTrigger id="ai-model-catalog" className="min-w-0 max-w-full">
          <SelectValue placeholder={catalog.isFetching ? '正在获取模型…' : '从服务选择模型'} />
        </SelectTrigger>
        <SelectContent position="popper"><SelectGroup>
          {items.map(item => <SelectItem key={item.id} value={item.id}>
            {item.displayName && item.displayName !== item.id ? `${item.displayName} · ${item.id}` : item.id}
          </SelectItem>)}
        </SelectGroup></SelectContent>
      </Select>
      <Button type="button" size="sm" variant="outline" disabled={disabled || catalog.isFetching} onClick={() => void catalog.refetch()}>刷新列表</Button>
    </div>
    {catalog.isError && <p role="status" className="text-muted-foreground">
      {String(catalog.error).includes('AuthenticationError') ? '获取模型失败，请检查服务密钥。' : '无法获取服务模型列表，可以手动填写。'}
    </p>}
    {catalog.isSuccess && !catalog.isFetching && !items.length &&
      <p role="status" className="text-muted-foreground">没有可添加的模型，可以手动填写。</p>}
  </Field>
}
