import { act, fireEvent, render, screen } from '@testing-library/react'
import { StrictMode, useImperativeHandle } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AnimatedGlyph, IconMotionContext } from './animated-glyph'

const playback = vi.hoisted(() => ({ start: vi.fn(), reset: vi.fn(), cancel: vi.fn() }))
vi.mock('./icon-catalog', () => ({ animatedIcons: {
  settings: function TestIcon({ref}:{ref:React.Ref<unknown>}) {
    useImperativeHandle(ref, () => ({startAnimation:playback.start,stopAnimation:playback.reset,cancelAnimation:playback.cancel}))
    return <svg data-testid="moving-icon"/>
  },
} }))
function deferred() { let resolve!:()=>void; const promise=new Promise<void>(done=>{resolve=done}); return {promise,resolve} }
function fixture(mode:'full'|'subtle'|'off'='full', disabled=false) {
 return <StrictMode><IconMotionContext.Provider value={mode}><button disabled={disabled}>设置<AnimatedGlyph meaning="settings"/></button></IconMotionContext.Provider></StrictMode>
}
beforeEach(()=>{vi.clearAllMocks();playback.start.mockResolvedValue(undefined);playback.reset.mockResolvedValue(undefined)})
describe('图标单次播放',()=>{
 it('持续悬停自动归位，移出不截断，重复进入不排队',async()=>{
  const play=deferred(),reset=deferred()
  playback.start.mockReturnValue(play.promise);playback.reset.mockReturnValue(reset.promise)
  const {container}=render(fixture())
  const button=screen.getByRole('button',{name:'设置'})
  const initial=container.querySelector('.icon-still')?.innerHTML
  fireEvent.mouseEnter(button)
  expect(playback.start).toHaveBeenCalledTimes(1)
  fireEvent.mouseLeave(button);fireEvent.mouseEnter(button);fireEvent.focusIn(button);fireEvent.pointerDown(button)
  expect(playback.start).toHaveBeenCalledTimes(1)
  expect(playback.reset).not.toHaveBeenCalled()
  await act(async()=>play.resolve())
  expect(playback.reset).toHaveBeenCalledTimes(1)
  expect(screen.getByTestId('moving-icon')).toBeInTheDocument()
  await act(async()=>reset.resolve())
  expect(screen.queryByTestId('moving-icon')).not.toBeInTheDocument()
  expect(container.querySelector('.icon-still')?.innerHTML).toBe(initial)
  expect(container.querySelector('.animated-glyph')).toHaveAttribute('data-animating','false')
  fireEvent.mouseLeave(button);fireEvent.mouseEnter(button)
  await act(async()=>{})
  expect(playback.start).toHaveBeenCalledTimes(2)
 })
 it('菜单整行和键盘聚焦触发；偏好切换后旧回调不能重播',async()=>{
  const play=deferred();playback.start.mockReturnValue(play.promise)
  const content=(mode:'full'|'off')=><IconMotionContext.Provider value={mode}><div role="menuitem" tabIndex={-1}>设置<AnimatedGlyph meaning="settings"/></div></IconMotionContext.Provider>
  const {rerender}=render(content('full'))
  fireEvent.focusIn(screen.getByRole('menuitem'))
  expect(playback.start).toHaveBeenCalledTimes(1)
  rerender(content('off'))
  expect(playback.cancel).toHaveBeenCalledTimes(1)
  await act(async()=>play.resolve())
  expect(playback.reset).not.toHaveBeenCalled()
  fireEvent.mouseEnter(screen.getByRole('menuitem'))
  expect(playback.start).toHaveBeenCalledTimes(1)
 })
 it.each(['subtle','off'] as const)('%s 和禁用项保持静态',mode=>{
  const {rerender}=render(fixture(mode))
  fireEvent.mouseEnter(screen.getByRole('button'));fireEvent.focusIn(screen.getByRole('button'))
  rerender(fixture('full',true))
  fireEvent.mouseEnter(screen.getByRole('button'))
  expect(playback.start).not.toHaveBeenCalled()
 })
 it('被动图标自身 hover，卸载取消播放，不新增按钮角色',async()=>{
  const play=deferred();playback.start.mockReturnValue(play.promise)
  const {container,unmount}=render(<IconMotionContext.Provider value="full"><AnimatedGlyph meaning="settings"/></IconMotionContext.Provider>)
  fireEvent.mouseEnter(container.querySelector('.animated-glyph')!)
  expect(playback.start).toHaveBeenCalledTimes(1)
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
  unmount()
  expect(playback.cancel).toHaveBeenCalledTimes(1)
  await act(async()=>play.resolve())
  expect(playback.reset).not.toHaveBeenCalled()
 })
})

it('命令选项的键盘选择变化触发一次动画，失败会恢复静态',async()=>{
 playback.start.mockRejectedValueOnce(new Error('motion failure'))
 const {container}=render(<IconMotionContext.Provider value="full"><div role="option" data-selected="false"><AnimatedGlyph meaning="settings"/></div></IconMotionContext.Provider>)
 const option=screen.getByRole('option')
 await act(async()=>option.setAttribute('data-selected','true'))
 expect(playback.start).toHaveBeenCalledTimes(1)
 expect(container.querySelector('[data-animating=true]')).not.toBeInTheDocument()
 expect(container.querySelector('.icon-still')).toBeInTheDocument()
})
