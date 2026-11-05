import { expect, it } from 'vitest'
import { libraryKey } from './library-query'
import { articleQueryKey } from './library'
it('默认查询兼容基础键，过滤与页大小不能共用缓存',()=>{
 const query={scope:{kind:'all'},sort:'published_desc',limit:50}
 expect(libraryKey(query)).toEqual(articleQueryKey)
 expect(libraryKey({...query,favorite:false})).not.toEqual(articleQueryKey)
 expect(libraryKey({...query,limit:200})).not.toEqual(articleQueryKey)
 expect(libraryKey({...query,search:'并发'})).not.toEqual(articleQueryKey)
 expect(libraryKey({...query,scope:{kind:'feed',id:'one'}})).not.toEqual(libraryKey({...query,scope:{kind:'feed',id:'two'}}))
})
