import * as Service from '../../bindings/github.com/coolerks/Fealdhord/internal/reader/service'
import { APIError,unwrap } from './api'
export type { Detail } from '../../bindings/github.com/coolerks/Fealdhord/internal/reader/models'
export const readerAPI={
 async get(article_id:string,mode='feed',signal?:AbortSignal){
  const call=Service.Get({article_id,mode}),cancel=()=>call.cancel()
  if(signal?.aborted)call.cancel()
  signal?.addEventListener('abort',cancel,{once:true})
  try{return unwrap(await call)}finally{signal?.removeEventListener('abort',cancel)}
 },
 async extract(article_id:string,signal:AbortSignal){
  const call=Service.Extract({article_id,mode:'reader'}),cancel=()=>call.cancel()
  if(signal.aborted)call.cancel()
  signal.addEventListener('abort',cancel,{once:true})
  try{return unwrap(await call)}catch(error){if(signal.aborted)throw new Error('提取已取消，当前正文已保留。');throw error}finally{signal.removeEventListener('abort',cancel)}
 },
 async open(article_id:string,resource_id=''){
  const result=await Service.OpenLink({article_id,resource_id})
  if(!result.ok)throw new APIError(result.error ?? {code:'IOError',message:'无法打开链接。',retryable:true})
 },
}
