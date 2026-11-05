import { act, fireEvent, render, screen } from '@testing-library/react'
import { StrictMode } from 'react'
import { afterEach, expect, it } from 'vitest'
import { MotionGlobalConfig, processFrame } from 'motion'
import { AnimatedGlyph, IconMotionContext } from './animated-glyph'
import { Command, CommandGroup, CommandInput, CommandItem, CommandList } from './ui/command'

afterEach(() => { MotionGlobalConfig.useManualTiming = false })
function setup() {
  MotionGlobalConfig.useManualTiming = true
  processFrame(0)
  render(<StrictMode><IconMotionContext.Provider value="full"><Command label="搜索命令" defaultValue="first">
    <CommandInput/><CommandList><CommandGroup>
      <CommandItem value="first">起始命令</CommandItem>
      <CommandItem value="settings"><AnimatedGlyph meaning="settings"/>设置命令</CommandItem>
      <CommandItem value="blocked" disabled><AnimatedGlyph meaning="settings"/>禁用命令</CommandItem>
    </CommandGroup></CommandList>
  </Command></IconMotionContext.Provider></StrictMode>)
  let time = 0
  return async (duration:number) => {
    const end = time + duration
    while (time < end) {
      time += 16
      await act(async () => { processFrame(time); await Promise.resolve() })
    }
  }
}
it('真实 cmdk 可用项 data-disabled=false：整行悬停能运动，自动归位并重播', async () => {
  const advance = setup()
  const option = screen.getByRole('option', {name:'设置命令'})
  expect(option).toHaveAttribute('data-disabled','false')
  await act(async () => { fireEvent.mouseEnter(option); fireEvent.pointerMove(option) })
  await advance(400)
  expect(option.querySelector('svg')?.style.transform).toContain('rotate(')
  await advance(4000)
  expect(option.querySelector('[data-animating=true]')).not.toBeInTheDocument()
  await act(async () => { fireEvent.mouseLeave(option); fireEvent.mouseEnter(option) })
  await advance(400)
  expect(option.querySelector('svg')?.style.transform).toContain('rotate(')
})
it('真实 cmdk 键盘选择也播放，真正禁用的命令保持静态', async () => {
  const advance = setup()
  const input = screen.getByRole('combobox', {name:'搜索命令'})
  await act(async () => { input.focus(); fireEvent.keyDown(input, {key:'ArrowDown'}) })
  const option = screen.getByRole('option', {name:'设置命令'})
  expect(option).toHaveAttribute('aria-selected','true')
  await advance(400)
  expect(option.querySelector('svg')?.style.transform).toContain('rotate(')
  const disabled = screen.getByRole('option', {name:'禁用命令'})
  expect(disabled).toHaveAttribute('data-disabled','true')
  await act(async () => { fireEvent.mouseEnter(disabled); fireEvent.pointerMove(disabled) })
  await advance(400)
  expect(disabled.querySelector('[data-animating=true]')).not.toBeInTheDocument()
  expect(disabled.querySelector('svg')?.style.transform).not.toContain('rotate(')
})
