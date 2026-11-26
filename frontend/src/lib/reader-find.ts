import { useWorkspace } from '@/stores/workspace'

export function readReaderSelection(content: Element | null | undefined) {
  const selection = window.getSelection()
  if (!selection?.anchorNode || !selection.focusNode ||
    !content?.contains(selection.anchorNode) || !content.contains(selection.focusNode)) return ''
  return selection.toString()
}

export function requestReaderFind(articleID: string, selection: string) {
  const query = selection.replace(/\r\n?|\n/g, ' ').trim()
  useWorkspace.setState({ readerFindRequest: { articleID, query } })
}
