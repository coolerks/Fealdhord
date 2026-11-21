import { expect, it, vi } from 'vitest'
import { animateWorkspaceEntry } from './workspace-motion'

it('两栏同步使用有效动效，关闭时不播放，快速切换时取消前一次动画', () => {
  const surfaces = [document.createElement('aside'), document.createElement('main')].map(element => {
    const cancel = vi.fn()
    return Object.assign(element, { animate: vi.fn(() => ({ cancel })) })
  })

  animateWorkspaceEntry(surfaces, 'off', 6)
  const cancelFull = animateWorkspaceEntry(surfaces, 'full', 6)
  cancelFull?.()
  animateWorkspaceEntry(surfaces, 'subtle', -6)

  for (const surface of surfaces) {
    expect(surface.animate).toHaveBeenCalledTimes(2)
    expect(surface.animate).toHaveBeenNthCalledWith(1,
      [{ opacity: 0, translate: '0 6px' }, { opacity: 1, translate: '0 0' }],
      expect.objectContaining({ duration: 220 }),
    )
    expect(surface.animate).toHaveBeenNthCalledWith(2,
      [{ opacity: 0 }, { opacity: 1 }],
      expect.objectContaining({ duration: 100 }),
    )
    expect(surface.animate.mock.results[0].value.cancel).toHaveBeenCalledOnce()
  }
})
