import * as Service from '../../bindings/github.com/coolerks/Fealdhord/internal/navigation/service'
import { unwrap } from './api'
import type { CategoryInput, CategoryDeletion } from '../../bindings/github.com/coolerks/Fealdhord/internal/database/models'
export type { Navigation, CategoryInfo, CategoryInput, CategoryDeletion, MembershipInfo } from '../../bindings/github.com/coolerks/Fealdhord/internal/database/models'
export const navigationQueryKey=['navigation'] as const
export const navigationAPI={
 async list(){return unwrap(await Service.List())},
 async save(input:CategoryInput){return unwrap(await Service.Save(input))},
 async previewDelete(id:string){return unwrap(await Service.PreviewDelete({id}))},
 async remove(input:CategoryDeletion){return unwrap(await Service.Remove(input))},
 async memberships(id:string){return unwrap(await Service.Memberships({id})).items ?? []},
 async setMembership(article_id:string,category_id:string,state:string){return unwrap(await Service.SetMembership({article_id,category_id,state}))},
}
