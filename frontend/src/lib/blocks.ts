// 消息内容分块：把回复拆成 普通文本 / ```typst / ```mermaid / ```其他代码。
//
// 模型被 system prompt 约定使用围栏标记排版与图：
//  ```typst
//  #set page(...)
//  $x^2$
//  ```
//  ```mermaid
//  flowchart LR
//  ...
//  ```

export type BlockKind = 'text' | 'typst' | 'mermaid' | 'code'

export interface Block {
  kind: BlockKind
  content: string
}

const FENCE_RE = /```([^\n`]*)\n([\s\S]*?)(?:```|$)/g

export function splitBlocks(src: string): Block[] {
  const blocks: Block[] = []
  let cursor = 0
  let match: RegExpExecArray | null

  FENCE_RE.lastIndex = 0
  while ((match = FENCE_RE.exec(src)) !== null) {
    if (match.index > cursor) {
      pushText(blocks, src.slice(cursor, match.index))
    }
    const lang = (match[1] || '').trim().toLowerCase()
    const body = match[2]
    if (lang === 'typst') {
      blocks.push({ kind: 'typst', content: body })
    } else if (lang === 'mermaid') {
      blocks.push({ kind: 'mermaid', content: body })
    } else {
      blocks.push({ kind: 'code', content: body })
    }
    cursor = match.index + match[0].length
  }
  if (cursor < src.length) {
    pushText(blocks, src.slice(cursor))
  }
  return blocks
}

function pushText(blocks: Block[], text: string) {
  const t = text.trim()
  if (t !== '') blocks.push({ kind: 'text', content: text })
}
