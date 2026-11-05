import type { CategoryInfo } from '@/lib/navigation-api'
export function categoryPath(category:CategoryInfo,categories:CategoryInfo[]){
 const names=[category.name],seen=new Set([category.id]);let parentID=category.parent_id
 for(let depth=0;parentID&&depth<5;depth++){
  if(seen.has(parentID))break
  seen.add(parentID);const parent=categories.find(item=>item.id===parentID)
  if(!parent)break
  names.unshift(parent.name);parentID=parent.parent_id
 }
 return names.join(' / ')
}
