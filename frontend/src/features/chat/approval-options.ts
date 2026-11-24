import { ClipboardCheck, Key, SlidersHorizontal, X } from 'lucide-react'

export const approvalModes = [
  { value: 'ask', label: '需要审批', icon: ClipboardCheck, meaning: 'clipboard-check' as const },
  { value: 'bypass', label: '绕过审批', icon: Key, meaning: 'key' as const },
  { value: 'deny', label: '拒绝审批', icon: X, meaning: 'x' as const },
  { value: 'custom', label: '自定义权限', icon: SlidersHorizontal, meaning: 'sliders-horizontal' as const },
]

export const approvalPresentations = [
  { value: 'composer', label: '输入框内审批' },
  { value: 'dialog', label: '对话框审批' },
]
