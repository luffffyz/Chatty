// 消息内容按“整条 = Typst 文档”切分：
//  - ```mermaid 围栏内的源码被抽出单独渲染成图；
//  - 其余全部内容原样作为 Typst 源码（typst 语言中 ``` 块即 raw 代码
//    块，模型可用它展示代码；若出现 ```typst 围栏也兼容抽取其内容）。

export interface TypstSegments {
  /** 拼好的 Typst 源码主体 */
  typst: string
  /** 各 mermaid 图源码（按出现顺序） */
  diagrams: string[]
}

const FENCE_RE = /```([^\n`]*)\n([\s\S]*?)(?:```|$)/g

export function splitTypstMessage(src: string): TypstSegments {
  const parts: string[] = []
  const diagrams: string[] = []
  let cursor = 0

  FENCE_RE.lastIndex = 0
  let match: RegExpExecArray | null
  while ((match = FENCE_RE.exec(src)) !== null) {
    const lang = (match[1] || '').trim().toLowerCase()
    const body = match[2]

    if (lang === 'mermaid') {
      parts.push(src.slice(cursor, match.index))
      diagrams.push(body)
      cursor = match.index + match[0].length
      continue
    }
    if (lang === 'typst') {
      // 兼容旧版 prompt 的 ```typst 围栏：围栏内容直接进入排版主体
      parts.push(src.slice(cursor, match.index))
      parts.push('\n\n')
      parts.push(body)
      cursor = match.index + match[0].length
      continue
    }
    // 其它 ``` 块保留原文 —— typst 视其为 raw 代码块
  }

  if (cursor < src.length) {
    parts.push(src.slice(cursor))
  }
  return { typst: parts.join(''), diagrams }
}
