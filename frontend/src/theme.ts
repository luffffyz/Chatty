// 主题与字号应用层：依据 config.Settings.Appearance 设置
// <html data-theme> 与 --fs-* 字号刻度。
import type { Appearance } from '../bindings/chatty/internal/config/models'

export type ResolvedTheme = 'light' | 'dark'

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

  const target: ResolvedTheme = resolve(app?.theme, mql.matches)
  document.documentElement.dataset.theme = target

  // system 模式下监听系统切换；非 system 时移除监听
  const shouldWatch = app?.theme === 'system'
  if (shouldWatch && !systemListener) {
    systemListener = () => {
      document.documentElement.dataset.theme = resolve('system', mql.matches)
    }
    mql.addEventListener('change', systemListener)
  } else if (!shouldWatch && systemListener) {
    mql.removeEventListener('change', systemListener)
    systemListener = null
  }
}
