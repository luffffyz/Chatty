// Mermaid 渲染封装：画布跟随主题。
//  浅色：theme 'default'（白画布）
//  暗色：theme 'base' + 黑画布 + 白字/白描边（节点用深灰黑底，靠白边勾勒）
import mermaid from 'mermaid'
import { view } from '../theme'

let seq = 0

// 暗色画布：黑色；节点填充略亮的深灰，白字白边。
const CANVAS = '#000000'
const NODE = '#14161a'

function darkVariables(): Record<string, string> {
  const white = '#ffffff'
  return {
    background: CANVAS,
    primaryColor: NODE,
    primaryBorderColor: white,
    primaryTextColor: white,
    secondaryColor: NODE,
    secondaryBorderColor: white,
    tertiaryColor: NODE,
    tertiaryBorderColor: white,
    lineColor: '#ffffffcc',
    edgeLabelBackground: '#24262b',
    fontColor: white,
    nodeTextColor: white,
    titleColor: white,
    clusterBkg: CANVAS,
    clusterBorder: white,
    // sequence
    actorBkg: NODE,
    actorBorder: white,
    actorTextColor: white,
    actorLineColor: '#ffffffcc',
    signalColor: white,
    signalTextColor: white,
    labelBoxBkg: NODE,
    labelBoxBorder: white,
    labelTextColor: white,
    loopTextColor: white,
    noteBkgColor: NODE,
    noteBorderColor: white,
    noteTextColor: white,
    activationBkgColor: '#ffffff33',
    activationBorderColor: white,
    // class / state
    classText: white,
    stateBkg: NODE,
    stateBorder: white,
    stateLabelColor: white,
  }
}

export function renderMermaid(code: string): Promise<string> {
  const dark = view.theme === 'dark'
  const fontFamily = 'system-ui, sans-serif'

  if (dark) {
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      theme: 'base',
      fontFamily,
      themeVariables: darkVariables(),
    })
  } else {
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      theme: 'default',
      fontFamily,
    })
  }

  const id = `chatty-mermaid-${++seq}`
  return mermaid.render(id, code).then((r) => r.svg)
}
