<script setup lang="ts">
// 整条 assistant 消息的 Typst 渲染：
// 把消息内容当作一份连续排版的 Typst 文档编译，输出透明底、随容器宽度
// 换行的 SVG，文字颜色/字号跟随当前主题与字号设置。
// 编译失败时回退为原文，保证内容可读。
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { renderTypst } from '../lib/typst'
import { textHex, view } from '../theme'

const props = defineProps<{ source: string }>()

const host = ref<HTMLElement | null>(null)
const widthPx = ref(0)
const svg = ref('')
const pending = ref(false)
const error = ref('')

let ro: ResizeObserver | null = null
let timer: number | undefined

function measure() {
  // 优先宿主宽度；布局未就绪时回退父级/窗口宽度，避免编译被跳过。
  const w =
    host.value?.clientWidth ??
    host.value?.parentElement?.clientWidth ??
    Math.floor(window.innerWidth * 0.85)
  widthPx.value = Math.max(120, Math.floor(w))
}

function scheduleCompile() {
  clearTimeout(timer)
  timer = window.setTimeout(() => void compile(), 90)
}

async function compile() {
  measure()
  pending.value = true
  error.value = ''
  const widthPt = Math.round((widthPx.value * 0.75) / 4) * 4 // px→pt，取整到 4pt 网格
  const sizePt = Math.max(8, view.fontSize * 0.75).toFixed(2)
  // wrapper 前置；提示词已要求模型不要写 #set page/text/import。
  const wrapped =
    `#set page(width: ${widthPt}pt, height: auto, margin: 0pt, fill: none)\n` +
    `#set text(size: ${sizePt}pt, fill: rgb("${textHex()}"), font: "Roboto")\n` +
    `#set par(leading: 1.05em)\n\n` +
    props.source
  try {
    const out = await renderTypst(wrapped)
    svg.value = out && out.length > 0 ? out : ''
    if (!svg.value) error.value = '编译器未返回 SVG'
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
    console.error('[TypstDoc] compile failed:', error.value)
  } finally {
    pending.value = false
  }
}

onMounted(async () => {
  measure()
  ro = new ResizeObserver(() => scheduleCompile())
  if (host.value) ro.observe(host.value)
  await nextTick()
  compile()
})

watch(
  () => props.source,
  () => {
    svg.value = ''
    compile()
  },
)
watch(() => view.fontSize, scheduleCompile)
watch(() => view.theme, scheduleCompile)

onBeforeUnmount(() => {
  clearTimeout(timer)
  ro?.disconnect()
})
</script>

<template>
  <div ref="host" class="doc">
    <!-- 渲染成功 -->
    <div v-if="svg" class="doc__svg" v-html="svg"></div>
    <!-- 编译中 -->
    <div v-else-if="pending" class="doc__pending">排版中…</div>
    <!-- 任何失败/异常状态都回退原文，绝不出现空泡 -->
    <div v-else class="doc__fallback">
      <pre class="doc__raw">{{ source }}</pre>
      <p v-if="error" class="doc__err">⚠ Typst 渲染失败：{{ error }}</p>
    </div>
  </div>
</template>

<style scoped>
.doc {
  width: 100%;
}
.doc__pending {
  color: var(--text-faint);
  font-size: var(--fs-sm);
  padding: 2px 0;
}
.doc__svg {
  width: 100%;
  overflow-x: auto;
  overflow-y: hidden;
}
.doc__svg :deep(svg) {
  display: block;
  width: 100%;
  height: auto;
  overflow: visible;
}
.doc__fallback {
  width: 100%;
}
.doc__raw {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: ui-monospace, Consolas, "Courier New", monospace;
  font-size: var(--fs-sm);
  color: var(--text);
}
.doc__err {
  margin: 6px 0 0;
  color: var(--text-faint);
  font-size: var(--fs-xs);
}
</style>
