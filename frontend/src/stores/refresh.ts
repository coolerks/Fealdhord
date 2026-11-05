import { create } from 'zustand'
import type { Snapshot } from '@/lib/rss-api'
export const isRefreshing = (snapshot:Snapshot|null) => snapshot?.status === 'running' || snapshot?.status === 'cancelling'
type State = {
  snapshot:Snapshot|null; error:string|null; dismissedGeneration:number; pending:boolean
  accept:(next:Snapshot)=>void; dismiss:()=>void
}
export const useRefresh = create<State>((set) => ({
  snapshot:null,error:null,dismissedGeneration:-1,pending:false,
  accept(next) { set(state => {
    const current = state.snapshot
    if(next.generation<=state.dismissedGeneration || (current && (next.generation<current.generation || (next.generation===current.generation && next.sequence<=current.sequence)))) return state
    return {snapshot:next,error:null}
  }) },
  dismiss() { set(state => isRefreshing(state.snapshot) ? state : {snapshot:null,error:null,dismissedGeneration:state.snapshot?.generation ?? state.dismissedGeneration}) },
}))
