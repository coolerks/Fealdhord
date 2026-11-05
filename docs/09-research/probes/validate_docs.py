"""核验规格完整性、链接与任务边界；不执行应用测试。"""
from pathlib import Path
import hashlib
import json
import re
import sys

ROOT = Path(__file__).resolve().parents[3]
DOCS = ROOT / 'docs'
TASK = ROOT / '.trellis/tasks/11-04-fealdhord-spec'
errors = []


def require(condition, message):
    if not condition:
        errors.append(message)


def main():
    manifest = json.loads((DOCS / '00-product/requirements-manifest.json').read_text())
    source = ROOT / manifest['source']
    source_heads = re.findall(r'^# ([一二三四五六七八九十百零]+(?:-[A-E])?)、(.+)$', source.read_text(), re.M)
    rows = manifest['requirements']
    require(len(rows) == len(source_heads) == 113, '源章节数量不是 113')
    require(hashlib.sha256(source.read_bytes()).hexdigest() == manifest['sha256'], '原文哈希改变')
    require([(r['source_section'], r['title']) for r in rows] == source_heads, '追踪章节顺序或名称不一致')
    require(len({r['id'] for r in rows}) == 113, '需求 ID 重复')
    markdowns = list(DOCS.rglob('*.md')) + list((ROOT / '.trellis/spec/backend').glob('*.md')) + list((ROOT / '.trellis/spec/frontend').glob('*.md')) + [TASK / 'prd.md', TASK / 'design.md', TASK / 'implement.md', ROOT / 'README.md', ROOT / 'AGENTS.md']
    new_docs = [p for p in markdowns if p != source]
    definition_text = '\n'.join(p.read_text() for p in new_docs if p.name != 'requirements-traceability.md')
    for row in rows:
        require((DOCS / row['spec']).resolve().is_file(), f"追踪目标不存在：{row['id']}")
        # 验收需在正文被定义；不以追踪表自身的提及充数。
        criterion = re.escape(row['acceptance'])
        require(bool(re.search(rf'(?:^|\n)(?:- )?(?:(?:验收|完成判断) )?{criterion}[：／]', definition_text)), f"验收 ID 未定义：{row['acceptance']}")
    links = 0
    for p in markdowns:
        if p == source:
            continue  # 不修改用户不可变原始材料中的链接与示例。
        text = p.read_text()
        require(not re.search(r'To be filled|To fill|PLACEHOLDER|\bTODO\b|\bTBD\b', text), f'残留占位：{p.relative_to(ROOT)}')
        for index, line in enumerate(text.splitlines(), 1):
            require(line == line.rstrip(), f'行尾空白：{p.relative_to(ROOT)}:{index}')
        require(len(re.findall(r'^```', text, re.M)) % 2 == 0, f'代码围栏未闭合：{p.relative_to(ROOT)}')
        first = text.splitlines()[0]
        require(p.name == 'AGENTS.md' or bool(re.search(r'[\u4e00-\u9fff]', first)), f'新文档标题非中文：{p.relative_to(ROOT)}')
        for target in re.findall(r'\[[^\]]*\]\(([^)]+)\)', text):
            target = target.strip('<>')
            if re.match(r'^[a-zA-Z][a-zA-Z0-9+.-]*:', target) or target.startswith('#'):
                continue
            clean = target.split('#', 1)[0]
            if clean:
                links += 1
                require((p.parent / clean).resolve().exists(), f'失效链接：{p.relative_to(ROOT)} → {target}')
    contract_files = [p for layer in ['backend','frontend'] for p in (ROOT / f'.trellis/spec/{layer}').glob('*.md') if p.name != 'index.md'] + list((DOCS / '05-api').glob('*.md'))
    for p in contract_files:
        text = p.read_text()
        for n in range(1,8):
            require(bool(re.search(rf'^## {n}\. ', text, re.M)), f'契约缺少第 {n} 节：{p.relative_to(ROOT)}')
    adrs = sorted((DOCS / '06-decisions').glob('ADR-*.md'))
    require(len(adrs) >= 12 and [int(p.name[4:8]) for p in adrs] == list(range(1,len(adrs)+1)), 'ADR 编号不连续或缺少原始决策')
    for p in adrs:
        require(all(x in p.read_text() for x in ['## 背景','## 决定','## 替代方案','## 后果与验证']), f'ADR 结构不全：{p.name}')
    require(json.loads((TASK / 'task.json').read_text())['status'] in {'planning','in_progress','completed'}, '规格任务状态非法')
    require('dispatch_mode: inline' in (ROOT / '.trellis/config.yaml').read_text(), 'inline 配置丢失')
    require('禁止使用 subagent' in (ROOT / 'AGENTS.md').read_text(), '禁止子代理约束丢失')
    require((ROOT / 'go.mod').exists() == (ROOT / 'frontend/package.json').exists(), '前后端工程状态不一致')
    report = {'检查类型':'静态文档检查，非应用测试','Markdown 文件数':len(markdowns),'原始章节':len(source_heads),'追踪项':len(rows),'本地链接':links,'七节契约文件':len(contract_files),'ADR':len(adrs),'错误':errors}
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 1 if errors else 0


if __name__ == '__main__':
    sys.exit(main())
