// Mermaid 渲染封装：跟随当前主题。
//  浅色：theme 'default'（白底黑字，现状）
//  暗色：theme 'base' + 深色背景(外观里九选一)/白字/白边框
import mermaid from 'mermaid'
import { DEFAULT_CHART_BG, view } from '../theme'

let seq = 0

function darkVariables(bg: string): Record<string, string> {
  const white = '#ffffff'
  return {
    background: bg,
    primaryColor: bg,
    primaryBorderColor: white,
    primaryTextColor: white,
    secondaryColor: bg,
    tertiaryColor: bg,
    lineColor: '#ffffffcc',
    edgeLabelBackground: bg,
    fontColor: white,
    nodeTextColor: white,
    titleColor: white,
    clusterBkg: bg,
    clusterBorder: white,
    // sequence
    actorBkg: bg,
    actorBorder: white,
    actorTextColor: white,
    actorLineColor: '#ffffffcc',
    signalColor: white,
    signalTextColor: white,
    labelBoxBkg: bg,
    labelBoxBorder: white,
    labelTextColor: white,
    loopTextColor: white,
    noteBkgColor: bg,
    noteBorderColor: white,
    noteTextColor: white,
    activationBkgColor: '#ffffff33',
    activationBorderColor: white,
    // class / state
    classText: white,
    stateBkg: bg,
    stateBorder: white,
    stateLabelColor: white,
  }
}

export function renderMermaid(code: string): Promise<string> {
  const dark = view.theme === 'dark'
  const fontFamily = 'system-ui, sans-serif'

  if (dark) {
    const bg = view.chartBg || DEFAULT_CHART_BG
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      theme: 'base',
      fontFamily,
      themeVariables: darkVariables(bg),
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
