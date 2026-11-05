import { QueryClient,QueryClientProvider } from '@tanstack/react-query'
import { render,screen,within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach,expect,it,vi } from 'vitest'
import { useWorkspace } from '@/stores/workspace'
import { preferencesFixture } from '@/test/fixtures'
const api=vi.hoisted(()=>({list:vi.fn()}))
vi.mock('@/lib/navigation-api',()=>({navigationQueryKey:['navigation'],navigationAPI:api}))
vi.mock('@/lib/preferences',()=>({preferencesAPI:{update:vi.fn()}}))
vi.mock('@/lib/native',()=>({native:{isMac:()=>false}}))
import { TooltipProvider } from '@/components/ui/tooltip'
vi.mock('@/lib/library-api',()=>({libraryAPI:{}}))
vi.mock('@/lib/rss-api',()=>({rssAPI:{},feedsQueryKey:['rss','feeds']}))
import { Sidebar } from './Sidebar'
const category={id:'root',parent_id:'',name:'技术',revision:1,icon:'folder',color:'',description:'',sort_position:0,total:1000,unread:600}
beforeEach(()=>{
 useWorkspace.setState({preferences:preferencesFixture(),settingsOpen:false})
 api.list.mockResolvedValue({categories:[category,{...category,id:'child',parent_id:'root',name:'Go',total:800,unread:500}],feeds:[{id:'feed',title:'博客',total:1200,unread:700}],totals:{total:1500,unread:900,favorites:300}})
})
it('侧栏计数来自全库，展开分类和选择订阅保存正确 scope ID',async()=>{
 render(<QueryClientProvider client={new QueryClient()}><TooltipProvider><Sidebar motionMode="off"/></TooltipProvider></QueryClientProvider>)
 const all=screen.getByRole('button',{name:'全部文章'})
 expect(await within(all).findByText('900')).toBeInTheDocument()
 expect(screen.queryByRole('button',{name:'浏览分类 技术 / Go'})).not.toBeInTheDocument()
 await userEvent.click(await screen.findByRole('button',{name:'展开分类 技术'}))
 await userEvent.click(screen.getByRole('button',{name:'浏览分类 技术 / Go'}))
 expect(useWorkspace.getState().preferences).toEqual(expect.objectContaining({selected_scope:'category',selected_scope_id:'child'}))
 await userEvent.click(screen.getByRole('button',{name:'浏览订阅 博客'}))
 expect(useWorkspace.getState().preferences).toEqual(expect.objectContaining({selected_scope:'feed',selected_scope_id:'feed'}))
 await userEvent.click(all)
 expect(useWorkspace.getState().preferences).toEqual(expect.objectContaining({selected_scope:'all',selected_scope_id:''}))
})
