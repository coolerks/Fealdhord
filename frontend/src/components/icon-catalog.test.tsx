import { act, render } from '@testing-library/react'
import { createRef } from 'react'
import { afterEach, expect, it } from 'vitest'
import { MotionGlobalConfig, processFrame } from 'motion'
import { animatedIcons } from './icon-catalog'
import type { IconHandle } from './animated-glyph'

afterEach(()=>{MotionGlobalConfig.useManualTiming=false})
it('所有库动画能完成播放与归位，多段动画不会提前报告完成',async()=>{
 MotionGlobalConfig.useManualTiming=true
 processFrame(0)
 const entries=Object.entries(animatedIcons).map(([name,Icon])=>({name,Icon,ref:createRef<IconHandle>()}))
 render(<>{entries.map(({name,Icon,ref})=><Icon key={name} ref={ref}/>)}</>)
 const completed=new Set<string>()
 const failures:string[]=[]
 await act(async()=>{
  for(const {name,ref} of entries) {
   void ref.current!.startAnimation().then(()=>ref.current!.stopAnimation()).then(()=>{completed.add(name)}).catch(error=>{failures.push(`${name}: ${String(error)}`)})
  }
  for(let step=1;step<=1200;step++){processFrame(step*16);await Promise.resolve()}
 })
 expect(failures).toEqual([])
 expect(entries.filter(entry=>!completed.has(entry.name)).map(entry=>entry.name)).toEqual([])
},15000)
