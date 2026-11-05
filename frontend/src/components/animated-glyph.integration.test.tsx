import { act, fireEvent, render, screen } from '@testing-library/react'
import { StrictMode } from 'react'
import { MotionGlobalConfig, processFrame } from 'motion'
import { MotionConfig } from 'motion/react'
import { afterEach, expect, it } from 'vitest'
import { AnimatedGlyph, IconMotionContext } from './animated-glyph'
import { animatedIcons } from './icon-catalog'

afterEach(() => { MotionGlobalConfig.useManualTiming = false })

it('StrictMode 首次悬停按需挂载的全部真实图标确实运动、归位并可重播', async () => {
  MotionGlobalConfig.useManualTiming = true
  processFrame(0)
  const names = Object.keys(animatedIcons) as (keyof typeof animatedIcons)[]
  const { container } = render(<StrictMode><IconMotionContext.Provider value="full"><MotionConfig reducedMotion="never">
    {names.map((name, index) => index % 3 === 0
      ? <button key={name} data-testid={name}><AnimatedGlyph meaning={name}/>{name}</button>
      : index % 3 === 1
        ? <div key={name} role="menuitem" tabIndex={-1} data-testid={name}><AnimatedGlyph meaning={name}/>{name}</div>
        : <AnimatedGlyph key={name} data-testid={name} meaning={name}/>)}
  </MotionConfig></IconMotionContext.Provider></StrictMode>)
  await act(async () => {
    for (const name of names) fireEvent.mouseEnter(screen.getByTestId(name))
  })
  const frames = new Map(names.map(name => [name, new Set<string>()]))
  for (let step = 1; step <= 750; step++) {
    await act(async () => { processFrame(step * 16); await Promise.resolve() })
    if (step % 4 !== 0) continue
    for (const name of names) {
      const target = screen.getByTestId(name)
      const active = target.matches('[data-animating=true]') || target.querySelector('[data-animating=true]')
      if (active) frames.get(name)!.add(target.querySelector('svg')!.outerHTML)
    }
  }
  expect(names.filter(name => frames.get(name)!.size < 3), '实际 SVG 必须变化，不能只断言 busy 或 Promise').toEqual([])
  expect(container.querySelector('[data-animating=true]')).not.toBeInTheDocument()
  expect(container.querySelectorAll('.icon-still')).toHaveLength(names.length)

  // 指针没有移出，自动归位已完成；再次进入要启动新一轮。
  const settings = screen.getByTestId('settings')
  fireEvent.mouseLeave(settings)
  await act(async () => { fireEvent.mouseEnter(settings) })
  await act(async () => { processFrame(12200); await Promise.resolve() })
  expect(settings.querySelector('svg')?.style.transform).toContain('rotate(')
  for (let step = 1; step <= 250; step++) {
    await act(async () => { processFrame(12200 + step * 16); await Promise.resolve() })
  }
  expect(settings.querySelector('.icon-still')).toBeInTheDocument()
}, 15000)
