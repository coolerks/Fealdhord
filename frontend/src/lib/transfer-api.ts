import * as Service from '../../bindings/github.com/coolerks/Fealdhord/internal/transfer/api'
import type { PreviewData as WirePreview } from '../../bindings/github.com/coolerks/Fealdhord/internal/transfer/models'
export type PreviewData=Omit<WirePreview,'items'>&{items:NonNullable<WirePreview['items']>}
import { unwrap } from './api'
import type { CancellablePromise } from '@wailsio/runtime'
export type { ImportRequest } from '../../bindings/github.com/coolerks/Fealdhord/internal/transfer/models'
export type { ImportOutline } from '../../bindings/github.com/coolerks/Fealdhord/internal/database/models'
async function cancellable<T>(call:CancellablePromise<T>,signal:AbortSignal):Promise<T>{
 const cancel=()=>call.cancel()
 if(signal.aborted)cancel()
 signal.addEventListener('abort',cancel,{once:true})
 try{return await call}finally{signal.removeEventListener('abort',cancel)}
}
export const transferAPI={
 async choose(kind:'import'|'export'){return unwrap(await (kind==='import'?Service.ChooseImport():Service.ChooseExport()))},
 async preview(token:string,signal:AbortSignal){const data=unwrap(await cancellable(Service.PreviewOPML({dialog_token:token}),signal));return {...data,items:data.items??[]}},
 async import(request:Parameters<typeof Service.ImportOPML>[0],signal:AbortSignal){return unwrap(await cancellable(Service.ImportOPML(request),signal))},
 async export(token:string,signal:AbortSignal){return unwrap(await cancellable(Service.ExportOPML({dialog_token:token}),signal))},
 async discard(token='',previewID=''){await Service.Discard({dialog_token:token,preview_id:previewID})},
}
