import { StrictMode } from 'react'
import { QueryClient,QueryClientProvider } from '@tanstack/react-query'
import { render,screen,waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach,expect,it,vi } from 'vitest'
const api=vi.hoisted(()=>({choose:vi.fn(),preview:vi.fn(),import:vi.fn(),export:vi.fn(),discard:vi.fn()}))
vi.mock('@/lib/transfer-api',()=>({transferAPI:api}))
vi.mock('@/lib/refresh',()=>({invalidateRSS:vi.fn()}))
import { TransferPanel } from './TransferPanel'
import { useWorkspace } from '@/stores/workspace'
const items=[{key:'folder',parent_key:'',title:'技术',kind:'folder',status:'new',reason:'',depth:1,url:'',site_url:''},{key:'feed',parent_key:'folder',title:'源',kind:'feed',status:'existing',reason:'',depth:2,url:'https://example.test/rss',site_url:''}]
const preview={preview_id:'preview',name:'订阅.opml',items}
function setup(){return render(<StrictMode><QueryClientProvider client={new QueryClient()}><TransferPanel/></QueryClientProvider></StrictMode>)}
beforeEach(()=>{vi.resetAllMocks();useWorkspace.setState({transferIntent:null});api.discard.mockResolvedValue(undefined);api.choose.mockResolvedValue({token:'grant',name:'订阅.opml',cancelled:false});api.preview.mockResolvedValue(preview);api.import.mockResolvedValue({report:{feeds_added:0,feeds_reused:1,feeds_restored:0,categories_added:1,associations_added:1,skipped:0},refresh_queued:true,refresh_error:''})})
it('预览不写入，确认携带选择与选项，只执行一次',async()=>{
 setup();await userEvent.click(screen.getByRole('button',{name:'导入 OPML'}));await screen.findByRole('region',{name:'OPML 导入预览'})
 expect(api.import).not.toHaveBeenCalled();expect(api.discard).not.toHaveBeenCalledWith('grant')
 await userEvent.click(screen.getByRole('checkbox',{name:'分类关联应用于已有文章'}));await userEvent.click(screen.getByRole('button',{name:'确认导入'}))
 await waitFor(()=>expect(api.import).toHaveBeenCalledTimes(1));expect(api.import.mock.calls[0][0]).toEqual({preview_id:'preview',selected:['folder','feed'],apply_existing:false,refresh:true})
 expect(await screen.findByText(/复用 1 个/)).toBeInTheDocument();expect(screen.queryByRole('button',{name:'确认导入'})).not.toBeInTheDocument()
})
it('取消预览不导入；原生取消不请求预览',async()=>{
 setup();await userEvent.click(screen.getByRole('button',{name:'导入 OPML'}));await screen.findByRole('button',{name:'确认导入'});await userEvent.click(screen.getByRole('button',{name:'取消'}));expect(api.import).not.toHaveBeenCalled();expect(api.discard).toHaveBeenCalledWith('','preview')
 api.choose.mockResolvedValue({token:'',name:'',cancelled:true});await userEvent.click(screen.getByRole('button',{name:'导入 OPML'}));await waitFor(()=>expect(screen.getByRole('button',{name:'导入 OPML'})).toBeEnabled());expect(api.preview).toHaveBeenCalledTimes(1)
})
it('早到菜单意图在 StrictMode 下只打开一次，卸载后丢弃晚到授权',async()=>{
 useWorkspace.setState({transferIntent:'import'});let resolve!:(value:unknown)=>void;api.choose.mockImplementation(()=>new Promise(done=>{resolve=done}))
 const view=setup();await waitFor(()=>expect(api.choose).toHaveBeenCalledTimes(1));view.unmount();resolve({token:'late',name:'late.opml',cancelled:false});await waitFor(()=>expect(api.discard).toHaveBeenCalledWith('late'));expect(api.preview).not.toHaveBeenCalled()
})
it('确认失败保留错误且要求重新预览，取消提交不承诺回滚',async()=>{
 api.import.mockRejectedValueOnce(new Error('订阅或分类已变化，请重新生成预览。'));setup();await userEvent.click(screen.getByRole('button',{name:'导入 OPML'}));await userEvent.click(await screen.findByRole('button',{name:'确认导入'}));expect(await screen.findByRole('alert')).toHaveTextContent('重新生成预览');expect(screen.queryByRole('button',{name:'确认导入'})).not.toBeInTheDocument()
})
it('导出由原生选择目标授权，返回文件名而非宿主路径',async()=>{
 api.export.mockResolvedValue({name:'Fealdhord.opml',bytes:128});setup();await userEvent.click(screen.getByRole('button',{name:'导出 OPML'}));expect(await screen.findByText('已导出 Fealdhord.opml（128 字节）')).toBeInTheDocument();expect(api.export).toHaveBeenCalledWith('grant',expect.any(AbortSignal));expect(api.import).not.toHaveBeenCalled()
})
it('大型预览限制 DOM，跨页选择仍可确认',async()=>{
 const many=Array.from({length:450},(_,index)=>({...items[1],key:`row-${index}`,parent_key:'',title:`源 ${index}`}));api.preview.mockResolvedValue({...preview,items:many});setup();await userEvent.click(screen.getByRole('button',{name:'导入 OPML'}));await screen.findByRole('region',{name:'OPML 导入预览'});expect(screen.getAllByRole('checkbox')).toHaveLength(202)
 await userEvent.click(screen.getByRole('button',{name:'下一页'}));expect(screen.getByText('源 200')).toBeInTheDocument();expect(screen.queryByText('源 0')).not.toBeInTheDocument();await userEvent.click(screen.getByRole('button',{name:'确认导入'}));await waitFor(()=>expect(api.import).toHaveBeenCalledTimes(1));expect(api.import.mock.calls[0][0].selected).toHaveLength(450)
})
it('取消在途预览会中止并丢弃晚到结果，重复点击不会再次选择',async()=>{
 let resolve!:(value:unknown)=>void;api.preview.mockImplementation(()=>new Promise(done=>{resolve=done}));setup();await userEvent.dblClick(screen.getByRole('button',{name:'导入 OPML'}));await waitFor(()=>expect(api.preview).toHaveBeenCalledTimes(1));const signal=api.preview.mock.calls[0][1];await userEvent.click(screen.getByRole('button',{name:'取消'}));expect(signal.aborted).toBe(true);resolve(preview);await waitFor(()=>expect(api.discard).toHaveBeenCalledWith('grant','preview'));expect(screen.queryByRole('button',{name:'确认导入'})).not.toBeInTheDocument();expect(api.import).not.toHaveBeenCalled()
})
