import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { FieldError, FieldLabel } from '@/components/ui/field'
import { SegmentedControl } from '@/components/ui/segmented-control'
import { Switch } from '@/components/ui/switch'
import { ToggleGroupItem } from '@/components/ui/toggle-group'
import { notify } from '@/lib/notifications'
import { rssAPI } from '@/lib/rss-api'
import { useWorkspace } from '@/stores/workspace'
import { SettingCard, SettingRow, SettingSection } from './settings-primitives'

const cacheSizeQueryKey = ['media', 'cache-size'] as const

function formatCacheSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  return unit === 0 ? `${Math.round(value)} B` : `${value >= 100 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`
}

export function MediaPreferences() {
  const client = useQueryClient()
  const preferences = useWorkspace(state => state.preferences)!
  const patch = useWorkspace(state => state.patch)
  const cacheSize = useQuery({
    queryKey: cacheSizeQueryKey,
    queryFn: rssAPI.cacheSize,
    staleTime: 0,
    refetchOnMount: 'always',
    retry: false,
  })
  const clearCache = useMutation({
    mutationFn: async () => {
      await rssAPI.clearCache()
      void notify('图片缓存已清理')
    },
    onSuccess: () => {
      client.setQueryData(cacheSizeQueryKey, 0)
      void client.invalidateQueries({ queryKey: cacheSizeQueryKey })
    },
  })

  return (
    <SettingSection title="图片与缓存">
      <SettingCard>
        <SettingRow>
          <FieldLabel id="remote-images-label" className="font-normal">远程图片</FieldLabel>
          <SegmentedControl
            aria-labelledby="remote-images-label"
            value={preferences.remote_images}
            onValueChange={value => { if (value) patch({ remote_images: value }) }}
          >
            {([['always', '总是允许', 'check'], ['subscriptions', '仅活动订阅', 'radio-tower'], ['never', '从不加载', 'x']] as const).map(([value, label, icon]) => (
              <ToggleGroupItem key={value} value={value}>
                <AnimatedGlyph meaning={icon} />{label}
              </ToggleGroupItem>
            ))}
          </SegmentedControl>
        </SettingRow>
        <SettingRow>
          <FieldLabel htmlFor="web-image-fallback" className="font-normal">从原文页面补充首图</FieldLabel>
          <Switch
            id="web-image-fallback"
            checked={preferences.web_image_fallback}
            onCheckedChange={value => patch({ web_image_fallback: value })}
          />
        </SettingRow>
        <SettingRow>
          <FieldLabel htmlFor="media-cache" className="font-normal">磁盘图片缓存</FieldLabel>
          <Switch
            id="media-cache"
            checked={preferences.media_cache_enabled}
            onCheckedChange={value => {
              patch({ media_cache_enabled: value })
              if (!value) {
                client.setQueryData(cacheSizeQueryKey, 0)
                void client.invalidateQueries({ queryKey: cacheSizeQueryKey })
              }
            }}
          />
        </SettingRow>
        <SettingRow>
          <div className="flex items-baseline gap-2">
            <span className="text-sm">已缓存图片</span>
            <span className="text-xs tabular-nums text-muted-foreground" role="status" aria-live="polite">
              {cacheSize.isFetching ? '计算中' : cacheSize.isError ? '计算失败' : formatCacheSize(cacheSize.data ?? 0)}
            </span>
          </div>
          <Button
            size="sm"
            variant="outline"
            disabled={clearCache.isPending}
            onClick={() => clearCache.mutate()}
          >
            {clearCache.isPending ? '正在清理…' : '清理图片缓存'}
          </Button>
        </SettingRow>
      </SettingCard>
      {clearCache.error && <FieldError>{clearCache.error instanceof Error ? clearCache.error.message : '清理图片缓存失败'}</FieldError>}
    </SettingSection>
  )
}
