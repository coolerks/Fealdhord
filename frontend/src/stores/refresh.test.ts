import { beforeEach, expect, it } from 'vitest'
import { useRefresh } from './refresh'
import type { Snapshot } from '@/lib/rss-api'
const snapshot=(generation:number,sequence:number,status='running'):Snapshot=>({generation,sequence,status,operation_id:`op-${generation}`,completed:0,total:2,results:[]})
beforeEach(()=>useRefresh.setState({snapshot:null,error:null,dismissedGeneration:-1,pending:false}))
it('早到事件不被 Start 响应或前一批刷新覆盖',()=>{
  const accept=useRefresh.getState().accept
  accept(snapshot(2,8));accept(snapshot(2,1));accept(snapshot(1,99))
  expect(useRefresh.getState().snapshot).toEqual(snapshot(2,8))
  accept(snapshot(3,1));expect(useRefresh.getState().snapshot).toEqual(snapshot(3,1))
})
it('活动任务不可关闭，已关闭终态不会被迟到事件重新显示',()=>{
  useRefresh.getState().accept(snapshot(1,1));useRefresh.getState().dismiss()
  expect(useRefresh.getState().snapshot).not.toBeNull()
  useRefresh.getState().accept(snapshot(1,2,'completed'));useRefresh.getState().dismiss()
  useRefresh.getState().accept(snapshot(1,3,'completed'))
  expect(useRefresh.getState().snapshot).toBeNull()
  useRefresh.getState().accept(snapshot(2,1));expect(useRefresh.getState().snapshot?.generation).toBe(2)
})
