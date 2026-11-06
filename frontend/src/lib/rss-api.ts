import { Events } from '@wailsio/runtime'
import '../../bindings/events'
import * as FeedService from '../../bindings/github.com/coolerks/Fealdhord/internal/feed/service'
import * as RefreshService from '../../bindings/github.com/coolerks/Fealdhord/internal/refresh/api'
import * as MediaService from '../../bindings/github.com/coolerks/Fealdhord/internal/media/api'
import { unwrap, APIError } from './api'
import type { AddRequest, UpdateRequest, RevisionRequest } from '../../bindings/github.com/coolerks/Fealdhord/internal/feed/models'
import type { Snapshot } from '../../bindings/github.com/coolerks/Fealdhord/internal/refresh/models'
export type { Feed, Candidate, Page, UpdateRequest } from '../../bindings/github.com/coolerks/Fealdhord/internal/feed/models'
export type { Snapshot } from '../../bindings/github.com/coolerks/Fealdhord/internal/refresh/models'
export const feedsQueryKey = ['rss', 'feeds'] as const
export const rssAPI = {
  async list() { return unwrap(await FeedService.List({include_archived:true})) },
  async discover(url:string) { return unwrap(await FeedService.Discover({url})).candidates ?? [] },
  async previewIcon(url:string, signal:AbortSignal) {
    const call = FeedService.PreviewIcon({url})
    const cancel = () => call.cancel()
    if (signal.aborted) cancel()
    signal.addEventListener('abort', cancel, {once:true})
    try { return await call } finally { signal.removeEventListener('abort', cancel) }
  },
  async add(request:AddRequest) { return unwrap(await FeedService.Add(request)) },
  async update(request:UpdateRequest,signal?:AbortSignal) {
    const call=FeedService.Update(request),cancel=()=>call.cancel()
    if(signal?.aborted) call.cancel()
    signal?.addEventListener('abort',cancel,{once:true})
    try{return unwrap(await call)}catch(error){if(signal?.aborted)throw new Error('保存请求已取消，请重新载入确认结果。');throw error}finally{signal?.removeEventListener('abort',cancel)}
  },
  async archive(request:RevisionRequest) { return unwrap(await FeedService.Remove(request)) },
  async restore(request:RevisionRequest) { return unwrap(await FeedService.Restore(request)) },
  async start(feed_ids:string[] = []) { return unwrap(await RefreshService.Start({feed_ids})) },
  async get(operation_id:string) { return unwrap(await RefreshService.Get({operation_id})) },
  async current() { return unwrap(await RefreshService.Current()) },
  async cancel(operation_id:string) { return unwrap(await RefreshService.Cancel({operation_id})) },
  async clearCache() {
    const result = await MediaService.ClearCache()
    if (!result.ok) throw new APIError(result.error ?? {code:'IOError',message:'清理缓存失败。',retryable:true})
  },
  subscribeRefresh(listener:(snapshot:Snapshot)=>void) {
    return Events.On('fealdhord:refresh', event => { if(event.data.schema_version===1) listener(event.data.payload) })
  },
  subscribeLibrary(listener:()=>void) { return Events.On('fealdhord:library-changed', listener) },
}
