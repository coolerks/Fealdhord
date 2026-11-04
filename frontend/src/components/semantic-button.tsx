import type { ComponentProps } from 'react'
import type { LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { AnimatedGlyph } from './animated-glyph'
import { cn } from '@/lib/utils'

type Props = Omit<ComponentProps<typeof Button>, 'children'> & { icon: LucideIcon; label: string; meaning?: string; active?: boolean }
export function SemanticButton({ icon: Icon, label, meaning = 'default', active, className, ...props }: Props) {
  return <Tooltip><TooltipTrigger asChild><span className="icon-hit-area"><Button type="button" variant="ghost" size="icon" aria-label={label} data-active={active || undefined} className={cn('semantic-button', `meaning-${meaning}`, className)} {...props}><AnimatedGlyph icon={Icon} meaning={meaning} /></Button></span></TooltipTrigger><TooltipContent>{label}</TooltipContent></Tooltip>
}
