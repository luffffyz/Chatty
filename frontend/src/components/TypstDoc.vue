<script setup lang="ts">
// 整条 assistant 消息的 Typst 渲染：
// 排版宽度 = 主窗口去掉侧栏后的比例（固定来源，避免容器测量反馈环），
// 文字颜色/字号跟随当前主题与字号设置，窗口/主题/字号变化时重编译。
// 编译失败时回退为原文，保证内容可读。
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { renderTypst } from '../lib/typst'
import { textHex, view } from '../theme'

const props = defineProps<{
  source: string
  /** 段序号（如 "3"），渲染失败时用于定位该段 */
  label?: string
}>()

const svg = ref('')
const pending = ref(false)
const error = ref('')
/** 当前编译宽度对应的显示像素宽（pt×4/3，保证 1pt→1px 的字号观感） */
const displayW = ref(0)

let timer: number | undefined

// 侧栏固定宽度；消息排版宽 = 主窗口去掉侧栏后按比例取整。
const SIDEBAR_PX = 240
const MSG_RATIO = 0.9

function msgWidthPx(): number {
  return Math.max(320, Math.floor((window.innerWidth - SIDEBAR_PX) * MSG_RATIO))
}

function scheduleCompile() {
  clearTimeout(timer)
  timer = window.setTimeout(() => void compile(), 90)
}

async function compile() {
  pending.value = true
  error.value = ''
  const widthPt = Math.round((msgWidthPx() * 0.75) / 4) * 4 // px→pt，取整到 4pt 网格
  displayW.value = Math.round(widthPt * (4 / 3))
  const sizePt = Math.max(8, view.fontSize * 0.75).toFixed(2)
  // wrapper 前置；提示词已要求模型不要写 #set page/text/import。
  const wrapped =
    `#set page(width: ${widthPt}pt, height: auto, margin: (x: 0pt, y: 8pt), fill: none)\n` +
    `#set text(size: ${sizePt}pt, fill: rgb("${textHex()}"), font: ("Libertinus Serif", "MiSans"))\n` +
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

function onWindowResize() {
  scheduleCompile()
}

onMounted(() => {
  compile()
  window.addEventListener('resize', onWindowResize)
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
  window.removeEventListener('resize', onWindowResize)
})
</script>

<template>
  <div class="doc">
    <!-- 渲染成功 -->
    <div v-if="svg" class="doc__svg" :style="{ '--tsw': displayW + 'px' }" v-html="svg"></div>
    <!-- 编译中 -->
    <div v-else-if="pending" class="doc__pending">排版中…</div>
    <!-- 任何失败/异常状态都回退原文，绝不出现空泡；标注段号便于定位 -->
    <div v-else class="doc__fallback">
      <pre class="doc__raw">{{ source }}</pre>
      <p v-if="error" class="doc__err">
        ⚠ Typst 渲染失败<template v-if="label">（第 {{ label }} 段）</template>：{{ error }}
      </p>
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
  /* 固定宽度（--tsw = 编译时窗口比例宽 pt×4/3 px）。不拉伸、不回读父级宽度，
     从根上消除“气泡(flex 内容定宽) ↔ 排版宽度”的反馈循环 */
  display: block;
  width: var(--tsw);
  max-width: 100%;
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
