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
const compactNumber = new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 })
const price = new Intl.NumberFormat(undefined, { style: 'currency', currency: 'USD', maximumFractionDigits: 6 })

export function ModelCapabilities({ metadata }: { metadata?: Metadata }) {
  return <>
    {metadata?.modalities?.map(input => {
      const [Icon, label] = modalities[input as keyof typeof modalities] ?? [FileText, input]
      return <span key={input} role="img" title={label} aria-label={label}><Icon className="size-3.5" /></span>
    })}
    {metadata?.context ? <span className="whitespace-nowrap">{compactNumber.format(metadata.context)} 上下文</span> : null}
  </>
}

export function ModelDetails({ metadata }: { metadata?: Metadata }) {
  if (!metadata) return null
  const inputPrice = metadata.promptPrice
  const outputPrice = metadata.completionPrice
  const hasPrices = inputPrice != null || outputPrice != null
  return <dl className="grid grid-cols-[1fr_auto] gap-x-4 gap-y-1.5 text-xs">
    {metadata.context ? <><dt className="text-muted-foreground">上下文窗口</dt><dd className="text-right tabular-nums">{metadata.context.toLocaleString()}</dd></> : null}
    {metadata.outputLimit ? <><dt className="text-muted-foreground">最大输出</dt><dd className="text-right tabular-nums">{metadata.outputLimit.toLocaleString()}</dd></> : null}
    {hasPrices && <>
      <dt className="col-span-2 text-muted-foreground">{metadata.referencePrice ? '目录参考价' : '价格'} / 百万令牌</dt>
      {inputPrice != null && <><dt className="text-muted-foreground">输入</dt><dd className="text-right tabular-nums">{price.format(inputPrice)}</dd></>}
      {outputPrice != null && <><dt className="text-muted-foreground">输出</dt><dd className="text-right tabular-nums">{price.format(outputPrice)}</dd></>}
    </>}
    {metadata.source && <><dt className="text-muted-foreground">来源</dt>
      <dd className="text-right" title={metadata.updatedAt ? new Date(metadata.updatedAt).toLocaleString() : undefined}>{metadata.source}</dd>
    </>}
  </dl>
}
