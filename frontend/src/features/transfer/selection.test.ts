import { expect,it } from 'vitest'
import { selectOutline } from './selection'
import type { ImportOutline } from '@/lib/transfer-api'
it('子项选择包含祖先，取消文件夹清空后代，非法项不可加入',()=>{
 const items=[['a',''],['b','a'],['c','b'],['d','b']].map(([key,parent_key])=>({key,parent_key,status:key==='d'?'invalid':'new'})) as ImportOutline[]
 expect([...selectOutline(items,new Set(),'c',true)]).toEqual(['c','b','a'])
 expect([...selectOutline(items,new Set(['a','b','c']),'b',false)]).toEqual(['a'])
 expect([...selectOutline(items,new Set(),'a',true)]).toEqual(['a','b','c'])
})
