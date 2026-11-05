/** 从已安装的 MIT 图标生成同形静态 SVG；更换库源后运行 npm run icons:generate。 */
import { readFileSync, writeFileSync, readdirSync } from 'node:fs'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'
import vm from 'node:vm'
import ts from 'typescript'
import React from 'react'
import { renderToStaticMarkup } from 'react-dom/server'

const root = new URL('../src/components/icons/', import.meta.url)
const require = createRequire(import.meta.url)
const stills = {}
const imports = []
const entries = []
for (const file of readdirSync(root).filter(file => file.endsWith('.tsx')).sort()) {
  const filename = fileURLToPath(new URL(file, root))
  const source = readFileSync(filename, 'utf8')
  const { outputText } = ts.transpileModule(source, { compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } })
  const module = { exports: {} }
  // 执行的只有仓库中已审阅的图标源；cn 在构建时无需引入应用别名。
  vm.runInNewContext(outputText, { exports: module.exports, module, require: id => id === '@/lib/utils' ? { cn: (...values) => values.filter(Boolean).join(' ') } : require(id) }, { filename })
  const [name, Component] = Object.entries(module.exports).find(([name]) => name.endsWith('Icon'))
  const markup = renderToStaticMarkup(React.createElement(Component, { size: 20 }))
  const key = file.replace('.tsx', '')
  stills[key] = markup.slice(markup.indexOf('<svg'), markup.lastIndexOf('</svg>') + 6)
  imports.push(`import { ${name} } from './icons/${key}'`)
  entries.push(`  '${key}': ${name},`)
}
writeFileSync(new URL('../src/components/icon-stills.json', import.meta.url), JSON.stringify(stills, null, 2) + '\n')
writeFileSync(new URL('../src/components/icon-catalog.ts', import.meta.url), '// 由 npm run icons:generate 从已安装的 lucide-animated 源生成，请勿手改。\n' + imports.join('\n') + '\n\nexport const animatedIcons = {\n' + entries.join('\n') + '\n}\nexport type AnimatedIconName = keyof typeof animatedIcons\n')
console.log(`已生成 ${entries.length} 个官方同形静态图标与类型目录。`)
