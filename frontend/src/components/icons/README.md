# 官方动画图标维护

图标来自 lucide-animated 官方 shadcn 注册表，MIT 许可证见 LICENSE。图形与动画轨迹保留官方版本；本项目仅补充 Promise 完成类型、取消接口和多阶段完成等待／代次校验。

新增时通过 npx shadcn@latest view／add 读取和安装 https://lucide-animated.com/r/图标名.json，再移至本目录；更新前先比较现有生命周期适配，不能直接覆盖。库不含全部 Lucide 图形，选用语义相近的官方图形，不自行仿制轨迹。

在 frontend 运行 npm run icons:generate，生成 icon-catalog.ts 和 icon-stills.json，静态 SVG 来自相同组件的初始渲染。禁止手改生成文件或把用户／网络 HTML 放入静态目录。修改完成后运行图标生命周期、目录播放测试和类型检查。
