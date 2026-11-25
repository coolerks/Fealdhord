import { useSyncExternalStore } from 'react'
import type { LucideIcon } from 'lucide-react'
import { AnimatedGlyph } from '@/components/animated-glyph'
import type { IconMeaning } from '@/components/animated-glyph'
import { animatedIcons } from '@/components/icon-catalog'
import type { AnimatedIconName } from '@/components/icon-catalog'
import { cn } from '@/lib/utils'

const animatedAliases: Record<string, AnimatedIconName> = {
  favorite: 'bookmark',
  folder: 'folder-open',
  grid: 'layout-grid',
  ai: 'sparkles',
  collapse: 'panel-left-close',
  expand: 'panel-left-open',
  read: 'check-check',
  book: 'book-text',
  list: 'layers',
  refresh: 'refresh-cw',
  add: 'plus',
  unread: 'circle-dashed',
}

export function resolveAnimatedIcon(value?: string): AnimatedIconName | null {
  if (!value || value.startsWith('lucide:')) return null
  if (Object.hasOwn(animatedAliases, value)) return animatedAliases[value]
  if (Object.hasOwn(animatedIcons, value)) return value as AnimatedIconName
  return null
}

export function parseFeedBuiltinIcon(data?: string): string | undefined {
  return data?.startsWith('icon:') ? data.slice(5) : undefined
}

type LucideRegistry = Record<string, LucideIcon>

let cachedRegistry: LucideRegistry | null = null
let loadPromise: Promise<LucideRegistry> | null = null
const listeners = new Set<() => void>()

export function ensureLucideIcons(): Promise<LucideRegistry> {
  if (cachedRegistry) return Promise.resolve(cachedRegistry)
  if (!loadPromise) {
    loadPromise = import('lucide-react').then(module => {
      cachedRegistry = module.icons as LucideRegistry
      listeners.forEach(listener => listener())
      return cachedRegistry
    })
  }
  return loadPromise
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}

export function useLucideRegistry(enabled: boolean): LucideRegistry | null {
  if (enabled && !cachedRegistry) void ensureLucideIcons()
  return useSyncExternalStore(subscribe, () => cachedRegistry, () => null)
}

export function pascalToKebab(name: string): string {
  return name
    .replace(/([a-z0-9])([A-Z])/g, '$1-$2')
    .replace(/([A-Z])([A-Z][a-z])/g, '$1-$2')
    .toLowerCase()
}

export function CustomIcon({
  value,
  fallback = 'folder',
  className,
}: {
  value?: string
  fallback?: IconMeaning
  className?: string
}) {
  const isLucide = Boolean(value?.startsWith('lucide:'))
  const registry = useLucideRegistry(isLucide)

  if (isLucide && value) {
    const iconName = value.slice(7)
    const IconComponent = registry?.[iconName]
    if (IconComponent) {
      return <IconComponent aria-hidden="true" className={cn('size-4 shrink-0', className)} strokeWidth={1.7} />
    }
    return <AnimatedGlyph meaning={fallback} className={className} />
  }

  const animated = resolveAnimatedIcon(value)
  return <AnimatedGlyph meaning={animated ?? fallback} className={className} />
}
