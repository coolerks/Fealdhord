import { AnimatedGlyph } from '@/components/animated-glyph'

export const approvalModes = [
  { value: 'ask', label: '需要审批', graphic: <AnimatedGlyph meaning="clipboard-check" /> },
  { value: 'bypass', label: '绕过审批', graphic: <AnimatedGlyph meaning="key" /> },
  { value: 'deny', label: '拒绝审批', graphic: <AnimatedGlyph meaning="x" /> },
]

export const approvalPresentations = [
  { value: 'composer', label: '输入框内审批' },
  { value: 'dialog', label: '对话框审批' },
]
