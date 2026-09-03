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
import compilerWasmUrl from '@myriaddreamin/typst-ts-web-compiler/pkg/typst_ts_web_compiler_bg.wasm?url'
import rendererWasmUrl from '@myriaddreamin/typst-ts-renderer/pkg/typst_ts_renderer_bg.wasm?url'

let configured = false

async function ensureConfigured(): Promise<void> {
  if (configured) return
  $typst.setCompilerInitOptions({ getModule: () => compilerWasmUrl })
  $typst.setRendererInitOptions({ getModule: () => rendererWasmUrl })
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
