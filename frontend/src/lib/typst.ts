// Typst 渲染封装：浏览器内 wasm 编译 typst 源码 → SVG。
//
// 使用说明（typst.ts 官方集成模式）：
// 需显式 import 两个 wasm 子包产物，并通过 setCompilerInitOptions /
// setRendererInitOptions 提供 getModule，否则浏览器端 wasm 加载会抛
// "Cannot import wasm module without importer"。`?url` 让 Vite 把 wasm
// 作为静态资源输出并返回其 URL。
//
// $typst 是共享单例，编译过程会改动其内部状态，因此并发渲染必须
// 串行化——用一个 promise 链排队，单条失败不阻塞后续。
import { $typst } from '@myriaddreamin/typst.ts'
import type { BeforeBuildFn } from '@myriaddreamin/typst.ts'
import compilerWasmUrl from '@myriaddreamin/typst-ts-web-compiler/pkg/typst_ts_web_compiler_bg.wasm?url'
import rendererWasmUrl from '@myriaddreamin/typst-ts-renderer/pkg/typst_ts_renderer_bg.wasm?url'

// public/fonts 下打包的字体（含两个 ~24MB 的思源宋全量）。
// 默认 typst-assets 三组 25 个 + 思源 Bold。
const FONT_NAMES: string[] = [
  'DejaVuSansMono-Bold.ttf',
  'DejaVuSansMono-BoldOblique.ttf',
  'DejaVuSansMono-Oblique.ttf',
  'DejaVuSansMono.ttf',
  'InriaSerif-Bold.ttf',
  'InriaSerif-BoldItalic.ttf',
  'InriaSerif-Italic.ttf',
  'InriaSerif-Regular.ttf',
  'LibertinusSerif-Bold.otf',
  'LibertinusSerif-BoldItalic.otf',
  'LibertinusSerif-Italic.otf',
  'LibertinusSerif-Regular.otf',
  'LibertinusSerif-Semibold.otf',
  'LibertinusSerif-SemiboldItalic.otf',
  // 微软雅黑（本机复制，见 scripts/fetch-system-fonts.ps1；可能缺失，加载会告警跳过）
  'msyh.ttc',
  'msyhbd.ttc',
  'wqy-zenhei.ttf', // 文泉驿正黑(开源 OFL)，思源宋替换，体积 -36MB
  'NewCM10-Bold.otf',
  'NewCM10-BoldItalic.otf',
  'NewCM10-Italic.otf',
  'NewCM10-Regular.otf',
  'NewCMMath-Bold.otf',
  'NewCMMath-Book.otf',
  'NewCMMath-Regular.otf',
  'NotoColorEmoji-Regular-COLR.subset.ttf',
  'Roboto-Regular.ttf',
  'TwitterColorEmoji.ttf',
]

// 逐个、容忍失败地加载字体：某个字体（尤其 ~24MB 大文件）下载失败只
// 告警并跳过，不让整批 fetch 失败中断排版。顺序加载避免并发洪峰。
const installFonts = (async (...args: unknown[]) => {
  const ctx = args[1] as {
    ref: { loadFonts: (builder: unknown, urls: string[]) => Promise<unknown> }
    builder: unknown
  }
  for (const name of FONT_NAMES) {
    try {
      await ctx.ref.loadFonts(ctx.builder, [`/fonts/${name}`])
    } catch (e) {
      console.warn('[typst] 字体加载失败，已跳过:', name, e)
    }
  }
}) as BeforeBuildFn

let configured = false

async function ensureConfigured(): Promise<void> {
  if (configured) return
  $typst.setCompilerInitOptions({ getModule: () => compilerWasmUrl })
  $typst.setRendererInitOptions({ getModule: () => rendererWasmUrl })
  // 字体全部本地化：逐个加载，不依赖 jsdelivr CDN。
  $typst.use({
    key: 'chatty-local-fonts',
    forRoles: ['compiler', 'renderer'],
    provides: [installFonts],
  })
  configured = true
}

let chain: Promise<unknown> = Promise.resolve()

export function renderTypst(source: string): Promise<string> {
  const run = chain.then(async () => {
    await ensureConfigured()
    return $typst.svg({
      mainContent: source,
    })
  })
  // 失败也放行队列，避免一次错误卡死整条链
  chain = run.then(
    () => undefined,
    () => undefined,
  )
  return run
}
