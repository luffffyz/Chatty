// 主题与字号应用层：依据 config.Settings.Appearance 设置
// <html data-theme> 与 --fs-* 字号刻度；同时维护响应式 view，
// 供 Typst 渲染等需要感知主题/字号的组件订阅。
import { reactive } from 'vue'
import type { Appearance } from '../bindings/chatty/internal/config/models'

/** 暗色主题下 mermaid 图可选的深色背景（用户指定色板）。 */
export const CHART_BG_COLORS = [
  '#6e2b2b',
  '#6e3a2b',
  '#75561c',
  '#69622b',
  '#46692b',
  '#255947',
  '#254d59',
  '#403159',
  '#593148',
]
export const DEFAULT_CHART_BG = CHART_BG_COLORS[0]

export type ResolvedTheme = 'light' | 'dark'

/** 当前已解析的主题与基础字号（Typst/Mermaid 渲染依赖）。 */
export const view = reactive<{ theme: ResolvedTheme; fontSize: number; chartBg: string }>({
  theme: 'light',
  fontSize: 14,
  chartBg: '',
})

/** 主题正文色（给 Typst #set text(fill: ...) 用）。 */
export const textHex = (): string => (view.theme === 'dark' ? '#e6e8ec' : '#1f2328')

const mql = window.matchMedia('(prefers-color-scheme: dark)')

let systemListener: ((e: MediaQueryListEvent) => void) | null = null

function resolve(theme: string | undefined, prefersDark: boolean): ResolvedTheme {
  if (theme === 'dark') return 'dark'
  if (theme === 'light') return 'light'
  return prefersDark ? 'dark' : 'light'
}

// 字号刻度随基础字号缩放：base=FontSize
function applyFontScale(base: number) {
  const root = document.documentElement
  const set = (name: string, v: number) => root.style.setProperty(name, `${v}px`)
  set('--fs-md', base)
  set('--fs-2xs', base - 3)
  set('--fs-xs', base - 2)
  set('--fs-sm', base - 1)
  set('--fs-lg', base + 2)
  set('--fs-xl', base + 6)
}

/** 应用外观设置；可在启动、设置保存、系统主题变化时反复调用。 */
export function applyAppearance(app?: Appearance | null): void {
  const base = app && app.fontSize > 0 ? app.fontSize : 14
  applyFontScale(base)
  view.fontSize = base
  view.chartBg = app?.chartBg ?? ''

  const target: ResolvedTheme = resolve(app?.theme, mql.matches)
  view.theme = target
  document.documentElement.dataset.theme = target

  // system 模式下监听系统切换；非 system 时移除监听
  const shouldWatch = app?.theme === 'system'
  if (shouldWatch && !systemListener) {
    systemListener = () => {
      const t = resolve('system', mql.matches)
      view.theme = t
      document.documentElement.dataset.theme = t
    }
    mql.addEventListener('change', systemListener)
  } else if (!shouldWatch && systemListener) {
    mql.removeEventListener('change', systemListener)
    systemListener = null
  }
}
