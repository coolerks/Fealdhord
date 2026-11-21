import { useQuery } from '@tanstack/react-query'
import { AudioLines, Bot, FileText, Film, Image, TextCursor } from 'lucide-react'
import { BrandIcon } from '../../../bindings/github.com/coolerks/Fealdhord/internal/aiconfig/api'
import type { Metadata } from '../../../bindings/github.com/coolerks/Fealdhord/internal/ai/models'

export function ModelBrand({ brand, override }: { brand?: string; override?: string }) {
  const selected = override === 'none' ? undefined : override || brand
  const icon = useQuery({ queryKey: ['ai-brand', selected], queryFn: () => BrandIcon(selected ?? ''), enabled: Boolean(selected), staleTime: Infinity, retry: false, retryOnMount: false })
  return icon.data ? <img alt="" className="size-4 shrink-0 object-contain dark:brightness-0 dark:invert" src={icon.data} /> : <Bot aria-hidden className="size-4 shrink-0 text-muted-foreground" />
}

const modalities = {
  text: [TextCursor, '文字'], image: [Image, '图片'], audio: [AudioLines, '音频'],
  video: [Film, '视频'], pdf: [FileText, 'PDF'],
} as const
const price = new Intl.NumberFormat(undefined, { style: 'currency', currency: 'USD', maximumFractionDigits: 6 })

export function ModelCapabilities({ metadata }: { metadata?: Metadata }) {
  if (!metadata) return null
  return <span className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
    {metadata.modalities?.map(input => {
      const [Icon, label] = modalities[input as keyof typeof modalities] ?? [FileText, input]
      return <span key={input} role="img" title={label} aria-label={label}><Icon className="size-4" /></span>
    })}
    {metadata.context ? <span>{metadata.context.toLocaleString()} 上下文</span> : null}
    {metadata.source && <span title={metadata.updatedAt ? new Date(metadata.updatedAt).toLocaleString() : undefined}>{metadata.source}</span>}
    {metadata.referencePrice && (metadata.promptPrice != null || metadata.completionPrice != null) && <span>目录参考价</span>}
    {metadata.promptPrice != null && <span>输入 {price.format(metadata.promptPrice)}/百万令牌</span>}
    {metadata.completionPrice != null && <span>输出 {price.format(metadata.completionPrice)}/百万令牌</span>}
  </span>
}
