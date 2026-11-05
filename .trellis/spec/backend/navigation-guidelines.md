# 分类管理与导航契约

## 1. 范围与触发条件

修改分类元数据、树结构、来源规则、订阅默认分类范围或侧栏计数时。复用 effective_article_categories，不复制手动／Feed／AI 优先级。

## 2. 签名

navigation.Service.List → Navigation{categories,feeds,totals}；Save(CategoryInput{id,revision,parent_id,name,icon,color,description,sort_position})；PreviewDelete{id} → CategoryDeletion{id,name,revision,tree_revision,children,articles,feeds}；Remove(CategoryDeletion)；Memberships{id:articleID}；SetMembership{article_id,category_id,state:auto|included|excluded}。

## 3. 契约

计数在同一读事务中聚合，全库 all／unread／favorites 不因多分类重复；分类仅直接有效成员，不默认包含后代。侧栏只列活动订阅，归档文章保留在全库。分类 Save CAS，数据库触发器拒绝环／五层越界；同父级名称唯一。图标限五个已接动效含义，颜色仅空值或 #RRGGBB；name≤256字节、description≤2048字节，排序位置±1000000。

删除先预览，tree_revision 为有序分类行摘要，事务中复核树和分类 revision，再把子项移到父级并删除分类；重名原子失败，不删除文章或订阅。文章 UI 只能写 manual 来源，auto 删除手动行；不能传 source 模拟 AI／Feed。

FeedService.Update 新增 apply_existing?:bool，默认 true。仅默认分类实际变化才重分配历史 Feed 来源；false 保留历史继承，仅新入库文章采用新默认。刷新已有文章不重新继承，重命名也不重分配；manual 行永不被覆盖。取消仅保证未提交事务回滚，前端不能声称已提交写入被撤销。

## 4. 校验与错误矩阵

过期编辑／删除树预览→Conflict；重名→Conflict；环／深度／非法元数据→ValidationError；目标不存在→NotFound；数据库未知错误为安全中文，不能回传 SQL／路径。Feed 保存请求取消后显示“重新载入确认结果”，避免承诺取消已发生的提交。

## 5. 良好、基础和异常案例

良好：一篇文章属于两类，全库仍计1；人工排除后刷新仍排除，恢复自动按其他来源生效。基础：空分类／空源可浏览。异常：预览后另加子项或更名，旧删除请求拒绝；子项迁父重名则整笔回滚。

## 6. 必需测试

真实 SQLite 计数／CAS／树／删除回滚／未来继承／仅改名／刷新；服务验证错误与 manual 边界；组件验证创建失败保留、删除预览取消／确认、成员自动／加入／排除、侧栏全库计数与持久 scope ID。图标使用 AnimatedGlyph，父节点旋转外层容器，不覆盖库 SVG 动画 transform。

## 7. 错误与正确做法

错误：以当前文章页数当侧栏计数，或为仅未来应用的旧文章在每次刷新时重新继承。正确：独立导航聚合 Query，出生时继承与显式历史重分配分开；分类变化的领域事件同时失效导航、订阅与文章缓存。普通 scope 清 selected_scope_id，feed／category scope 保存 ID，旧偏好安全默认空 ID。
