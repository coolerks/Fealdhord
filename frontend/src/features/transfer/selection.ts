import type { ImportOutline } from '@/lib/transfer-api'
/** 只维护选择意图；条目及版本仍由 Go 的私有预览拥有。 */
export function selectOutline(items:ImportOutline[],selected:Set<string>,key:string,checked:boolean):Set<string>{
 const next=new Set(selected),byKey=new Map(items.map(item=>[item.key,item]))
 const descendants=new Set([key])
 for(const item of items)if(descendants.has(item.parent_key))descendants.add(item.key)
 for(const item of items)if(descendants.has(item.key)&&item.status!=='invalid'){if(checked)next.add(item.key);else next.delete(item.key)}
 if(checked){let item=byKey.get(key);while(item?.parent_key){next.add(item.parent_key);item=byKey.get(item.parent_key)}}
 return next
}
