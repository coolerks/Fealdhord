# 前端与动效调研

核验日期：2026-11-04。

## 官方事实

[React](https://react.dev/learn)、[Vite](https://vite.dev/guide/)、[Tailwind Vite 插件](https://tailwindcss.com/docs/installation/using-vite)、[shadcn Vite 安装](https://ui.shadcn.com/docs/installation/vite)提供对应集成路线。[TanStack Query](https://tanstack.com/query/latest/docs/framework/react/guides/query-keys)用查询键隔离异步缓存；[Zustand](https://zustand.docs.pmnd.rs/learn/guides/auto-generating-selectors)建议选择器订阅；[Lucide](https://lucide.dev/guide/react)提供 React 图标。

[TanStack Virtual](https://tanstack.com/virtual/latest/docs/api/virtualizer)支持 lanes、动态测量和滚动 API；多列能力不自动解决逻辑顺序、异步图片和锚点保持。[Motion 无障碍](https://motion.dev/docs/react-accessibility)提供 reduced motion 支持。

## 项目决定

Query 管领域快照，Zustand 管 UI；双视图复用同一查询。瀑布流采用逻辑索引顺序的 DOM 与显式坐标定位，首图尺寸预估，ResizeObserver 测量；列数变化先保存 article_id + offset 锚点，再重建位置。高频滚动禁用大范围 layout 动画，简单按钮用 CSS。

## 未验证

依赖精确版本、锁文件与 WebView 最低版本在 Phase 0 固定；不凭最新版 CSS 在本机浏览器可见就认定 Linux WebKitGTK 兼容。动态 lanes 与顺序、锚点的原型是 Phase 1 门禁；失败时调整定位算法，不以 CSS columns 替代。尚未生成 UI 或运行组件测试。
