// 消息内容按"顶级结构块"切分，每块独立排版、独立失败回退：
//  - ```mermaid 围栏 → mermaid 块（交给 MermaidBlock 渲染）
//  - 其它 ``` 围栏（raw 代码块）→ 整块作为一个独立 Typst 文档段
//  - 普通文本按空行切段 → 每段是一个独立 Typst 文档段
// 好处：不必等整条消息完成后一次性编译；某段语法错只回退该段（标注
// 第 N 段），其余照常渲染，方便定位渲染失败位置。
//
// streaming 时逐段判定"稳定"（只排版已闭合的段，避免拿半截语法编译）：
//  - 文本段：其后出现空行/围栏边界即视为闭合；
//  - 围栏段：必须有闭合的 ```；未闭合部分显示原文。

export type DocPart =
  | { kind: 'text'; text: string; stable: boolean } // typst 主体段落
  | { kind: 'code'; text: string; stable: boolean } // ``` raw 围栏整块(含围栏行)
  | { kind: 'mermaid'; text: string; stable: boolean } // ```mermaid 源码(不含围栏)

const isFence = (line: string): boolean => line.trimStart().startsWith('```')

/**
 * splitDocParts 把整条 assistant 消息切成独立文档段。
 * @param done 消息是否已结束；false 时末尾未闭合段 stable=false。
 */
export function splitDocParts(src: string, done = true): DocPart[] {
  const parts: DocPart[] = []
  const textLines: string[] = []
  const n = src.length
  let i = 0

  const flushText = (stable: boolean) => {
    if (textLines.length === 0) return
    const text = textLines.join('\n').trim()
    textLines.length = 0
    if (text) parts.push({ kind: 'text', text, stable })
  }

  while (i < n) {
    const nl = src.indexOf('\n', i)
    const lineEnd = nl === -1 ? n : nl // 行尾(不含 \n)
    const line = src.slice(i, lineEnd)

    if (!isFence(line)) {
      if (line.trim() === '') {
        flushText(true) // 空行 = 段落边界，此前的段落已闭合
      } else {
        textLines.push(line)
      }
      i = lineEnd + 1
      continue
    }

    // ---------- 围栏开始 ----------
    flushText(true) // 围栏边界切断前面的文本段（已闭合）
    const fenceStart = i
    const lang = line.slice(3).trim().toLowerCase()
    const afterOpen = lineEnd + 1 // 开围栏行后的首字节

    // 寻找闭合 ```（行首允许空白）
    let closeStart = -1
    let closeEnd = n // 闭合行行尾(不含 \n)
    let j = afterOpen
    while (j <= n) {
      const jnl = src.indexOf('\n', j)
      const jend = jnl === -1 ? n : jnl
      if (isFence(src.slice(j, jend))) {
        closeStart = j
        closeEnd = jend
        i = jend + 1
        break
      }
      j = jend + 1
    }

    if (closeStart === -1) {
      // 未闭合：streaming 中途(原文显示)；消息结束时也按原文处理，便于定位
      parts.push({ kind: 'text', text: src.slice(fenceStart), stable: false })
      return parts
    }

    if (lang === 'mermaid') {
      parts.push({ kind: 'mermaid', text: src.slice(afterOpen, closeStart).trim(), stable: true })
    } else {
      const whole = src.slice(fenceStart, closeEnd) // fenceStart..闭合行(不含末尾换行)
      parts.push({ kind: 'code', text: whole, stable: true })
    }
  }

  flushText(done) // 消息结束；末尾文本段 done=false 时仍视为未闭合
  return parts
}
