// Mermaid 渲染封装。
import mermaid from 'mermaid'

let initialized = false
let seq = 0

export function renderMermaid(code: string): Promise<string> {
  if (!initialized) {
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      theme: 'default',
      fontFamily: 'system-ui, sans-serif',
    })
    initialized = true
  }
  const id = `chatty-mermaid-${++seq}`
  return mermaid.render(id, code).then((r) => r.svg)
}
