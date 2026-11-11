// 图形来自 Lucide（ISC），https://lucide.dev/icons/ellipsis
import { motion, useAnimation } from 'motion/react'
import { useImperativeHandle } from 'react'
import type { Ref } from 'react'
import type { IconHandle } from '@/components/animated-glyph'

export function EllipsisIcon({ ref, size = 20 }: { ref?: Ref<IconHandle>; size?: number }) {
  const controls = useAnimation()
  useImperativeHandle(ref, () => ({
    cancelAnimation: () => controls.stop(),
    startAnimation: () => controls.start('animate'),
    stopAnimation: () => controls.start('normal'),
  }))
  return <svg aria-hidden="true" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
    {[5, 12, 19].map((x, index) => (
      <motion.circle key={x} cx={x} cy={12} r={1} custom={index} animate={controls} variants={{
        normal: { y: 0 },
        animate: (dot: number) => ({ y: [0, -2.5, 0], transition: { duration: .24, delay: dot * .045 } }),
      }} />
    ))}
  </svg>
}
