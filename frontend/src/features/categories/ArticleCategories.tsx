import type { ComponentProps } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatedGlyph } from '@/components/animated-glyph'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { FieldError } from '@/components/ui/field'
import { SegmentedControl } from '@/components/ui/segmented-control'
import { ToggleGroupItem } from '@/components/ui/toggle-group'
import { SettingCard, SettingEmpty, SettingRow, StatusBadge } from '@/features/settings/settings-primitives'
import { articleQueryKey } from '@/lib/library'
import { navigationAPI, navigationQueryKey } from '@/lib/navigation-api'
import { categoryIcons } from './category-icons'
import { useNavigation } from './use-navigation'

type Props = {
  articleID: string
  open: boolean
  onOpenChange: (open: boolean) => void
  onCloseAutoFocus?: ComponentProps<typeof DialogContent>['onCloseAutoFocus']
}

export function ArticleCategories({ articleID, open, onOpenChange, onCloseAutoFocus }: Props) {
  const client = useQueryClient()
  const navigation = useNavigation()
  const key = ['article-memberships', articleID]
  const query = useQuery({ queryKey: key, queryFn: () => navigationAPI.memberships(articleID), enabled: open, retry: false })
  const mutation = useMutation({
    mutationFn: ({ id, state }: { id: string; state: string }) => navigationAPI.setMembership(articleID, id, state),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: key })
      void client.invalidateQueries({ queryKey: navigationQueryKey })
      void client.invalidateQueries({ queryKey: articleQueryKey })
    },
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent aria-describedby={undefined} onCloseAutoFocus={onCloseAutoFocus} className="flex max-h-[75dvh] flex-col sm:max-w-md">
        <DialogHeader>
          <DialogTitle>文章分类</DialogTitle>
        </DialogHeader>
        {query.isPending && <p role="status" className="text-[13px] text-muted-foreground">正在读取分类…</p>}
        {query.isError && (
          <div className="flex items-center justify-between gap-2">
            <FieldError>分类读取失败</FieldError>
            <Button size="sm" variant="outline" onClick={() => { void query.refetch() }}>重试</Button>
          </div>
        )}
        {query.data && query.data.length > 0 && (
          <div className="min-h-0 flex-1 overflow-y-auto">
            <SettingCard>
              {query.data.map(item => {
                const category = navigation.data?.categories?.find(entry => entry.id === item.category_id)
                const icon = categoryIcons.find(name => name === category?.icon) ?? 'folder'
                const color = category?.color && /^#[a-f0-9]{6}$/i.test(category.color) ? category.color : undefined
                return (
                  <SettingRow key={item.category_id} className="px-3 py-2">
                    <div className="flex min-w-0 items-center gap-2">
                      <span className="inline-flex shrink-0 text-muted-foreground" style={{ color }}>
                        <AnimatedGlyph meaning={icon} className="size-4 [&_svg]:size-4" />
                      </span>
                      <span id={`membership-${item.category_id}`} className="truncate text-[13px] font-medium">
                        {category?.name ?? '分类'}
                      </span>
                      {item.effective && <StatusBadge tone="active">已加入</StatusBadge>}
                    </div>
                    <SegmentedControl
                      disabled={mutation.isPending}
                      aria-labelledby={`membership-${item.category_id}`}
                      value={item.manual}
                      onValueChange={state => { if (state) mutation.mutate({ id: item.category_id, state }) }}
                    >
                      <ToggleGroupItem value="auto"><AnimatedGlyph meaning="refresh" />自动</ToggleGroupItem>
                      <ToggleGroupItem value="included"><AnimatedGlyph meaning="plus" />加入</ToggleGroupItem>
                      <ToggleGroupItem value="excluded"><AnimatedGlyph meaning="x" />排除</ToggleGroupItem>
                    </SegmentedControl>
                  </SettingRow>
                )
              })}
            </SettingCard>
          </div>
        )}
        {query.isSuccess && !query.data.length && <SettingEmpty icon={<AnimatedGlyph meaning="folder-tree" />} title="暂无分类" />}
        {mutation.error && <FieldError>{mutation.error.message}</FieldError>}
      </DialogContent>
    </Dialog>
  )
}
