import { QueryClient,QueryClientProvider } from '@tanstack/react-query'
import { render,screen,waitFor,within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach,expect,it,vi } from 'vitest'
const api=vi.hoisted(()=>({list:vi.fn(),save:vi.fn(),previewDelete:vi.fn(),remove:vi.fn(),memberships:vi.fn(),setMembership:vi.fn()}))
vi.mock('@/lib/navigation-api',()=>({navigationQueryKey:['navigation'],navigationAPI:api}))
vi.mock('@/lib/rss-api',()=>({feedsQueryKey:['rss','feeds']}))
import { CategoryManager } from './CategoryManager'
import { ArticleCategories } from './ArticleCategories'
const category={id:'cat',revision:3,parent_id:'',name:'技术',icon:'folder',color:'',description:'',sort_position:0,total:12,unread:8}
const preview={id:'cat',revision:3,name:'技术',tree_revision:'hash',children:2,articles:12,feeds:1}
function setup(element:React.ReactNode){return render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}})}>{element}</QueryClientProvider>)}
beforeEach(()=>{vi.resetAllMocks();api.list.mockResolvedValue({categories:[category],feeds:[],totals:{total:12,unread:8,favorites:0}})})
it('分类创建失败保留表单，不冒充已保存',async()=>{
 api.save.mockRejectedValue(new Error('同级分类重名'))
 setup(<CategoryManager/>);await userEvent.click(screen.getByRole('button',{name:'新建分类'}))
 const dialog=screen.getByRole('dialog',{name:'新建分类'})
 await userEvent.type(within(dialog).getByRole('textbox',{name:'名称'}),'技术')
 await userEvent.click(within(dialog).getByRole('button',{name:'保存分类'}))
 expect(await within(dialog).findByRole('alert')).toHaveTextContent('同级分类重名')
 expect(within(dialog).getByRole('textbox',{name:'名称'})).toHaveValue('技术')
})
it('删除先读取影响预览；取消不写入，确认携带树版本',async()=>{
 api.previewDelete.mockResolvedValue(preview);api.remove.mockResolvedValue({id:'cat'})
 setup(<CategoryManager/>);await userEvent.click(await screen.findByRole('button',{name:'删除分类 技术'}))
 let dialog=await screen.findByRole('dialog',{name:'删除“技术”？'})
 expect(within(dialog).getByText(/2 个子分类/)).toBeInTheDocument();expect(api.remove).not.toHaveBeenCalled()
 await userEvent.click(within(dialog).getByRole('button',{name:'取消'}));expect(api.remove).not.toHaveBeenCalled()
 await userEvent.click(screen.getByRole('button',{name:'删除分类 技术'}));dialog=await screen.findByRole('dialog',{name:'删除“技术”？'})
 await userEvent.click(within(dialog).getByRole('button',{name:'确认删除分类'}))
 await waitFor(()=>expect(api.remove).toHaveBeenCalledWith(preview))
})
it('手动排除与恢复自动是独立明确操作',async()=>{
 api.memberships.mockResolvedValue([{category_id:'cat',effective:true,manual:'auto'}]);api.setMembership.mockResolvedValue({id:'article'})
 setup(<ArticleCategories articleID="article" open onOpenChange={() => {}}/>)
 await userEvent.click(await screen.findByRole('radio',{name:'排除'}))
 await waitFor(()=>expect(api.setMembership).toHaveBeenCalledWith('article','cat','excluded'))
 api.memberships.mockResolvedValue([{category_id:'cat',effective:false,manual:'excluded'}])
 await userEvent.click(screen.getByRole('radio',{name:'加入'}))
 await waitFor(()=>expect(api.setMembership).toHaveBeenCalledWith('article','cat','included'))
 await waitFor(()=>expect(screen.getByRole('radio',{name:'自动'})).toBeEnabled())
 await userEvent.click(screen.getByRole('radio',{name:'自动'}))
 await waitFor(()=>expect(api.setMembership).toHaveBeenCalledWith('article','cat','auto'))
})
