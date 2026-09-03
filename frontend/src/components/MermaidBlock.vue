<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { renderMermaid } from '../lib/mermaid'
import { view } from '../theme'

const props = defineProps<{ source: string }>()

const svg = ref('')
const error = ref('')
const pending = ref(false)

async function render() {
  if (pending.value) return
  pending.value = true
  error.value = ''
  try {
    svg.value = await renderMermaid(props.source)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    pending.value = false
  }
}

function rerenderOnThemeChange() {
  // 主题变化：清掉旧图重新渲染（画布黑/白跟随）
  if (svg.value || error.value) {
    svg.value = ''
    error.value = ''
    render()
  }
}

onMounted(render)
watch(() => props.source, () => {
  svg.value = ''
  error.value = ''
  render()
})
watch(() => view.theme, rerenderOnThemeChange)

// 画布跟随主题：浅色 = 白纸卡，暗色 = 黑色画布
const containerStyle = computed(() =>
  view.theme === 'dark' ? { background: '#000000', borderColor: 'transparent' } : {},
)
</script>

<template>
  <div class="mermaid-block">
    <div v-if="pending" class="mermaid-block__status">渲染图中…</div>
    <div v-else-if="error" class="mermaid-block__error">
      <div class="mermaid-block__error-msg">Mermaid 渲染失败：{{ error }}</div>
      <pre class="mermaid-block__raw">{{ source }}</pre>
    </div>
    <div v-else v-html="svg" class="mermaid-block__svg" :style="containerStyle"></div>
  </div>
</template>

<style scoped>
.mermaid-block {
  overflow-x: auto;
}
.mermaid-block__svg {
  display: flex;
  justify-content: center;
  background: var(--paper);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 8px;
}
.mermaid-block__svg :deep(svg) {
  max-width: 100%;
  height: auto;
}
.mermaid-block__status {
  color: var(--text-faint);
  font-size: var(--fs-xs);
  padding: 4px 0;
}
.mermaid-block__error {
  color: var(--danger);
}
.mermaid-block__error-msg {
  font-size: var(--fs-xs);
  margin-bottom: 4px;
}
.mermaid-block__raw {
  background: var(--danger-bg);
  border-radius: 6px;
  padding: 8px;
  font-size: var(--fs-xs);
  white-space: pre-wrap;
  margin: 0;
}
</style>
